package game

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/player"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/tak"
)

var (
	// ErrNotPlaying means the player isn't in this game, or it has ended.
	ErrNotPlaying = errors.New("not playing this game")
	// ErrTimeout means the game didn't answer in time.
	ErrTimeout = errors.New("game did not respond")
)

// Recipient is a connection a game can send to directly, e.g. to replay a
// game to a player who has just reconnected.
type Recipient interface {
	Send(ev protocol.Event)
}

// seat is one side of the board. Players are identified by name because a
// registered player who disconnects gets a new *player.Player on return.
type seat struct {
	name      string
	player    *player.Player
	connected bool
	away      int // counts disconnections, so a stale abandon timer can tell
}

// Game is a game in progress. All of its state is owned by its goroutine;
// other goroutines interact with it only through its methods.
type Game struct {
	info  protocol.GameInfo
	m     *Manager
	inbox chan func()
	done  chan struct{} // closed when the goroutine exits

	// Owned by the goroutine.
	engine *tak.Game
	clock  *clock
	seats  [2]*seat
	over   bool
}

func newGame(m *Manager, info protocol.GameInfo, white, black *player.Player) *Game {
	return &Game{
		info:   info,
		m:      m,
		inbox:  make(chan func(), 16),
		done:   make(chan struct{}),
		engine: tak.New(tak.Settings{Size: info.Size, Pieces: info.Pieces, Capstones: info.Capstones, Komi: info.Komi, Opening: info.Opening}),
		clock:  newClock(info.GameSettings),
		seats: [2]*seat{
			{name: white.Name, player: white, connected: true},
			{name: black.Name, player: black, connected: true},
		},
	}
}

// ID is the game's number, shared with the games DB.
func (g *Game) ID() int { return g.info.ID }

// Info describes the game for the game list.
func (g *Game) Info() protocol.GameInfo { return g.info }

func (g *Game) run() {
	defer close(g.done)
	for f := range g.inbox {
		f()
		if g.over {
			return
		}
	}
}

// do runs f on the game's goroutine and waits for it to finish.
func (g *Game) do(ctx context.Context, f func()) error {
	timeout := time.NewTimer(g.m.cfg.CommandTimeout)
	defer timeout.Stop()
	ran := make(chan struct{})
	select {
	case g.inbox <- func() { f(); close(ran) }:
	case <-g.done:
		return ErrNotPlaying
	case <-ctx.Done():
		return ctx.Err()
	case <-timeout.C:
		g.m.log.Error("game did not accept a command in time", "game", g.info.ID)
		return ErrTimeout
	}
	select {
	case <-ran:
		return nil
	case <-g.done:
		// The game ended. f may have been what ended it, or may never run.
		select {
		case <-ran:
			return nil
		default:
			return ErrNotPlaying
		}
	case <-timeout.C:
		g.m.log.Error("game did not finish a command in time", "game", g.info.ID)
		return ErrTimeout
	}
}

// post queues f to run on the game's goroutine without waiting, for timers.
func (g *Game) post(f func()) {
	select {
	case g.inbox <- f:
	case <-g.done:
	}
}

// Place plays a placement for the named player. A rule violation is
// returned as an error whose text explains it; ErrNotPlaying and ErrTimeout
// mean the move wasn't considered.
func (g *Game) Place(ctx context.Context, name, square string, kind tak.Kind) error {
	sq, ok := tak.ParseSquare(square)
	if !ok {
		return tak.ErrOffBoard
	}
	return g.play(ctx, name, func(c tak.Color) error { return g.engine.Place(c, sq, kind) })
}

// Move plays a stack move for the named player.
func (g *Game) Move(ctx context.Context, name, from, to string, drops []int) error {
	f, ok1 := tak.ParseSquare(from)
	t, ok2 := tak.ParseSquare(to)
	if !ok1 || !ok2 {
		return tak.ErrOffBoard
	}
	return g.play(ctx, name, func(c tak.Color) error { return g.engine.Move(c, f, t, drops) })
}

func (g *Game) play(ctx context.Context, name string, apply func(tak.Color) error) error {
	var result error
	err := g.do(ctx, func() {
		color, ok := g.colorOf(name)
		if !ok {
			result = ErrNotPlaying
			return
		}
		firstPly := g.engine.Ply() == 0
		if err := apply(color); err != nil {
			result = err
			return
		}
		g.clock.moved(color, firstPly, g.m.now())
		g.sendBoth(g.clock.event(g.info.ID))
		moves := g.engine.Moves()
		g.send(color.Opponent(), protocol.MovePlayed{GameID: g.info.ID, Move: moves[len(moves)-1]})
		if r := g.engine.Result(); r != tak.None {
			g.end(r, "")
		}
	})
	if err != nil {
		return err
	}
	return result
}

// Resign ends the game as a loss for the named player.
func (g *Game) Resign(ctx context.Context, name string) error {
	var result error
	err := g.do(ctx, func() {
		color, ok := g.colorOf(name)
		if !ok {
			result = ErrNotPlaying
			return
		}
		g.end(tak.Win(color.Opponent()), "")
	})
	if err != nil {
		return err
	}
	return result
}

// TPS returns the position for a player in the game.
func (g *Game) TPS(ctx context.Context, name string) (string, error) {
	var tps string
	var result error
	err := g.do(ctx, func() {
		if _, ok := g.colorOf(name); !ok {
			result = ErrNotPlaying
			return
		}
		tps = g.engine.TPS()
	})
	if err != nil {
		return "", err
	}
	return tps, result
}

// SquareString returns one square's stack for a player in the game.
func (g *Game) SquareString(ctx context.Context, name, square string) (string, error) {
	sq, _ := tak.ParseSquare(square)
	var s string
	var result error
	err := g.do(ctx, func() {
		if _, ok := g.colorOf(name); !ok {
			result = ErrNotPlaying
			return
		}
		s = g.engine.SquareString(sq)
	})
	if err != nil {
		return "", err
	}
	return s, result
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
		s := g.seats[color]
		s.connected = false
		s.away++
		away := s.away

		window := g.m.cfg.ReconnectTimeout
		if g.info.Tournament {
			window = g.m.cfg.TournamentReconnectTimeout
		}
		// On their own turn their clock is running too, so they may have
		// less time than the window; say whichever is sooner.
		seconds := int64(window / time.Second)
		if g.engine.Turn() == color && g.clock.started {
			g.clock.sync(color, g.m.now())
			left := (max(g.clock.remaining[color], 0) + 999) / 1000
			seconds = min(seconds, left)
		}
		g.send(color.Opponent(), protocol.ServerMessage{
			Text: fmt.Sprintf("%s has disconnected. They have %d seconds to reconnect", name, seconds),
		})

		time.AfterFunc(window, func() {
			g.post(func() {
				if !s.connected && s.away == away {
					g.end(tak.Win(color.Opponent()), name)
				}
			})
		})
	})
}

// Reconnected replays the game to a player's new connection: the start
// message, every move, the clock and a notice. Their opponent is told.
func (g *Game) Reconnected(ctx context.Context, p *player.Player, to Recipient) error {
	return g.do(ctx, func() {
		color, ok := g.colorOf(p.Name)
		if !ok {
			return
		}
		s := g.seats[color]
		s.player = p
		s.connected = true
		s.away++ // cancels any pending abandon

		to.Send(protocol.GameStarted{Game: g.info, Color: protocolColor(color)})
		for _, m := range g.engine.Moves() {
			to.Send(protocol.MovePlayed{GameID: g.info.ID, Move: m})
		}
		g.clock.sync(g.engine.Turn(), g.m.now())
		to.Send(g.clock.event(g.info.ID))
		to.Send(protocol.ServerMessage{Text: "Your game is resumed"})
		g.send(color.Opponent(), protocol.ServerMessage{Text: p.Name + " has reconnected."})
	})
}

// end finishes the game. quitter names a player who abandoned it, else "".
func (g *Game) end(r tak.Result, quitter string) {
	g.over = true
	if quitter != "" {
		// Only the player who stayed is told.
		for c, s := range g.seats {
			if s.name != quitter {
				g.send(tak.Color(c), protocol.GameAbandoned{GameID: g.info.ID, Quitter: quitter})
			}
		}
	} else {
		g.sendBoth(protocol.GameOver{GameID: g.info.ID, Result: r.String()})
	}
	g.m.finished(g, strings.Join(g.engine.Moves(), ","), r)
}

func (g *Game) colorOf(name string) (tak.Color, bool) {
	for c, s := range g.seats {
		if s.name == name {
			return tak.Color(c), true
		}
	}
	return 0, false
}

// send delivers ev to the player in seat c. A disconnected player gets
// nothing; the game is replayed to them when they return.
func (g *Game) send(c tak.Color, ev protocol.Event) {
	if s := g.seats[c]; s.connected && s.player != nil {
		s.player.Send(ev)
	}
}

func (g *Game) sendBoth(ev protocol.Event) {
	g.send(tak.White, ev)
	g.send(tak.Black, ev)
}

func protocolColor(c tak.Color) protocol.Color {
	if c == tak.White {
		return protocol.White
	}
	return protocol.Black
}
