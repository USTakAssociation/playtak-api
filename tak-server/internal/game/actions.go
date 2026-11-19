package game

import (
	"context"
	"fmt"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/tak"
)

const (
	giveTimeAmount   = 15 * time.Second
	giveTimeCooldown = 15 * time.Second
)

// CooldownError means the player gave time too recently.
type CooldownError struct{ Left time.Duration }

func (e *CooldownError) Error() string {
	return fmt.Sprintf("You can give time again in %d seconds", e.Seconds())
}

// Seconds is the time left, rounded up.
func (e *CooldownError) Seconds() int64 {
	return int64((e.Left + time.Second - 1) / time.Second)
}

// RequestUndo asks to take back the last ply, or agrees if the opponent has
// already asked. Asking twice does nothing.
func (g *Game) RequestUndo(ctx context.Context, name string) error {
	return g.act(ctx, name, func(color tak.Color) error {
		if g.engine.Ply() == 0 {
			return ErrNothingToUndo
		}
		switch opponent := color.Opponent(); {
		case g.undoAsked[opponent]:
			g.undoAsked = [2]bool{}
			g.engine.Undo()
			g.clock.undo(g.manager.now())
			g.sendAll(g.clock.event(g.info.ID))
			g.sendAll(protocol.Undone{GameID: g.info.ID})
			g.resetTimeout()
		case !g.undoAsked[color]:
			g.undoAsked[color] = true
			g.send(opponent, protocol.UndoRequested{GameID: g.info.ID})
		}
		return nil
	})
}

// RemoveUndo withdraws the named player's undo request, if they made one.
func (g *Game) RemoveUndo(ctx context.Context, name string) error {
	return g.act(ctx, name, func(color tak.Color) error {
		if g.undoAsked[color] {
			g.undoAsked[color] = false
			g.send(color.Opponent(), protocol.UndoRemoved{GameID: g.info.ID})
		}
		return nil
	})
}

// OfferDraw offers a draw, or agrees to one if the opponent has already
// offered. Offering twice does nothing. Unlike undo requests, offers stay
// open when a move is played.
func (g *Game) OfferDraw(ctx context.Context, name string) error {
	return g.act(ctx, name, func(color tak.Color) error {
		switch opponent := color.Opponent(); {
		case g.drawOffered[opponent]:
			g.end(tak.Draw, "")
		case !g.drawOffered[color]:
			g.drawOffered[color] = true
			g.send(opponent, protocol.DrawOffered{GameID: g.info.ID})
		}
		return nil
	})
}

// RemoveDraw withdraws the named player's draw offer, if they made one.
func (g *Game) RemoveDraw(ctx context.Context, name string) error {
	return g.act(ctx, name, func(color tak.Color) error {
		if g.drawOffered[color] {
			g.drawOffered[color] = false
			g.send(color.Opponent(), protocol.DrawRemoved{GameID: g.info.ID})
		}
		return nil
	})
}

// GiveTime adds giveTimeAmount to the opponent's clock. A player can give
// once per giveTimeCooldown; too soon returns a *CooldownError.
func (g *Game) GiveTime(ctx context.Context, name string) error {
	return g.act(ctx, name, func(color tak.Color) error {
		if !g.clock.started {
			return ErrClockNotStarted
		}
		now := g.manager.now()
		if last := g.lastGift[color]; !last.IsZero() {
			if left := giveTimeCooldown - now.Sub(last); left > 0 {
				return &CooldownError{Left: left}
			}
		}
		g.lastGift[color] = now
		opponent := color.Opponent()
		g.clock.give(opponent, giveTimeAmount.Milliseconds())
		g.sendAll(protocol.TimeGiven{GameID: g.info.ID, To: protocolColor(opponent), Millis: giveTimeAmount.Milliseconds()})
		g.sendAll(g.clock.event(g.info.ID))
		g.resetTimeout()
		return nil
	})
}
