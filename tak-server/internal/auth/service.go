// Package auth handles credentials and API tokens for every entry point:
// the telnet and WebSocket protocols and the REST API.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/ratelimit"
)

var (
	ErrAuthFailed      = errors.New("authentication failed")
	ErrInvalidUsername = errors.New("invalid username")
	ErrInvalidEmail    = errors.New("invalid email")
	ErrInvalidPassword = errors.New("invalid password")
	ErrGuestName       = errors.New("username contains guest")
	ErrNameTaken       = errors.New("username is already taken")
	ErrNoSuchPlayer    = errors.New("no such player")
	// ErrBadNameOrEmail covers both an unknown name and the wrong email, so
	// the reply doesn't reveal which names are registered.
	ErrBadNameOrEmail = errors.New("username or email is invalid")
	ErrWrongToken     = errors.New("wrong or expired token")
	ErrWrongPassword  = errors.New("wrong password")
)

var (
	usernamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{3,15}$`)
	emailPattern    = regexp.MustCompile(`^[A-Za-z.0-9_+!#$%&'*^?=-]{1,30}@[A-Za-z.0-9-]{3,30}$`)
	passwordPattern = regexp.MustCompile(`^[^\s\x0B]{6,50}$`)
)

// ValidUsername, ValidEmail and ValidPassword apply the same rules the text
// protocol enforces, for entry points like REST that don't go through it.
func ValidUsername(s string) bool { return usernamePattern.MatchString(s) }
func ValidEmail(s string) bool    { return emailPattern.MatchString(s) }
func ValidPassword(s string) bool { return passwordPattern.MatchString(s) }

const resetTokenTTL = time.Hour

type PlayerStore interface {
	ByName(ctx context.Context, name string) (players.Record, error)
	ByID(ctx context.Context, id int) (players.Record, error)
	Create(ctx context.Context, name, email, passwordHash string) (players.Record, error)
	SetPassword(ctx context.Context, id int, hash string) error
}

type TokenStore interface {
	Insert(ctx context.Context, tokens ...players.Token) error
	Get(ctx context.Context, hash []byte, now time.Time) (players.Token, error)
	Rotate(ctx context.Context, oldHash []byte, now time.Time, next ...players.Token) error
	Delete(ctx context.Context, hashes ...[]byte) error
	DeleteForPlayer(ctx context.Context, playerID int) (int64, error)
	DeleteExpired(ctx context.Context, now time.Time) (int64, error)
}

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

type Service struct {
	players PlayerStore
	tokens  TokenStore
	mail    Mailer
	log     *slog.Logger
	now     func() time.Time
	// dummyHash is checked against for unknown names; tests swap in a
	// cheap one.
	dummyHash func() string

	limits limiters // all off until EnableLimits

	mu          sync.Mutex
	resetTokens map[string]resetToken // by player name
}

type resetToken struct {
	token   string
	expires time.Time
}

func NewService(ps PlayerStore, ts TokenStore, mail Mailer, log *slog.Logger) *Service {
	return &Service{
		players:     ps,
		tokens:      ts,
		mail:        mail,
		log:         log,
		now:         time.Now,
		dummyHash:   dummyHash,
		resetTokens: make(map[string]resetToken),
	}
}

// Login checks a registered player's credentials, from a client at ip.
// Banned players, unknown names and wrong passwords all return
// ErrAuthFailed. Too many recent failures, from the IP or (when enabled)
// for the name, return a *RateLimitError before the password is checked.
// Only failures count, so many players behind one IP can all log in.
func (s *Service) Login(ctx context.Context, name, password, ip string) (_ players.Record, err error) {
	finish, err := s.startLogin(name, ip)
	if err != nil {
		return players.Record{}, err
	}
	// Deferred so no return path, or panic, leaves the attempt counted
	// forever. A database error isn't the client's fault, so it doesn't count.
	defer func() { finish(errors.Is(err, ErrAuthFailed)) }()
	record, err := s.players.ByName(ctx, name)
	switch {
	case errors.Is(err, players.ErrNotFound):
		// Hash anyway, so an unknown name takes as long as a wrong password.
		checkPassword(s.dummyHash(), password)
		err = ErrAuthFailed
	case err == nil && (!checkPassword(record.PasswordHash, password) || record.IsBanned):
		err = ErrAuthFailed
	}
	if err != nil {
		return players.Record{}, err
	}
	// A success clears the name's failures, but not the IP's: on a shared
	// IP, one player logging in mustn't wipe out someone else's guesses.
	if s.limits.failuresByName != nil {
		s.limits.failuresByName.Reset(nameKey(name))
	}
	// Logging in proves the player knows their password.
	s.mu.Lock()
	delete(s.resetTokens, name)
	s.mu.Unlock()
	return record, nil
}

// Register creates a player with a random password and emails it to them.
// Each client IP can register only a few accounts an hour.
func (s *Service) Register(ctx context.Context, name, email, ip string) (players.Record, error) {
	if err := s.allow(s.limits.registerByIP, "registrations per IP", ratelimit.IPKey(ip)); err != nil {
		return players.Record{}, err
	}
	switch {
	case !ValidUsername(name):
		return players.Record{}, ErrInvalidUsername
	case !ValidEmail(email):
		return players.Record{}, ErrInvalidEmail
	case strings.Contains(strings.ToLower(name), "guest"):
		return players.Record{}, ErrGuestName
	}

	password := newTemporaryPassword()
	hash, err := hashPassword(password)
	if err != nil {
		return players.Record{}, fmt.Errorf("register %s: %w", name, err)
	}
	record, err := s.players.Create(ctx, name, email, hash)
	if errors.Is(err, players.ErrNameTaken) {
		return players.Record{}, ErrNameTaken
	}
	if err != nil {
		return players.Record{}, err
	}
	s.sendMail(name, email, "playtak.com password",
		fmt.Sprintf("Hello %s, your password is %s You can change it on playtak.com.", name, password))
	return record, nil
}

// SendResetToken emails a one-time password reset token, valid for an hour.
// Only a few are sent per player per hour. An unknown name and the wrong
// email both return ErrBadNameOrEmail.
func (s *Service) SendResetToken(ctx context.Context, name, email string) error {
	if err := s.allow(s.limits.resetByName, "reset emails per name", nameKey(name)); err != nil {
		return err
	}
	record, err := s.players.ByName(ctx, name)
	if errors.Is(err, players.ErrNotFound) || (err == nil && record.Email != email) {
		return ErrBadNameOrEmail
	}
	if err != nil {
		return err
	}
	token := rand.Text()
	s.mu.Lock()
	s.resetTokens[name] = resetToken{token: token, expires: s.now().Add(resetTokenTTL)}
	s.mu.Unlock()
	s.sendMail(name, email, "playtak.com reset password", "Your reset token is "+token)
	return nil
}

// ResetPassword sets a new password using a token from SendResetToken. An
// unknown name gets ErrWrongToken, like a wrong token, so the reply doesn't
// reveal which names are registered.
func (s *Service) ResetPassword(ctx context.Context, name, token, password string) error {
	if !ValidPassword(password) {
		return ErrInvalidPassword
	}
	s.mu.Lock()
	rt, ok := s.resetTokens[name]
	valid := ok && subtle.ConstantTimeCompare([]byte(rt.token), []byte(token)) == 1 && s.now().Before(rt.expires)
	if valid {
		delete(s.resetTokens, name)
	}
	s.mu.Unlock()
	if !valid {
		return ErrWrongToken
	}
	record, err := s.players.ByName(ctx, name)
	if errors.Is(err, players.ErrNotFound) {
		return ErrWrongToken // deleted since the token was sent
	}
	if err != nil {
		return err
	}
	return s.setPassword(ctx, record.ID, password)
}

// ChangePassword replaces a player's password after checking the current one.
func (s *Service) ChangePassword(ctx context.Context, name, current, next string) error {
	record, err := s.players.ByName(ctx, name)
	if errors.Is(err, players.ErrNotFound) {
		return ErrWrongPassword
	}
	if err != nil {
		return err
	}
	if record.IsBanned || !checkPassword(record.PasswordHash, current) {
		return ErrWrongPassword
	}
	if !ValidPassword(next) {
		return ErrInvalidPassword
	}
	return s.setPassword(ctx, record.ID, next)
}

// SetPassword replaces a player's password without checking the current one,
// for admin use.
func (s *Service) SetPassword(ctx context.Context, name, password string) error {
	record, err := s.players.ByName(ctx, name)
	if errors.Is(err, players.ErrNotFound) {
		return ErrNoSuchPlayer
	}
	if err != nil {
		return err
	}
	return s.setPassword(ctx, record.ID, password)
}

// setPassword replaces the password and revokes every API token the player
// holds, so a stolen token stops working once the password changes.
func (s *Service) setPassword(ctx context.Context, id int, password string) error {
	hash, err := hashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := s.players.SetPassword(ctx, id, hash); err != nil {
		return err
	}
	// The password has changed either way, so this doesn't fail the call.
	if _, err := s.tokens.DeleteForPlayer(ctx, id); err != nil {
		s.log.Error("revoke tokens after password change", "player_id", id, "err", err)
	}
	return nil
}

// NewGuestToken returns a token that rejoins the same guest when passed to
// "Login Guest <token>": 20 lowercase letters, as the text protocol requires.
func NewGuestToken() string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 20)
	rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}

// sendMail sends to player's address in the background so callers aren't
// held up by SMTP. A failure is logged by player name, never by address.
func (s *Service) sendMail(player, to, subject, body string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := s.mail.Send(ctx, to, subject, body); err != nil {
			s.log.Error("send email", "player", player, "subject", subject, "err", err)
		}
	}()
}
