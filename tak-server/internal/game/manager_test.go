package game

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/dbtest"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/games"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/online"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/seek"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/tak"
)

// client is a fake session.
type client struct {
	version int
	mu      sync.Mutex
	events  []protocol.Event
}

func (c *client) Send(ev protocol.Event) { c.mu.Lock(); c.events = append(c.events, ev); c.mu.Unlock() }
func (c *client) ProtocolVersion() int   { return c.version }
func (c *client) Disconnect()            {}
func (c *client) Done() <-chan struct{}  { return nil }

// take returns and clears the events received so far.
func (c *client) take() []protocol.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	ev := c.events
	c.events = nil
	return ev
}

// waitFor waits until the client has received an event matching match.
func (c *client) waitFor(t *testing.T, match func(protocol.Event) bool) protocol.Event {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		for _, ev := range c.events {
			if match(ev) {
				c.mu.Unlock()
				return ev
			}
		}
		c.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for an event")
	return nil
}

type savedResults struct {
	mu      sync.Mutex
	results []games.Result
}

func (s *savedResults) Save(r games.Result) error {
	s.mu.Lock()
	s.results = append(s.results, r)
	s.mu.Unlock()
	return nil
}

func (s *savedResults) get() []games.Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.results)
}

type failingStore struct{}

func (failingStore) Insert(context.Context, games.Row) (int, error) {
	return 0, errors.New("database is locked")
}

type env struct {
	t       *testing.T
	m       *Manager
	seeks   *seek.Manager
	players *online.Registry
	results *savedResults
	now     time.Time
	timers  []*fakeTimer
	mu      sync.Mutex
}

// fakeTimer fires when the env's clock is advanced past it.
type fakeTimer struct {
	e    *env
	at   time.Time
	f    func()
	done bool // fired or stopped
}

func (t *fakeTimer) Stop() bool {
	t.e.mu.Lock()
	defer t.e.mu.Unlock()
	was := !t.done
	t.done = true
	return was
}

func newEnv(t *testing.T, cfg Config) *env {
	t.Helper()
	if cfg.CommandTimeout == 0 {
		cfg.CommandTimeout = 5 * time.Second
	}
	if cfg.ReconnectTimeout == 0 {
		cfg.ReconnectTimeout = time.Minute
	}
	e := &env{t: t, seeks: seek.NewManager(), players: online.NewRegistry(), results: &savedResults{}, now: t0}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	e.m = NewManager(cfg, games.NewStore(dbtest.NewGamesDB(t)), players.NewStore(dbtest.NewPlayersDB(t)),
		e.results, e.seeks, e.players, log)
	e.m.now = func() time.Time { e.mu.Lock(); defer e.mu.Unlock(); return e.now }
	e.m.afterFunc = func(d time.Duration, f func()) timer {
		e.mu.Lock()
		defer e.mu.Unlock()
		t := &fakeTimer{e: e, at: e.now.Add(d), f: f}
		e.timers = append(e.timers, t)
		return t
	}
	return e
}

// advance moves the clock on and fires, in order, the timers that are due.
// Their callbacks run before advance returns, so work they post to a game
// is queued ahead of anything the test does next.
func (e *env) advance(d time.Duration) {
	e.mu.Lock()
	e.now = e.now.Add(d)
	var due []*fakeTimer
	for _, t := range e.timers {
		if !t.done && !t.at.After(e.now) {
			t.done = true
			due = append(due, t)
		}
	}
	e.mu.Unlock()
	slices.SortStableFunc(due, func(a, b *fakeTimer) int { return a.at.Compare(b.at) })
	for _, t := range due {
		t.f()
	}
}

// pending counts timers that are neither fired nor stopped.
func (e *env) pending() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, t := range e.timers {
		if !t.done {
			n++
		}
	}
	return n
}

// settle waits until g has handled everything queued so far.
func settle(g *Game) { g.do(context.Background(), func() {}) }

// login brings a guest online with one connection.
func (e *env) login(version int) (*online.Player, *client) {
	p := e.players.Guest("")
	c := &client{version: version}
	e.players.Attach(p, c)
	return p, c
}

// disconnect drops all of p's connections and then tells g, as a session
// closing does.
func (e *env) disconnect(g *Game, p *online.Player) {
	for _, c := range p.Clients() {
		e.players.Detach(p, c)
	}
	g.Disconnected(context.Background(), p.Name)
}

// seek posts a seek for p from its connection.
func (e *env) seek(p *online.Player, c *client, s protocol.GameSettings, opponent string) seek.Seek {
	e.t.Helper()
	sk, err := e.seeks.Post(c, seek.Request{Creator: p.Name, Opponent: opponent, Settings: s})
	if err != nil {
		e.t.Fatal(err)
	}
	return sk
}

var std5 = protocol.GameSettings{Size: 5, Time: 600, Increment: 30, Pieces: 21, Capstones: 1, Color: protocol.White}

// start has b accept a's white seek, so a plays white.
func (e *env) start() (g *Game, a *online.Player, ac *client, b *online.Player, bc *client) {
	e.t.Helper()
	a, ac = e.login(0)
	b, bc = e.login(0)
	sk := e.seek(a, ac, std5, "")
	g, err := e.m.Accept(context.Background(), b, bc, sk.ID)
	if err != nil {
		e.t.Fatal(err)
	}
	ac.take()
	bc.take()
	return g, a, ac, b, bc
}

func TestAcceptStartsGame(t *testing.T) {
	e := newEnv(t, Config{})
	a, ac := e.login(0)
	b, bc := e.login(4)
	watcher := &client{}
	e.m.Subscribe(watcher)
	sk := e.seek(a, ac, std5, "")
	e.seek(b, bc, std5, "") // b's own seek goes away when the game starts

	g, err := e.m.Accept(context.Background(), b, bc, sk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if g.ID() != 1 {
		t.Errorf("game id %d, want the DB row id", g.ID())
	}
	info := protocol.GameInfo{ID: 1, White: a.Name, Black: b.Name, GameSettings: std5}
	if got := watcher.take(); !slices.Equal(got, []protocol.Event{protocol.GameListAdded{Game: info}}) {
		t.Errorf("watcher got %v", got)
	}
	// The seek's creator asked for white.
	if got := ac.take(); len(got) != 1 || got[0] != (protocol.GameStarted{Game: info, Color: protocol.White}) {
		t.Errorf("creator got %v", got)
	}
	if got := bc.take(); len(got) != 1 || got[0] != (protocol.GameStarted{Game: info, Color: protocol.Black}) {
		t.Errorf("acceptor got %v", got)
	}
	if n := len(e.seeks.List()); n != 0 {
		t.Errorf("%d seeks left, want both players' seeks removed", n)
	}
	if e.m.ForPlayer(a.Name) != g || e.m.ForPlayer(b.Name) != g {
		t.Error("both players should be in the game")
	}
	if e.m.IfIdle(a.Name, func() {}) {
		t.Error("a player in a game isn't idle")
	}
}

func TestAcceptRefusals(t *testing.T) {
	e := newEnv(t, Config{})
	a, ac := e.login(0)
	b, bc := e.login(0)
	c, cc := e.login(0)
	ctx := context.Background()

	own := e.seek(a, ac, std5, "")
	if _, err := e.m.Accept(ctx, a, ac, own.ID); !errors.Is(err, ErrCantAccept) {
		t.Errorf("own seek: %v", err)
	}
	private := e.seek(b, bc, std5, "Somebody")
	if _, err := e.m.Accept(ctx, c, cc, private.ID); !errors.Is(err, ErrCantAccept) {
		t.Errorf("seek addressed to someone else: %v", err)
	}
	if _, err := e.m.Accept(ctx, c, cc, 999); !errors.Is(err, ErrCantAccept) {
		t.Errorf("missing seek: %v", err)
	}
	dbs := std5
	dbs.Opening = protocol.OpeningDoubleBlackStack
	modern := &client{version: 4}
	e.players.Attach(a, modern)
	sk, _ := e.seeks.Post(modern, seek.Request{Creator: a.Name, Settings: dbs})
	if _, err := e.m.Accept(ctx, c, cc, sk.ID); !errors.Is(err, ErrCantAccept) {
		t.Errorf("seek the acceptor's client can't display: %v", err)
	}
	// a has two connections, each with its own seek, plus b's.
	if n := len(e.seeks.List()); n != 3 {
		t.Errorf("refused accepts must not remove seeks: %d left", n)
	}
}

func TestCantAcceptOwnSeekFromAnotherConnection(t *testing.T) {
	e := newEnv(t, Config{AllowMultipleSessions: true})
	a, ac := e.login(0)
	second := &client{}
	e.players.Attach(a, second) // the same player in another tab
	own := e.seek(a, ac, std5, "")
	if _, err := e.m.Accept(context.Background(), a, second, own.ID); !errors.Is(err, ErrCantAccept) {
		t.Fatalf("accepting your own seek from another connection: %v", err)
	}
	if len(e.seeks.List()) != 1 || !e.m.IfIdle(a.Name, func() {}) {
		t.Error("a refused accept must leave the seek and the player as they were")
	}
}

func TestAcceptRefusedWhilePlaying(t *testing.T) {
	e := newEnv(t, Config{})
	_, a, _, _, _ := e.start()
	c, cc := e.login(0)
	sk := e.seek(c, cc, std5, "")
	extra := &client{}
	e.players.Attach(a, extra) // a second connection, not the one in the game
	if _, err := e.m.Accept(context.Background(), a, extra, sk.ID); !errors.Is(err, ErrCantAccept) {
		t.Errorf("acceptor already playing: %v", err)
	}
}

func TestConcurrentAcceptsStartOneGame(t *testing.T) {
	e := newEnv(t, Config{})
	a, ac := e.login(0)
	sk := e.seek(a, ac, std5, "")
	var wg sync.WaitGroup
	var mu sync.Mutex
	started := 0
	for range 8 {
		p, c := e.login(0)
		wg.Go(func() {
			if _, err := e.m.Accept(context.Background(), p, c, sk.ID); err == nil {
				mu.Lock()
				started++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	if started != 1 {
		t.Errorf("%d games started from one seek", started)
	}
}

func TestInsertFailureReleasesPlayers(t *testing.T) {
	e := newEnv(t, Config{})
	e.m.store = failingStore{}
	a, ac := e.login(0)
	b, bc := e.login(0)
	sk := e.seek(a, ac, std5, "")
	if _, err := e.m.Accept(context.Background(), b, bc, sk.ID); err == nil {
		t.Fatal("expected an error")
	}
	if !e.m.IfIdle(a.Name, func() {}) || !e.m.IfIdle(b.Name, func() {}) {
		t.Error("players must be free to play again")
	}
	// Both are told: the seek's owner and the acceptor.
	want := []protocol.Event{protocol.ServerMessage{Text: "Could not start the game. Please try again."}}
	if got := ac.take(); !slices.Equal(got, want) {
		t.Errorf("seek owner got %v", got)
	}
	if got := bc.take(); !slices.Equal(got, want) {
		t.Errorf("acceptor got %v", got)
	}
}

func TestMoves(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, ac, b, bc := e.start()
	ctx := context.Background()

	if err := g.Place(ctx, b.Name, "A1", tak.Flat); err == nil || err.Error() != "Not your turn" {
		t.Errorf("black first: %v", err)
	}
	if err := g.Place(ctx, "stranger", "A1", tak.Flat); !errors.Is(err, ErrNotPlaying) {
		t.Errorf("stranger: %v", err)
	}
	e.advance(5 * time.Second)
	if err := g.Place(ctx, a.Name, "A1", tak.Flat); err != nil {
		t.Fatal(err)
	}
	wantTime := protocol.GameTime{GameID: 1, White: 630_000, Black: 600_000}
	if got := ac.take(); !slices.Equal(got, []protocol.Event{wantTime}) {
		t.Errorf("mover got %v", got)
	}
	if got := bc.take(); !slices.Equal(got, []protocol.Event{wantTime, protocol.MovePlayed{GameID: 1, Move: "P A1"}}) {
		t.Errorf("opponent got %v", got)
	}

	e.advance(7 * time.Second)
	if err := g.Place(ctx, b.Name, "E5", tak.Flat); err != nil {
		t.Fatal(err)
	}
	if got := ac.take(); len(got) != 2 || got[0] != (protocol.GameTime{GameID: 1, White: 630_000, Black: 623_000}) {
		t.Errorf("after black's move: %v", got)
	}
	bc.take()
	// A1 holds black's stone (white placed it in the opening swap).
	if err := g.Move(ctx, a.Name, "A1", "A2", []int{1}); err == nil || err.Error() != "You don't control that stack" {
		t.Errorf("illegal move: %v", err)
	}
	if tps, _ := g.TPS(ctx, a.Name); tps != "x4,1/x5/x5/x5/2,x4 1 2" {
		t.Errorf("TPS %q", tps)
	}
	if sq, _ := g.SquareString(ctx, b.Name, "A1"); sq != "[f]" {
		t.Errorf("A1 %q", sq)
	}
	if _, err := g.TPS(ctx, "stranger"); !errors.Is(err, ErrNotPlaying) {
		t.Errorf("stranger TPS: %v", err)
	}
}

func TestRoadWinEndsGame(t *testing.T) {
	e := newEnv(t, Config{})
	watcher := &client{}
	e.m.Subscribe(watcher)
	g, a, ac, b, bc := e.start()
	watcher.take()
	ctx := context.Background()
	moves := []struct {
		name, sq string
	}{{a.Name, "E5"}, {b.Name, "A1"}, {a.Name, "B1"}, {b.Name, "A5"}, {a.Name, "C1"}, {b.Name, "B5"}, {a.Name, "D1"}, {b.Name, "C5"}}
	for _, m := range moves {
		if err := g.Place(ctx, m.name, m.sq, tak.Flat); err != nil {
			t.Fatal(m, err)
		}
	}
	ac.take()
	bc.take()
	if err := g.Place(ctx, a.Name, "E1", tak.Flat); err != nil {
		t.Fatal(err)
	}
	over := protocol.GameOver{GameID: 1, Result: "R-0"}
	if got := ac.take(); len(got) != 2 || got[1] != over {
		t.Errorf("winner got %v", got)
	}
	if got := bc.take(); len(got) != 3 || got[2] != over {
		t.Errorf("loser got %v", got)
	}
	if got := watcher.take(); len(got) != 1 || got[0] != (protocol.GameListRemoved{Game: g.Info()}) {
		t.Errorf("watcher got %v", got)
	}
	want := games.Result{ID: 1, Notation: "P E5,P A1,P B1,P A5,P C1,P B5,P D1,P C5,P E1", Result: "R-0"}
	if got := e.results.get(); len(got) != 1 || got[0] != want {
		t.Errorf("saved %+v", got)
	}
	if e.m.ForPlayer(a.Name) != nil {
		t.Error("players should be free after the game")
	}
	if err := g.Place(ctx, b.Name, "D5", tak.Flat); !errors.Is(err, ErrNotPlaying) {
		t.Errorf("move after game over: %v", err)
	}
}

func TestResign(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, ac, b, bc := e.start()
	if err := g.Resign(context.Background(), a.Name); err != nil {
		t.Fatal(err)
	}
	over := protocol.GameOver{GameID: 1, Result: "0-1"}
	if got := ac.take(); !slices.Equal(got, []protocol.Event{over}) {
		t.Errorf("white got %v", got)
	}
	if got := bc.take(); !slices.Equal(got, []protocol.Event{over}) {
		t.Errorf("black got %v", got)
	}
	if got := e.results.get(); len(got) != 1 || got[0].Result != "0-1" || got[0].Notation != "" {
		t.Errorf("saved %+v", got)
	}
	if err := g.Resign(context.Background(), b.Name); !errors.Is(err, ErrNotPlaying) {
		t.Errorf("resigning a finished game: %v", err)
	}
}

func TestDisconnectAndAbandon(t *testing.T) {
	e := newEnv(t, Config{ReconnectTimeout: 1500 * time.Millisecond})
	g, _, ac, b, bc := e.start()
	e.disconnect(g, b)
	msg := ac.take()
	if want := (protocol.ServerMessage{Text: b.Name + " has disconnected. They have 1 seconds to reconnect"}); len(msg) != 1 || msg[0] != want {
		t.Errorf("got %v, want %v (the window rounds down to whole seconds)", msg, want)
	}
	e.advance(1499 * time.Millisecond)
	settle(g)
	if len(e.results.get()) != 0 {
		t.Fatal("abandoned before the window closed")
	}
	e.advance(time.Millisecond)
	settle(g)
	if got := ac.take(); len(got) != 1 || got[0] != (protocol.GameAbandoned{GameID: 1, Quitter: b.Name}) {
		t.Errorf("stayer got %v", got)
	}
	if got := e.results.get(); len(got) != 1 || got[0].Result != "1-0" {
		t.Errorf("saved %+v", got)
	}
	for _, ev := range bc.take() {
		if _, ok := ev.(protocol.GameAbandoned); ok {
			t.Error("the player who left isn't told")
		}
	}
}

func TestTournamentReconnectWindow(t *testing.T) {
	e := newEnv(t, Config{ReconnectTimeout: time.Minute, TournamentReconnectTimeout: 15 * time.Minute})
	a, ac := e.login(0)
	b, bc := e.login(0)
	s := std5
	s.Tournament = true
	sk := e.seek(a, ac, s, "")
	g, err := e.m.Accept(context.Background(), b, bc, sk.ID)
	if err != nil {
		t.Fatal(err)
	}
	e.disconnect(g, b)
	ac.waitFor(t, func(ev protocol.Event) bool {
		return ev == protocol.ServerMessage{Text: b.Name + " has disconnected. They have 900 seconds to reconnect"}
	})
}

func TestDisconnectMessageUsesClockOnTheirTurn(t *testing.T) {
	e := newEnv(t, Config{ReconnectTimeout: 2 * time.Minute})
	g, a, ac, b, bc := e.start()
	ctx := context.Background()
	g.Place(ctx, a.Name, "A1", tak.Flat) // free first ply: white 630s
	g.Place(ctx, b.Name, "E5", tak.Flat) // instant: black 630s
	// White, to move, thinks until 40.5s remain.
	e.advance(630*time.Second - 40500*time.Millisecond)
	ac.take()
	bc.take()

	// Black isn't to move, so only the window counts.
	e.disconnect(g, b)
	ac.waitFor(t, func(ev protocol.Event) bool {
		return ev == protocol.ServerMessage{Text: b.Name + " has disconnected. They have 120 seconds to reconnect"}
	})
	// Black returns (a disconnected player is sent nothing), then white
	// leaves: white's clock runs out before the window, rounded up.
	e.players.Attach(b, bc)
	if err := g.Reconnected(ctx, b, bc); err != nil {
		t.Fatal(err)
	}
	e.disconnect(g, a)
	bc.waitFor(t, func(ev protocol.Event) bool {
		return ev == protocol.ServerMessage{Text: a.Name + " has disconnected. They have 41 seconds to reconnect"}
	})
}

func TestDisconnectedPlayerIsSentNothing(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, ac, b, bc := e.start()
	ctx := context.Background()
	e.disconnect(g, b)
	if err := g.Place(ctx, a.Name, "A1", tak.Flat); err != nil {
		t.Fatal(err)
	}
	if got := bc.take(); len(got) != 0 {
		t.Errorf("disconnected player was sent %v", got)
	}
	if got := ac.take(); len(got) < 2 {
		t.Errorf("the connected player still gets events: %v", got)
	}
}

func TestStaleDisconnectIsIgnored(t *testing.T) {
	e := newEnv(t, Config{ReconnectTimeout: time.Minute})
	g, _, ac, b, bc := e.start()
	// An old session's cleanup reports the disconnect after the player has
	// already logged in again: still connected, so nothing happens.
	g.Disconnected(context.Background(), b.Name)
	if got := ac.take(); len(got) != 0 {
		t.Errorf("opponent got %v for a player who is online", got)
	}
	e.advance(2 * time.Minute)
	settle(g)
	if len(e.results.get()) != 0 {
		t.Fatal("a stale disconnect must not abandon the game")
	}
	if got := bc.take(); len(got) != 0 {
		t.Errorf("the player still connected got %v", got)
	}
}

func TestReconnectCancelsAbandon(t *testing.T) {
	e := newEnv(t, Config{ReconnectTimeout: time.Minute})
	g, a, ac, b, _ := e.start()
	ctx := context.Background()
	g.Place(ctx, a.Name, "A1", tak.Flat)
	ac.take()

	e.disconnect(g, b)
	fresh := &client{}
	if err := g.Reconnected(ctx, b, fresh); err != nil {
		t.Fatal(err)
	}
	info := g.Info()
	want := []protocol.Event{
		protocol.GameStarted{Game: info, Color: protocol.Black},
		protocol.MovePlayed{GameID: 1, Move: "P A1"},
		protocol.GameTime{GameID: 1, White: 630_000, Black: 600_000},
		protocol.ServerMessage{Text: "Your game is resumed"},
	}
	if got := fresh.take(); !slices.Equal(got, want) {
		t.Errorf("rejoin sent\n got %v\nwant %v", got, want)
	}
	ac.waitFor(t, func(ev protocol.Event) bool { return ev == protocol.ServerMessage{Text: b.Name + " has reconnected."} })
	e.advance(2 * time.Minute)
	settle(g)
	if len(e.results.get()) != 0 || e.m.ForPlayer(a.Name) != g {
		t.Error("the game must continue after reconnecting")
	}
}

func TestCommandTimeout(t *testing.T) {
	e := newEnv(t, Config{CommandTimeout: 100 * time.Millisecond})
	g, a, _, _, _ := e.start()
	block := make(chan struct{})
	g.post(func() { <-block }) // wedge the game's goroutine
	defer close(block)
	if err := g.Place(context.Background(), a.Name, "A1", tak.Flat); !errors.Is(err, ErrTimeout) {
		t.Errorf("err %v, want ErrTimeout", err)
	}
}

func TestTimedOutCommandNeverRuns(t *testing.T) {
	e := newEnv(t, Config{CommandTimeout: 100 * time.Millisecond})
	g, a, ac, _, bc := e.start()
	block := make(chan struct{})
	g.post(func() { <-block }) // wedge the game's goroutine
	if err := g.Place(context.Background(), a.Name, "A1", tak.Flat); !errors.Is(err, ErrTimeout) {
		t.Fatalf("err %v, want ErrTimeout", err)
	}
	close(block) // the game catches up
	settle(g)
	// ErrTimeout means the move wasn't considered: it must not be played late.
	if moves := g.engine.Moves(); len(moves) != 0 {
		t.Errorf("timed-out move was played: %v", moves)
	}
	if got := append(ac.take(), bc.take()...); len(got) != 0 {
		t.Errorf("events sent for a timed-out move: %v", got)
	}
}

func TestCommandTimingOutWhileRunning(t *testing.T) {
	e := newEnv(t, Config{CommandTimeout: 100 * time.Millisecond})
	g, _, _, _, _ := e.start()
	block := make(chan struct{})
	defer close(block)
	// Started, but not finished in time: the outcome can't be known.
	if err := g.do(context.Background(), func() { <-block }); !errors.Is(err, ErrOutcomeUnknown) {
		t.Errorf("err %v, want ErrOutcomeUnknown", err)
	}
}

func TestGameListGating(t *testing.T) {
	e := newEnv(t, Config{})
	a, ac := e.login(4)
	b, bc := e.login(4)
	old, modern := &client{version: 3}, &client{version: 4}
	e.m.Subscribe(old)
	e.m.Subscribe(modern)
	dbs := std5
	dbs.Opening = protocol.OpeningDoubleBlackStack
	sk := e.seek(a, ac, dbs, "")
	if _, err := e.m.Accept(context.Background(), b, bc, sk.ID); err != nil {
		t.Fatal(err)
	}
	if got := old.take(); len(got) != 0 {
		t.Errorf("old client was sent %v", got)
	}
	if got := modern.take(); len(got) != 1 {
		t.Errorf("modern client got %v", got)
	}
	late := &client{version: 4}
	e.m.Subscribe(late)
	if got := late.take(); len(got) != 1 {
		t.Errorf("Subscribe should send the current game list: %v", got)
	}
	e.m.SendList(old)
	if got := old.take(); len(got) != 0 {
		t.Errorf("GameList showed an incompatible game: %v", got)
	}
}

func TestBotFlag(t *testing.T) {
	e := newEnv(t, Config{})
	a, ac := e.login(0)
	bot := e.players.Acquire(players.Record{ID: 50, Name: "TakBot", IsBot: true})
	bc := &client{}
	e.players.Attach(bot, bc)
	sk := e.seek(a, ac, std5, "")
	g, err := e.m.Accept(context.Background(), bot, bc, sk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !g.Info().Bot {
		t.Error("a game with a bot in either seat is a bot game")
	}
}

func TestForfeit(t *testing.T) {
	e := newEnv(t, Config{})
	_, a, ac, b, bc := e.start()
	ctx := context.Background()
	if had, err := e.m.Forfeit(ctx, "nobody"); had || err != nil {
		t.Errorf("no game: %v %v", had, err)
	}
	if had, err := e.m.Forfeit(ctx, b.Name); !had || err != nil {
		t.Fatalf("%v %v", had, err)
	}
	for _, c := range []*client{ac, bc} {
		if got := c.take(); !slices.Equal(got, []protocol.Event{protocol.GameOver{GameID: 1, Result: "1-0"}}) {
			t.Errorf("got %v", got)
		}
	}
	if e.m.ForPlayer(a.Name) != nil {
		t.Error("game should be over")
	}
}
