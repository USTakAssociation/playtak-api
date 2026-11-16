package session

import (
	"context"
	"errors"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/game"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/online"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/tak"
)

// gameFor returns the game with id if p is playing in it, else nil.
func (s *Session) gameFor(p *online.Player, id int) *game.Game {
	g := s.manager.games.ForPlayer(p.Name)
	if g == nil || g.ID() != id {
		return nil
	}
	return g
}

// inGame runs an action in p's game id, answering OK, or NOK with the
// reason if the game refused it.
func (s *Session) inGame(p *online.Player, id int, action func(*game.Game) error) {
	g := s.gameFor(p, id)
	if g == nil {
		s.Send(protocol.NOK{})
		return
	}
	var cooldown *game.CooldownError
	switch err := action(g); {
	case err == nil:
		s.Send(protocol.OK{})
	case errors.As(err, &cooldown):
		s.Send(protocol.ServerMessage{Text: cooldown.Error()})
		s.Send(protocol.NOK{})
	case errors.Is(err, game.ErrNotPlaying), errors.Is(err, game.ErrTimeout), errors.Is(err, game.ErrNothingToUndo),
		errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		s.Send(protocol.NOK{})
	default:
		s.Send(protocol.MoveRejected{Reason: err.Error()})
	}
}

func (s *Session) placeStone(ctx context.Context, player *online.Player, command protocol.PlaceStone) {
	kind := tak.Flat
	switch {
	case command.Capstone:
		kind = tak.Capstone
	case command.Wall:
		kind = tak.Wall
	}
	s.inGame(player, command.GameID, func(g *game.Game) error { return g.Place(ctx, player.Name, command.Square, kind) })
}

func (s *Session) showGame(ctx context.Context, player *online.Player, command protocol.ShowGame) {
	g := s.gameFor(player, command.GameID)
	if g == nil {
		s.Send(protocol.NOK{})
		return
	}
	if tps, err := g.TPS(ctx, player.Name); err == nil {
		s.Send(protocol.PositionShown{GameID: command.GameID, TPS: tps})
	} else {
		s.Send(protocol.NOK{})
	}
}

func (s *Session) showSquare(ctx context.Context, player *online.Player, command protocol.ShowSquare) {
	g := s.gameFor(player, command.GameID)
	if g == nil {
		s.Send(protocol.NOK{})
		return
	}
	if stack, err := g.SquareString(ctx, player.Name, command.Square); err == nil {
		s.Send(protocol.SquareShown{GameID: command.GameID, Stack: stack})
	} else {
		s.Send(protocol.NOK{})
	}
}
