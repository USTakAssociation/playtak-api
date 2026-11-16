package session

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/clientip"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/game"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/online"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol/legacy"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport"
)

// commandTimeout bounds the database and service work for one command.
const commandTimeout = 10 * time.Second

// Session is one client connection and what it is logged in as.
type Session struct {
	id      int64
	manager *Manager
	conn    transport.Conn
	log     *slog.Logger // fixed at creation; read from any goroutine
	done    chan struct{}

	// Read by other goroutines (broadcasts), written by the read goroutine.
	protocolVersion atomic.Int32
	player          atomic.Pointer[online.Player]
	quitting        atomic.Bool
	// ready is set once the login messages have been sent; only then does
	// the session receive presence updates, so none arrives before Welcome.
	ready atomic.Bool
}

// Send renders event in the client's protocol and queues it.
func (s *Session) Send(event protocol.Event) {
	lines, err := legacy.Format(event, s.ProtocolVersion())
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
	s.manager.remove(s)
	if player := s.player.Load(); player != nil {
		s.manager.seeks.Unsubscribe(s)
		s.manager.seeks.Cancel(s)
		s.manager.games.Unsubscribe(s)
		s.manager.games.UnobserveAll(s)
		s.manager.chat.LeaveAll(s)
		s.manager.players.Detach(player, s)
		if g := s.manager.games.ForPlayer(player.Name); g != nil && !player.Online() {
			// Wait so the opponent hears about it before the presence update.
			if err := g.Disconnected(s.manager.ctx, player.Name); err != nil {
				s.log.Error("report disconnect to game", "player", player.Name, "game", g.ID(), "err", err)
			}
		}
		s.manager.broadcastPresence()
	}
	s.log.Debug("disconnected")
	close(s.done)
}

// HandleLine runs one command from the client.
func (s *Session) HandleLine(line string) {
	if s.quitting.Load() {
		return
	}
	player := s.player.Load()
	command := legacy.Parse(line, player != nil)
	ctx, cancel := context.WithTimeout(s.manager.ctx, commandTimeout)
	defer cancel()

	switch command := command.(type) {
	case protocol.Quit:
		s.quitting.Store(true)
		s.conn.Close(transport.CloseNormal)
	case protocol.Ping:
		if player != nil {
			player.Touch()
		}
		s.Send(protocol.OK{})
	case protocol.ClientName:
		s.clientName(command)
	case protocol.SetProtocol:
		s.protocolVersion.Store(int32(command.Version))
		s.Send(protocol.OK{})
	case protocol.LoginGuest:
		s.loginGuest(ctx, command)
	case protocol.Login:
		s.login(ctx, command)
	case protocol.Register:
		s.register(ctx, command)
	case protocol.RegisterMalformed:
		s.Send(protocol.RegistrationFailed{Reason: protocol.RegistrationMalformed})
	case protocol.SendResetToken:
		s.sendResetToken(ctx, command)
	case protocol.ResetPassword:
		s.resetPassword(ctx, command)
	case protocol.ChangePassword:
		s.changePassword(ctx, player, command)
	case protocol.ListSeeks:
		s.manager.seeks.SendList(s)
	case protocol.PostSeek:
		s.postSeek(player, command)
	case protocol.CancelSeek:
		s.manager.seeks.Cancel(s)
	case protocol.Rematch:
		s.rematch(player, command)
	case protocol.AcceptSeek:
		if _, err := s.manager.games.Accept(ctx, player, s, command.ID); err != nil {
			s.Send(protocol.NOK{})
		}
	case protocol.ListGames:
		s.manager.games.SendList(s)
	case protocol.PlaceStone:
		s.placeStone(ctx, player, command)
	case protocol.MoveStack:
		s.inGame(player, command.GameID, func(g *game.Game) error { return g.Move(ctx, player.Name, command.From, command.To, command.Drops) })
	case protocol.Resign:
		// The game announces the result; there's no OK.
		if g := s.gameFor(player, command.GameID); g == nil || g.Resign(ctx, player.Name) != nil {
			s.Send(protocol.NOK{})
		}
	case protocol.RequestUndo:
		s.inGame(player, command.GameID, func(g *game.Game) error { return g.RequestUndo(ctx, player.Name) })
	case protocol.RemoveUndo:
		s.inGame(player, command.GameID, func(g *game.Game) error { return g.RemoveUndo(ctx, player.Name) })
	case protocol.OfferDraw:
		s.inGame(player, command.GameID, func(g *game.Game) error { return g.OfferDraw(ctx, player.Name) })
	case protocol.RemoveDraw:
		s.inGame(player, command.GameID, func(g *game.Game) error { return g.RemoveDraw(ctx, player.Name) })
	case protocol.GiveTime:
		s.inGame(player, command.GameID, func(g *game.Game) error { return g.GiveTime(ctx, player.Name) })
	case protocol.Observe:
		// The game answers with what it has so far; there's no OK.
		if err := s.manager.games.Observe(ctx, player.Name, s, command.GameID); err != nil {
			s.Send(protocol.NOK{})
		}
	case protocol.Unobserve:
		if err := s.manager.games.Unobserve(ctx, s, command.GameID); err != nil {
			s.Send(protocol.NOK{})
		} else {
			s.Send(protocol.OK{})
		}
	case protocol.PostShout:
		s.shout(player, command)
	case protocol.PostRoomShout:
		if err := s.manager.chat.ShoutRoom(s.chatSender(player), command.Room, command.Text); err != nil {
			s.Send(protocol.CommandFailed{Reason: err.Error()})
		}
	case protocol.JoinRoom:
		s.manager.chat.Join(command.Room, s)
	case protocol.LeaveRoom:
		s.manager.chat.Leave(command.Room, s)
		s.Send(protocol.OK{})
	case protocol.SendTell:
		s.tell(player, command)
	case protocol.Sudo:
		s.sudo(ctx, player, command)
	case protocol.ShowGame:
		s.showGame(ctx, player, command)
	case protocol.ShowSquare:
		s.showSquare(ctx, player, command)
	default:
		s.Send(protocol.NOK{})
	}
}

// ip is the client's address, without a port.
func (s *Session) ip() string { return clientip.Host(s.conn.RemoteAddr()) }

// fail logs an unexpected error and tells the client the command failed.
func (s *Session) fail(op string, err error) {
	name := ""
	if p := s.player.Load(); p != nil {
		name = p.Name
	}
	s.log.Error(op, "player", name, "err", err)
	s.Send(protocol.NOK{})
}
