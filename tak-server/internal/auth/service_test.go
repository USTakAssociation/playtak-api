package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/dbtest"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
)

type sentMail struct{ to, subject, body string }

type fakeMailer chan sentMail

func (m fakeMailer) Send(_ context.Context, to, subject, body string) error {
	m <- sentMail{to, subject, body}
	return nil
}

func (m fakeMailer) next(t *testing.T) sentMail {
	t.Helper()
	select {
	case mail := <-m:
		return mail
	case <-time.After(5 * time.Second):
		t.Fatal("no email sent")
		return sentMail{}
	}
}

type fakeProfanity []string

func (f fakeProfanity) ContainsBadWord(s string) bool {
	for _, w := range f {
		if strings.Contains(strings.ToLower(s), w) {
			return true
		}
	}
	return false
}

type fixture struct {
	svc   *Service
	store *players.Store
	mail  fakeMailer
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	conn := dbtest.NewPlayersDB(t)
	store := players.NewStore(conn)
	mail := make(fakeMailer, 10)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(store, players.NewTokenStore(conn), mail, fakeProfanity{"darn"}, log)
	return fixture{svc: svc, store: store, mail: mail}
}

// addPlayer creates a player with a known password.
func (f fixture) addPlayer(t *testing.T, name, email, password string) players.Record {
	t.Helper()
	hash, err := hashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := f.store.Create(context.Background(), name, email, hash)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func TestLogin(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	alice := f.addPlayer(t, "alice", "a@example.com", "secret123")

	got, err := f.svc.Login(ctx, "alice", "secret123", "198.51.100.1")
	if err != nil || got.ID != alice.ID {
		t.Fatalf("Login = %+v, %v", got, err)
	}
	for _, tc := range []struct{ name, password string }{
		{"alice", "wrongpass"},
		{"Alice", "secret123"}, // names are case-sensitive
		{"nobody", "secret123"},
	} {
		if _, err := f.svc.Login(ctx, tc.name, tc.password, "198.51.100.1"); !errors.Is(err, ErrAuthFailed) {
			t.Errorf("Login(%q, %q) err = %v, want ErrAuthFailed", tc.name, tc.password, err)
		}
	}

	if err := f.store.SetFlag(ctx, "alice", players.FlagBanned, true); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Login(ctx, "alice", "secret123", "198.51.100.1"); !errors.Is(err, ErrAuthFailed) {
		t.Errorf("banned player logged in: err = %v", err)
	}
}

func TestLoginAcceptsJavaHash(t *testing.T) {
	// Hash of "password" written by jBCrypt, from scripts/development/add_user.sh.
	f := newFixture(t)
	const javaHash = "$2a$10$JpGWa00gDsDtJK0Ttq/dD.F2zTj9kCkdcHwO5pIZFfWbT3CfDY/C6"
	if _, err := f.store.Create(context.Background(), "legacy", "", javaHash); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Login(context.Background(), "legacy", "password", "198.51.100.1"); err != nil {
		t.Errorf("Java-era hash rejected: %v", err)
	}
}

func TestLongPasswordsAreTruncatedLikeJBCrypt(t *testing.T) {
	long := strings.Repeat("x", 72)
	hash, err := hashPassword(long + "ignored")
	if err != nil {
		t.Fatal(err)
	}
	if !checkPassword(hash, long+"different-suffix") {
		t.Error("bytes past 72 should not affect the hash")
	}
}

func TestRegister(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	rec, err := f.svc.Register(ctx, "newbie", "n@example.com", "198.51.100.1")
	if err != nil {
		t.Fatal(err)
	}
	mail := f.mail.next(t)
	if mail.to != "n@example.com" || mail.subject != "playtak.com password" {
		t.Errorf("mail = %+v", mail)
	}
	m := regexp.MustCompile(`your password is (\S+) You can`).FindStringSubmatch(mail.body)
	if m == nil {
		t.Fatalf("no password in body %q", mail.body)
	}
	if got, err := f.svc.Login(ctx, "newbie", m[1], "198.51.100.1"); err != nil || got.ID != rec.ID {
		t.Errorf("login with emailed password: %+v, %v", got, err)
	}

	for _, tc := range []struct {
		name, email string
		want        error
	}{
		{"ab", "x@example.com", ErrInvalidUsername},
		{"1abc", "x@example.com", ErrInvalidUsername},
		{"valid", "not-an-email", ErrInvalidEmail},
		{"MyGuestName", "x@example.com", ErrGuestName},
		{"darnit", "x@example.com", ErrProfanity},
		{"newbie", "x@example.com", ErrNameTaken},
		{"NEWB1E", "x@example.com", ErrNameTaken},
	} {
		if _, err := f.svc.Register(ctx, tc.name, tc.email, "198.51.100.1"); !errors.Is(err, tc.want) {
			t.Errorf("Register(%q, %q) err = %v, want %v", tc.name, tc.email, err, tc.want)
		}
	}
}

func TestPasswordReset(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.addPlayer(t, "alice", "a@example.com", "oldpass1")

	if err := f.svc.SendResetToken(ctx, "nobody", "a@example.com"); !errors.Is(err, ErrNoSuchPlayer) {
		t.Errorf("unknown player: %v", err)
	}
	if err := f.svc.SendResetToken(ctx, "alice", "other@example.com"); !errors.Is(err, ErrEmailMismatch) {
		t.Errorf("wrong email: %v", err)
	}
	if err := f.svc.SendResetToken(ctx, "alice", "a@example.com"); err != nil {
		t.Fatal(err)
	}
	token := strings.TrimPrefix(f.mail.next(t).body, "Your reset token is ")

	if err := f.svc.ResetPassword(ctx, "alice", "badtoken", "newpass1"); !errors.Is(err, ErrWrongToken) {
		t.Errorf("bad token: %v", err)
	}
	if err := f.svc.ResetPassword(ctx, "alice", token, "newpass1"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ResetPassword(ctx, "alice", token, "again123"); !errors.Is(err, ErrWrongToken) {
		t.Errorf("tokens must be single use: %v", err)
	}
	if _, err := f.svc.Login(ctx, "alice", "newpass1", "198.51.100.1"); err != nil {
		t.Errorf("login with reset password: %v", err)
	}
}

func TestResetTokenExpires(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.addPlayer(t, "alice", "a@example.com", "oldpass1")
	if err := f.svc.SendResetToken(ctx, "alice", "a@example.com"); err != nil {
		t.Fatal(err)
	}
	token := strings.TrimPrefix(f.mail.next(t).body, "Your reset token is ")

	f.svc.now = func() time.Time { return time.Now().Add(resetTokenTTL + time.Minute) }
	if err := f.svc.ResetPassword(ctx, "alice", token, "newpass1"); !errors.Is(err, ErrWrongToken) {
		t.Errorf("expired token accepted: %v", err)
	}
}

func TestLoginClearsResetToken(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.addPlayer(t, "alice", "a@example.com", "oldpass1")
	f.svc.SendResetToken(ctx, "alice", "a@example.com")
	token := strings.TrimPrefix(f.mail.next(t).body, "Your reset token is ")

	if _, err := f.svc.Login(ctx, "alice", "oldpass1", "198.51.100.1"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ResetPassword(ctx, "alice", token, "newpass1"); !errors.Is(err, ErrWrongToken) {
		t.Errorf("reset token should be discarded after login: %v", err)
	}
}

func TestChangePassword(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.addPlayer(t, "alice", "", "oldpass1")

	if err := f.svc.ChangePassword(ctx, "alice", "wrong", "newpass1"); !errors.Is(err, ErrWrongPassword) {
		t.Errorf("wrong current password: %v", err)
	}
	if err := f.svc.ChangePassword(ctx, "Guest1", "anything", "newpass1"); !errors.Is(err, ErrWrongPassword) {
		t.Errorf("unregistered name: %v", err)
	}
	if err := f.svc.ChangePassword(ctx, "alice", "oldpass1", "newpass1"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Login(ctx, "alice", "newpass1", "198.51.100.1"); err != nil {
		t.Errorf("login with changed password: %v", err)
	}
}

func TestNewGuestToken(t *testing.T) {
	re := regexp.MustCompile(`^[a-z]{20}$`)
	a, b := NewGuestToken(), NewGuestToken()
	if !re.MatchString(a) || a == b {
		t.Errorf("tokens %q, %q", a, b)
	}
}
