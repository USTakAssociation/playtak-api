package game

import (
	"context"
	"fmt"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/online"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/tak"
)

// Recipient is a connection a game can send to directly, e.g. to replay a
// game to a player who has just reconnected.
type Recipient interface {
	Send(event protocol.Event)
}

// Disconnected tells the game the named player has lost their last
// connection. If they don't return within the reconnect window, they lose.
// It returns once their opponent has been told.
func (g *Game) Disconnected(ctx context.Context, name string) error {
	return g.do(ctx, func() {
		color, ok := g.colorOf(name)
		if !ok || !g.seats[color].connected {
			return
		}
		seat := g.seats[color]
		seat.connected = false
		seat.away++
		away := seat.away

		window := g.manager.config.ReconnectTimeout
		if g.info.Tournament {
			window = g.manager.config.TournamentReconnectTimeout
		}
		// On their own turn their clock is running too, so they may have
		// less time than the window; say whichever is sooner.
		seconds := int64(window / time.Second)
		if g.engine.Turn() == color && g.clock.started {
			g.clock.sync(color, g.manager.now())
			left := (max(g.clock.remaining[color], 0) + 999) / 1000
			seconds = min(seconds, left)
		}
		message := protocol.ServerMessage{Text: fmt.Sprintf("%s has disconnected. They have %d seconds to reconnect", name, seconds)}
		g.send(color.Opponent(), message)
		g.sendObservers(message)

		g.manager.afterFunc(window, func() {
			g.post(func() {
				if !seat.connected && seat.away == away {
					g.end(tak.Win(color.Opponent()), name)
				}
			})
		})
	})
}

// Reconnected replays the game to a player's new connection: the start
// message, every move, the clock and a notice. Their opponent is told.
func (g *Game) Reconnected(ctx context.Context, player *online.Player, to Recipient) error {
	return g.do(ctx, func() {
		color, ok := g.colorOf(player.Name)
		if !ok {
			return
		}
		s := g.seats[color]
		s.player = player
		s.connected = true
		s.away++ // cancels any pending abandon

		to.Send(protocol.GameStarted{Game: g.info, Color: protocolColor(color)})
		for _, m := range g.engine.Moves() {
			to.Send(protocol.MovePlayed{GameID: g.info.ID, Move: m})
		}
		g.clock.sync(g.engine.Turn(), g.manager.now())
		to.Send(g.clock.event(g.info.ID))
		to.Send(protocol.ServerMessage{Text: "Your game is resumed"})
		// Requests still waiting for this player's answer.
		opponent := color.Opponent()
		if g.drawOffered[opponent] {
			to.Send(protocol.DrawOffered{GameID: g.info.ID})
		}
		if g.undoAsked[opponent] {
			to.Send(protocol.UndoRequested{GameID: g.info.ID})
		}
		message := protocol.ServerMessage{Text: player.Name + " has reconnected."}
		g.send(opponent, message)
		g.sendObservers(message)
	})
}
