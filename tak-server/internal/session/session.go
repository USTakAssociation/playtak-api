package session

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/auth"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/player"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol/legacy"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/seek"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport"
)

// commandTimeout bounds the database and service work for one command.
const commandTimeout = 10 * time.Second

// Session is one client connection and what it is logged in as.
type Session struct {
	id   int64
	m    *Manager
	conn transport.Conn
	log  *slog.Logger // fixed at creation; read from any goroutine
	done chan struct{}

	// Read by other goroutines (broadcasts), written by the read goroutine.
	protocolVersion atomic.Int32
	player          atomic.Pointer[player.Player]
	quitting        atomic.Bool
}

// Send renders ev in the client's protocol and queues it.
func (s *Session) Send(ev protocol.Event) {
	lines, err := legacy.Format(ev, s.ProtocolVersion())
	if err != nil {
		s.log.Error("format event", "err", err)
		return
	}
	for _, l := range lines {
		s.conn.Send(l)
	}
}

// ProtocolVersion is the version the client declared with "Protocol", or 0.
func (s *Session) ProtocolVersion() int { return int(s.protocolVersion.Load()) }

// Disconnect closes the connection after delivering queued events.
func (s *Session) Disconnect() { s.conn.Close(transport.CloseKicked) }

// Done is closed once the session has been fully cleaned up.
func (s *Session) Done() <-chan struct{} { return s.done }

// Closed cleans up after the connection has gone away.
func (s *Session) Closed() {
	s.m.remove(s)
	if p := s.player.Load(); p != nil {
		s.m.seeks.Unsubscribe(s)
		s.m.seeks.Cancel(s)
		s.m.players.Detach(p, s)
		s.m.broadcastPresence()
	}
	s.log.Debug("disconnected")
	close(s.done)
}

// HandleLine runs one command from the client.
func (s *Session) HandleLine(line string) {
	if s.quitting.Load() {
		return
	}
	p := s.player.Load()
	cmd := legacy.Parse(line, p != nil)
	ctx, cancel := context.WithTimeout(s.m.ctx, commandTimeout)
	defer cancel()

	switch cmd := cmd.(type) {
	case protocol.Quit:
		s.quitting.Store(true)
		s.conn.Close(transport.CloseNormal)
	case protocol.Ping:
		if p != nil {
			p.Touch()
		}
		s.Send(protocol.OK{})
	case protocol.ClientName:
		s.clientName(cmd)
	case protocol.SetProtocol:
		s.protocolVersion.Store(int32(cmd.Version))
		s.Send(protocol.OK{})
	case protocol.LoginGuest:
		s.loginGuest(cmd)
	case protocol.Login:
		s.login(ctx, cmd)
	case protocol.Register:
		s.register(ctx, cmd)
	case protocol.RegisterMalformed:
		s.Send(protocol.RegistrationFailed{Reason: protocol.RegistrationMalformed})
	case protocol.SendResetToken:
		s.sendResetToken(ctx, cmd)
	case protocol.ResetPassword:
		s.resetPassword(ctx, cmd)
	case protocol.ChangePassword:
		s.changePassword(ctx, p, cmd)
	case protocol.ListSeeks:
		s.m.seeks.SendList(s)
	case protocol.PostSeek:
		s.postSeek(p, cmd)
	case protocol.CancelSeek:
		s.m.seeks.Cancel(s)
	case protocol.Rematch:
		s.rematch(p, cmd)
	case protocol.AcceptSeek:
		// Accepting starts a game, which isn't implemented yet.
		s.Send(protocol.NOK{})
	default:
		s.Send(protocol.NOK{})
	}
}

// Client versions too old to work with the current protocol.
var retiredClients = map[string]bool{
	"TreffnonX-08.09.16": true,
	"TakWeb-16.05.26":    true,
}

func (s *Session) clientName(cmd protocol.ClientName) {
	s.log.Debug("client", "name", cmd.Name)
	if retiredClients[cmd.Name] {
		s.Send(protocol.Shout{From: "Server", Text: "Your Playtak client is unfortunately no longer compatible. Please go to https://www.playtak.com in order to play."})
		return
	}
	s.Send(protocol.OK{})
}

func (s *Session) loginGuest(cmd protocol.LoginGuest) {
	p := s.m.players.Guest(cmd.Token)
	s.m.bind(s, p)
	s.log.Info("guest logged in", "player", p.Name)
	s.Send(protocol.LoggedIn{Name: p.Name})
	s.m.seeks.Subscribe(s)
	s.m.broadcastPresence()
}

func (s *Session) login(ctx context.Context, cmd protocol.Login) {
	rec, err := s.m.auth.Login(ctx, cmd.Name, cmd.Password)
	if errors.Is(err, auth.ErrAuthFailed) {
		s.Send(protocol.AuthFailure{})
		return
	}
	if err != nil {
		s.fail("login", err)
		return
	}
	p := s.m.players.Acquire(rec)
	s.m.bind(s, p)
	s.log.Info("logged in", "player", p.Name)
	s.Send(protocol.LoggedIn{Name: p.Name})
	if p.IsAdmin() || p.IsMod() {
		s.Send(protocol.IsMod{})
	}
	s.m.seeks.Subscribe(s)
	s.m.broadcastPresence()
}

func (s *Session) register(ctx context.Context, cmd protocol.Register) {
	rec, err := s.m.auth.Register(ctx, cmd.Name, cmd.Email)
	var reason protocol.RegistrationError
	switch {
	case err == nil:
		s.log.Info("registered", "name", rec.Name)
		s.Send(protocol.Registered{Name: rec.Name})
		return
	case errors.Is(err, auth.ErrInvalidUsername), errors.Is(err, auth.ErrInvalidEmail):
		reason = protocol.RegistrationMalformed
	case errors.Is(err, auth.ErrGuestName):
		reason = protocol.RegistrationGuestName
	case errors.Is(err, auth.ErrProfanity):
		reason = protocol.RegistrationProfanity
	case errors.Is(err, auth.ErrNameTaken):
		reason = protocol.RegistrationNameTaken
	default:
		s.log.Error("register", "err", err)
		reason = protocol.RegistrationServerError
	}
	s.Send(protocol.RegistrationFailed{Reason: reason})
}

func (s *Session) sendResetToken(ctx context.Context, cmd protocol.SendResetToken) {
	switch err := s.m.auth.SendResetToken(ctx, cmd.Name, cmd.Email); {
	case err == nil:
		s.Send(protocol.ResetTokenSent{})
	case errors.Is(err, auth.ErrNoSuchPlayer):
		s.Send(protocol.ResetTokenFailed{Reason: protocol.ResetTokenNoSuchPlayer})
	case errors.Is(err, auth.ErrEmailMismatch):
		s.Send(protocol.ResetTokenFailed{Reason: protocol.ResetTokenEmailMismatch})
	default:
		s.fail("send reset token", err)
	}
}

func (s *Session) resetPassword(ctx context.Context, cmd protocol.ResetPassword) {
	switch err := s.m.auth.ResetPassword(ctx, cmd.Name, cmd.Token, cmd.Password); {
	case err == nil:
		s.Send(protocol.PasswordReset{})
	case errors.Is(err, auth.ErrNoSuchPlayer):
		s.Send(protocol.PasswordResetFailed{Reason: protocol.ResetPasswordNoSuchPlayer})
	case errors.Is(err, auth.ErrWrongToken):
		s.Send(protocol.PasswordResetFailed{Reason: protocol.ResetPasswordWrongToken})
	default:
		s.fail("reset password", err)
	}
}

func (s *Session) changePassword(ctx context.Context, p *player.Player, cmd protocol.ChangePassword) {
	if p.Guest {
		s.Send(protocol.WrongPassword{})
		return
	}
	switch err := s.m.auth.ChangePassword(ctx, p.Name, cmd.Old, cmd.New); {
	case err == nil:
		s.Send(protocol.PasswordChanged{})
	case errors.Is(err, auth.ErrWrongPassword):
		s.Send(protocol.WrongPassword{})
	default:
		s.fail("change password", err)
	}
}

func (s *Session) postSeek(p *player.Player, cmd protocol.PostSeek) {
	_, err := s.m.seeks.Post(s, seek.Request{Creator: p.Name, Bot: p.IsBot(), Opponent: cmd.Opponent, Settings: cmd.Settings})
	if err != nil {
		s.Send(protocol.NOK{})
	}
}

func (s *Session) rematch(p *player.Player, cmd protocol.Rematch) {
	res, err := s.m.seeks.Rematch(s, cmd.GameID, seek.Request{Creator: p.Name, Bot: p.IsBot(), Opponent: cmd.Opponent, Settings: cmd.Settings})
	switch {
	case err != nil:
		s.Send(protocol.NOK{})
	case res.AcceptID != 0:
		s.Send(protocol.AcceptRematch{SeekID: res.AcceptID})
	default:
		s.Send(protocol.RematchSeekCreated{SeekID: res.Created.ID})
	}
}

// fail logs an unexpected error and tells the client the command failed.
func (s *Session) fail(op string, err error) {
	name := ""
	if p := s.player.Load(); p != nil {
		name = p.Name
	}
	s.log.Error(op, "player", name, "err", err)
	s.Send(protocol.NOK{})
}
