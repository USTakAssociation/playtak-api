package session

import (
	"context"
	"errors"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/auth"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/online"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

// Client versions too old to work with the current text protocol.
var retiredClients = map[string]bool{
	"TreffnonX-08.09.16": true,
	"TakWeb-16.05.26":    true,
}

func (s *Session) clientName(command protocol.ClientName) {
	s.log.Debug("client software", "client", command.Name)
	if retiredClients[command.Name] {
		s.Send(protocol.Shout{From: "Server", Text: "Your Playtak client is unfortunately no longer compatible. Please go to https://www.playtak.com in order to play."})
		return
	}
	s.Send(protocol.OK{})
}

// Login message order matches the Java server: a guest's game is resumed
// before their Welcome, a registered player's after Welcome and Is Mod; then
// come the seek list, the game list and the presence update.
func (s *Session) loginGuest(ctx context.Context, command protocol.LoginGuest) {
	p := s.manager.players.Guest(command.Token)
	s.manager.bind(s, p)
	s.log.Info("guest logged in", "player", p.Name)
	s.resumeGame(ctx, p)
	s.Send(protocol.LoggedIn{Name: p.Name})
	s.subscribe()
}

func (s *Session) login(ctx context.Context, command protocol.Login) {
	record, err := s.manager.auth.Login(ctx, command.Name, command.Password, s.ip())
	// A rate-limited login looks like any other failure to old clients.
	if errors.Is(err, auth.ErrAuthFailed) || errors.Is(err, auth.ErrRateLimited) {
		s.Send(protocol.AuthFailure{})
		return
	}
	if err != nil {
		s.fail("login", err)
		return
	}
	p := s.manager.players.Acquire(record)
	s.manager.bind(s, p)
	s.log.Info("logged in", "player", p.Name)
	s.Send(protocol.LoggedIn{Name: p.Name})
	if p.IsAdmin() || p.IsMod() {
		s.Send(protocol.IsMod{})
	}
	s.resumeGame(ctx, p)
	s.subscribe()
}

func (s *Session) subscribe() {
	s.manager.seeks.Subscribe(s)
	s.manager.games.Subscribe(s)
	s.ready.Store(true)
	s.manager.broadcastPresence()
}

// resumeGame replays the player's game in progress, if any, to this session.
func (s *Session) resumeGame(ctx context.Context, p *online.Player) {
	if g := s.manager.games.ForPlayer(p.Name); g != nil {
		if err := g.Reconnected(ctx, p, s); err != nil {
			s.log.Error("resume game", "player", p.Name, "game", g.ID(), "err", err)
		}
	}
}

func (s *Session) register(ctx context.Context, command protocol.Register) {
	record, err := s.manager.auth.Register(ctx, command.Name, command.Email, s.ip())
	var reason protocol.RegistrationError
	switch {
	case err == nil:
		s.log.Info("registered", "player", record.Name)
		s.Send(protocol.Registered{Name: record.Name})
		return
	case errors.Is(err, auth.ErrInvalidUsername), errors.Is(err, auth.ErrInvalidEmail):
		reason = protocol.RegistrationMalformed
	case errors.Is(err, auth.ErrGuestName):
		reason = protocol.RegistrationGuestName
	case errors.Is(err, auth.ErrNameTaken):
		reason = protocol.RegistrationNameTaken
	case errors.Is(err, auth.ErrRateLimited):
		reason = protocol.RegistrationRateLimited
	default:
		s.log.Error("register", "err", err)
		reason = protocol.RegistrationServerError
	}
	s.Send(protocol.RegistrationFailed{Reason: reason})
}

func (s *Session) sendResetToken(ctx context.Context, command protocol.SendResetToken) {
	switch err := s.manager.auth.SendResetToken(ctx, command.Name, command.Email); {
	case err == nil:
		s.Send(protocol.ResetTokenSent{})
	case errors.Is(err, auth.ErrNoSuchPlayer):
		s.Send(protocol.ResetTokenFailed{Reason: protocol.ResetTokenNoSuchPlayer})
	case errors.Is(err, auth.ErrEmailMismatch):
		s.Send(protocol.ResetTokenFailed{Reason: protocol.ResetTokenEmailMismatch})
	case errors.Is(err, auth.ErrRateLimited):
		s.Send(protocol.ResetTokenFailed{Reason: protocol.ResetTokenRateLimited})
	default:
		s.fail("send reset token", err)
	}
}

func (s *Session) resetPassword(ctx context.Context, command protocol.ResetPassword) {
	switch err := s.manager.auth.ResetPassword(ctx, command.Name, command.Token, command.Password); {
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

func (s *Session) changePassword(ctx context.Context, p *online.Player, command protocol.ChangePassword) {
	if p.Guest {
		s.Send(protocol.WrongPassword{})
		return
	}
	switch err := s.manager.auth.ChangePassword(ctx, p.Name, command.Old, command.New); {
	case err == nil:
		s.Send(protocol.PasswordChanged{})
	case errors.Is(err, auth.ErrWrongPassword):
		s.Send(protocol.WrongPassword{})
	default:
		s.fail("change password", err)
	}
}
