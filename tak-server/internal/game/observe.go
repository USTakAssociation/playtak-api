package game

import (
	"context"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

func (g *Game) sendObservers(event protocol.Event) {
	for o := range g.observers {
		o.Send(event)
	}
}

// observe adds o, watching for the named player, and sends them the game so
// far: the header, every move and the clock. Running on the game's
// goroutine, no move can slip in between.
func (g *Game) observe(ctx context.Context, name string, observer Listener) error {
	var result error
	err := g.do(ctx, func() {
		if _, seated := g.colorOf(name); seated || observer.ProtocolVersion() < g.info.RequiredProtocolVersion() {
			result = ErrCantObserve
			return
		}
		if !g.manager.addObserver(observer, g, name) {
			result = ErrCantObserve
			return
		}
		g.observers[observer] = struct{}{}
		observer.Send(protocol.GameObserved{Game: g.info})
		for _, m := range g.engine.Moves() {
			observer.Send(protocol.MovePlayed{GameID: g.info.ID, Move: m})
		}
		g.clock.sync(g.engine.Turn(), g.manager.now())
		observer.Send(g.clock.event(g.info.ID))
	})
	if err != nil {
		return err
	}
	return result
}

// unobserve stops sending events to o.
func (g *Game) unobserve(o Listener) {
	delete(g.observers, o)
	g.manager.removeObserver(o, g)
}
