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
	// ErrNothingToUndo means no ply has been played.
	ErrNothingToUndo = errors.New("nothing to undo")
	// ErrCantObserve means the game can't be observed: it's gone, it's the
	// observer's own, or their client can't display it.
	ErrCantObserve = errors.New("game can't be observed")
	// ErrClockNotStarted means time can't be given before the first ply.
	ErrClockNotStarted = errors.New("The clock has not started")
)

const (
	giveTimeAmount   = 15 * time.Second
	giveTimeCooldown = 15 * time.Second
	// botFirstMoveLimit is the most a human playing white against a bot may
	// take over their first move before the bot is awarded the game, so bots
	// aren't held up. A game with less starting time uses that instead.
	botFirstMoveLimit = 5 * time.Minute
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

	timeout    timer // fires when the player to move runs out of time
	timeoutGen int   // bumped whenever the timeout is reset, so a stale one can tell
	botTimer   timer // see botFirstMoveLimit; nil when it doesn't apply

	undoAsked   [2]bool      // by color; at most one is set
	drawOffered [2]bool      // by color; at most one is set
	lastGift    [2]time.Time // when each color last gave time

	observers map[Listener]struct{}
}

func newGame(m *Manager, info protocol.GameInfo, white, black *player.Player) *Game {
	return &Game{
		info:      info,
		m:         m,
		inbox:     make(chan func(), 16),
		done:      make(chan struct{}),
		engine:    tak.New(tak.Settings{Size: info.Size, Pieces: info.Pieces, Capstones: info.Capstones, Komi: info.Komi, Opening: info.Opening}),
		clock:     newClock(info.GameSettings),
		observers: make(map[Listener]struct{}),
		seats: [2]*seat{
			{name: white.Name, player: white, connected: true},
			{name: black.Name, player: black, connected: true},
		},
	}
}

// start arms the timer that frees a bot playing black whose opponent never
// makes the first move. It runs before the goroutine starts.
func (g *Game) start() {
	if !g.seats[tak.Black].player.IsBot() {
		return
	}
	limit := botFirstMoveLimit
	if t := time.Duration(g.info.Time) * time.Second; t > 0 && t < limit {
		limit = t
	}
	g.botTimer = g.m.afterFunc(limit, func() {
		g.post(func() {
			if g.engine.Ply() == 0 {
				g.end(tak.Win(tak.Black), "")
			}
		})
	})
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
	return g.act(ctx, name, func(color tak.Color) error {
		if err := apply(color); err != nil {
			return err
		}
		if g.botTimer != nil {
			g.botTimer.Stop()
			g.botTimer = nil
		}
		g.undoAsked = [2]bool{} // a move withdraws any undo request, silently
		g.clock.moved(color, g.m.now())
		g.sendAll(g.clock.event(g.info.ID))
		moves := g.engine.Moves()
		move := protocol.MovePlayed{GameID: g.info.ID, Move: moves[len(moves)-1]}
		g.send(color.Opponent(), move)
		g.sendObservers(move)
		if r := g.engine.Result(); r != tak.None {
			g.end(r, "")
			return nil
		}
		g.resetTimeout()
		return nil
	})
}

// Resign ends the game as a loss for the named player.
func (g *Game) Resign(ctx context.Context, name string) error {
	return g.act(ctx, name, func(color tak.Color) error {
		g.end(tak.Win(color.Opponent()), "")
		return nil
	})
}

// RequestUndo asks to take back the last ply, or agrees if the opponent has
// already asked. Asking twice does nothing.
func (g *Game) RequestUndo(ctx context.Context, name string) error {
	return g.act(ctx, name, func(color tak.Color) error {
		if g.engine.Ply() == 0 {
			return ErrNothingToUndo
		}
		switch opp := color.Opponent(); {
		case g.undoAsked[opp]:
			g.undoAsked = [2]bool{}
			g.engine.Undo()
			g.clock.undo(g.m.now())
			g.sendAll(g.clock.event(g.info.ID))
			g.sendAll(protocol.Undone{GameID: g.info.ID})
			g.resetTimeout()
		case !g.undoAsked[color]:
			g.undoAsked[color] = true
			g.send(opp, protocol.UndoRequested{GameID: g.info.ID})
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
		switch opp := color.Opponent(); {
		case g.drawOffered[opp]:
			g.end(tak.Draw, "")
		case !g.drawOffered[color]:
			g.drawOffered[color] = true
			g.send(opp, protocol.DrawOffered{GameID: g.info.ID})
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
		now := g.m.now()
		if last := g.lastGift[color]; !last.IsZero() {
			if left := giveTimeCooldown - now.Sub(last); left > 0 {
				return &CooldownError{Left: left}
			}
		}
		g.lastGift[color] = now
		opp := color.Opponent()
		g.clock.give(opp, giveTimeAmount.Milliseconds())
		g.sendAll(protocol.TimeGiven{GameID: g.info.ID, To: protocolColor(opp), Millis: giveTimeAmount.Milliseconds()})
		g.sendAll(g.clock.event(g.info.ID))
		g.resetTimeout()
		return nil
	})
}

// act runs f on the game's goroutine for the named player, first ending the
// game if the player to move has run out of time.
func (g *Game) act(ctx context.Context, name string, f func(tak.Color) error) error {
	var result error
	err := g.do(ctx, func() {
		color, ok := g.colorOf(name)
		if !ok || g.timeUp() {
			result = ErrNotPlaying
			return
		}
		result = f(color)
	})
	if err != nil {
		return err
	}
	return result
}

// timeUp ends the game if the player to move has run out of time.
func (g *Game) timeUp() bool {
	if !g.clock.started {
		return false
	}
	turn := g.engine.Turn()
	g.clock.sync(turn, g.m.now())
	if g.clock.remaining[turn] > 0 {
		return false
	}
	g.sendAll(g.clock.event(g.info.ID))
	g.end(tak.Win(turn.Opponent()), "")
	return true
}

// resetTimeout sets the out-of-time timer for the player to move, replacing any
// earlier one. A timer that fires early sets itself again.
func (g *Game) resetTimeout() {
	if g.timeout != nil {
		g.timeout.Stop()
	}
	g.timeoutGen++
	if g.over || !g.clock.started {
		return
	}
	gen := g.timeoutGen
	turn := g.engine.Turn()
	g.clock.sync(turn, g.m.now())
	d := time.Duration(max(g.clock.remaining[turn], 0)) * time.Millisecond
	g.timeout = g.m.afterFunc(d, func() {
		g.post(func() {
			if gen == g.timeoutGen && !g.timeUp() {
				g.resetTimeout()
			}
		})
	})
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
		msg := protocol.ServerMessage{Text: fmt.Sprintf("%s has disconnected. They have %d seconds to reconnect", name, seconds)}
		g.send(color.Opponent(), msg)
		g.sendObservers(msg)

		g.m.afterFunc(window, func() {
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
		// Requests still waiting for this player's answer.
		opp := color.Opponent()
		if g.drawOffered[opp] {
			to.Send(protocol.DrawOffered{GameID: g.info.ID})
		}
		if g.undoAsked[opp] {
			to.Send(protocol.UndoRequested{GameID: g.info.ID})
		}
		msg := protocol.ServerMessage{Text: p.Name + " has reconnected."}
		g.send(opp, msg)
		g.sendObservers(msg)
	})
}

// end finishes the game. quitter names a player who abandoned it, else "".
func (g *Game) end(r tak.Result, quitter string) {
	g.over = true
	for _, t := range []timer{g.timeout, g.botTimer} {
		if t != nil {
			t.Stop()
		}
	}
	if quitter != "" {
		// The player who stayed and the observers are told.
		ev := protocol.GameAbandoned{GameID: g.info.ID, Quitter: quitter}
		for c, s := range g.seats {
			if s.name != quitter {
				g.send(tak.Color(c), ev)
			}
		}
		g.sendObservers(ev)
	} else {
		g.sendAll(protocol.GameOver{GameID: g.info.ID, Result: r.String()})
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

// sendAll delivers ev to both players and every observer.
func (g *Game) sendAll(ev protocol.Event) {
	g.send(tak.White, ev)
	g.send(tak.Black, ev)
	g.sendObservers(ev)
}

func (g *Game) sendObservers(ev protocol.Event) {
	for o := range g.observers {
		o.Send(ev)
	}
}

// observe adds o, watching for the named player, and sends them the game so
// far: the header, every move and the clock. Running on the game's
// goroutine, no move can slip in between.
func (g *Game) observe(ctx context.Context, name string, o Listener) error {
	var result error
	err := g.do(ctx, func() {
		if _, seated := g.colorOf(name); seated || o.ProtocolVersion() < g.info.RequiredProtocolVersion() {
			result = ErrCantObserve
			return
		}
		if !g.m.addObserver(o, g, name) {
			result = ErrCantObserve
			return
		}
		g.observers[o] = struct{}{}
		o.Send(protocol.GameObserved{Game: g.info})
		for _, m := range g.engine.Moves() {
			o.Send(protocol.MovePlayed{GameID: g.info.ID, Move: m})
		}
		g.clock.sync(g.engine.Turn(), g.m.now())
		o.Send(g.clock.event(g.info.ID))
	})
	if err != nil {
		return err
	}
	return result
}

// unobserve stops sending events to o.
func (g *Game) unobserve(o Listener) {
	delete(g.observers, o)
	g.m.removeObserver(o, g)
}

func protocolColor(c tak.Color) protocol.Color {
	if c == tak.White {
		return protocol.White
	}
	return protocol.Black
}
