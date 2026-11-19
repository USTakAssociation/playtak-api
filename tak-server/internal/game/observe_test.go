package game

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/tak"
)

// watch has a new guest observe game id and returns their connection with
// the opening events cleared.
func (e *env) watch(id int) (name string, c *client) {
	e.t.Helper()
	p, c := e.login(0)
	if err := e.m.Observe(context.Background(), p.Name, c, id); err != nil {
		e.t.Fatal(err)
	}
	c.take()
	return p.Name, c
}

func (e *env) observingCount() int {
	e.m.mu.Lock()
	defer e.m.mu.Unlock()
	return len(e.m.observing)
}

func TestObserveSendsGameSoFar(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, _, b, _ := e.start()
	play(t, g, a.Name, b.Name, "A1", "E5")
	e.advance(3 * time.Second)

	p, c := e.login(0)
	if err := e.m.Observe(context.Background(), p.Name, c, 1); err != nil {
		t.Fatal(err)
	}
	want := []protocol.Event{
		protocol.GameObserved{Game: g.Info()},
		protocol.MovePlayed{GameID: 1, Move: "P A1"},
		protocol.MovePlayed{GameID: 1, Move: "P E5"},
		protocol.GameTime{GameID: 1, White: 627_000, Black: 630_000},
	}
	if got := c.take(); !slices.Equal(got, want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}

	// Observing again starts over, so a client can resync.
	if err := e.m.Observe(context.Background(), p.Name, c, 1); err != nil {
		t.Fatal(err)
	}
	if got := c.take(); len(got) != len(want) || got[0] != want[0] {
		t.Errorf("second observe sent %v", got)
	}
	// Still only one subscription: a move arrives once.
	play(t, g, a.Name, b.Name, "B1")
	if got := c.take(); len(got) != 2 {
		t.Errorf("after observing twice a move was sent as %v", got)
	}
}

func TestObserveBeforeAnyMove(t *testing.T) {
	e := newEnv(t, Config{})
	g, _, _, _, _ := e.start()
	p, c := e.login(0)
	e.m.Observe(context.Background(), p.Name, c, 1)
	want := []protocol.Event{protocol.GameObserved{Game: g.Info()}, protocol.GameTime{GameID: 1, White: 600_000, Black: 600_000}}
	if got := c.take(); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v (Java sends the clock even before a move)", got, want)
	}
}

func TestObserverReceivesGameEvents(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, _, b, bc := e.startWith(blitz)
	ctx := context.Background()
	_, obs := e.watch(1)

	play(t, g, a.Name, b.Name, "A1")
	wantMove := []protocol.Event{protocol.GameTime{GameID: 1, White: 60_000, Black: 60_000}, protocol.MovePlayed{GameID: 1, Move: "P A1"}}
	if got := obs.take(); !slices.Equal(got, wantMove) {
		t.Errorf("move: got %v, want %v", got, wantMove)
	}

	// Requests are between the players only.
	g.OfferDraw(ctx, a.Name)
	g.RemoveDraw(ctx, a.Name)
	g.RequestUndo(ctx, a.Name)
	g.RemoveUndo(ctx, a.Name)
	if got := obs.take(); len(got) != 0 {
		t.Errorf("observer saw requests: %v", got)
	}

	// An agreed undo and a gift are seen.
	g.RequestUndo(ctx, a.Name)
	g.RequestUndo(ctx, b.Name)
	if got := obs.take(); len(got) != 2 || got[1] != (protocol.Undone{GameID: 1}) {
		t.Errorf("undo: %v", got)
	}
	play(t, g, a.Name, b.Name, "A1")
	obs.take()
	g.GiveTime(ctx, a.Name)
	if got := obs.take(); len(got) != 2 || got[0] != (protocol.TimeGiven{GameID: 1, To: protocol.Black, Millis: 15_000}) {
		t.Errorf("give time: %v", got)
	}

	// Disconnect and reconnect notices.
	e.disconnect(g, b)
	g.Reconnected(ctx, b, bc)
	want := []protocol.Event{
		protocol.ServerMessage{Text: b.Name + " has disconnected. They have 60 seconds to reconnect"},
		protocol.ServerMessage{Text: b.Name + " has reconnected."},
	}
	if got := obs.take(); !slices.Equal(got, want) {
		t.Errorf("notices: got %v, want %v", got, want)
	}

	g.Resign(ctx, b.Name)
	if got := obs.take(); !slices.Equal(got, []protocol.Event{over("1-0")}) {
		t.Errorf("game over: %v", got)
	}
	if n := e.observingCount(); n != 0 {
		t.Errorf("%d sessions still recorded as observing an ended game", n)
	}
}

func TestObserverToldOfAbandon(t *testing.T) {
	e := newEnv(t, Config{ReconnectTimeout: time.Minute})
	g, _, _, b, _ := e.start()
	_, obs := e.watch(1)
	e.disconnect(g, b)
	e.advance(time.Minute)
	settle(g)
	got := obs.take()
	if len(got) != 2 || got[1] != (protocol.GameAbandoned{GameID: 1, Quitter: b.Name}) {
		t.Errorf("got %v", got)
	}
}

func TestObserverToldOfTimeout(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, _, b, _ := e.startWith(blitz)
	play(t, g, a.Name, b.Name, "A1", "E5")
	_, obs := e.watch(1)
	e.advance(60 * time.Second)
	settle(g)
	want := []protocol.Event{protocol.GameTime{GameID: 1, White: 0, Black: 60_000}, over("0-1")}
	if got := obs.take(); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestUnobserve(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, _, b, _ := e.start()
	ctx := context.Background()
	_, obs := e.watch(1)
	if err := e.m.Unobserve(ctx, obs, 1); err != nil {
		t.Fatal(err)
	}
	play(t, g, a.Name, b.Name, "A1")
	if got := obs.take(); len(got) != 0 {
		t.Errorf("unobserved but got %v", got)
	}
	if n := e.observingCount(); n != 0 {
		t.Errorf("%d observations recorded", n)
	}
	// Like Java: fine for a game you weren't observing, refused for one
	// that doesn't exist or has ended.
	if err := e.m.Unobserve(ctx, &client{}, 1); err != nil {
		t.Errorf("not observing: %v", err)
	}
	if err := e.m.Unobserve(ctx, obs, 99); !errors.Is(err, ErrNotPlaying) {
		t.Errorf("unknown game: %v", err)
	}
	g.Resign(ctx, a.Name)
	if err := e.m.Unobserve(ctx, obs, 1); !errors.Is(err, ErrNotPlaying) {
		t.Errorf("ended game: %v", err)
	}
}

func TestUnobserveAll(t *testing.T) {
	e := newEnv(t, Config{})
	g1, a1, _, b1, _ := e.start()
	g2, a2, _, b2, _ := e.start()
	p, obs := e.login(0)
	ctx := context.Background()
	e.m.Observe(ctx, p.Name, obs, g1.ID())
	e.m.Observe(ctx, p.Name, obs, g2.ID())
	obs.take()
	e.m.UnobserveAll(obs)
	settle(g1)
	settle(g2)
	play(t, g1, a1.Name, b1.Name, "A1")
	play(t, g2, a2.Name, b2.Name, "A1")
	if got := obs.take(); len(got) != 0 {
		t.Errorf("got %v", got)
	}
	if n := e.observingCount(); n != 0 {
		t.Errorf("%d observations recorded", n)
	}
}

func TestObserveRefusals(t *testing.T) {
	e := newEnv(t, Config{})
	ctx := context.Background()
	g, a, ac, _, _ := e.start()

	if err := e.m.Observe(ctx, a.Name, ac, g.ID()); !errors.Is(err, ErrCantObserve) {
		t.Errorf("own game: %v", err)
	}
	p, c := e.login(0)
	if err := e.m.Observe(ctx, p.Name, c, 99); !errors.Is(err, ErrCantObserve) {
		t.Errorf("unknown game: %v", err)
	}

	// A game the observer's client can't display.
	x, xc := e.login(4)
	y, yc := e.login(4)
	dbs := std5
	dbs.Opening = protocol.OpeningDoubleBlackStack
	sk := e.seek(x, xc, dbs, "")
	dbsGame, err := e.m.Accept(ctx, y, yc, sk.ID)
	if err != nil {
		t.Fatal(err)
	}
	old := &client{version: 3}
	if err := e.m.Observe(ctx, p.Name, old, dbsGame.ID()); !errors.Is(err, ErrCantObserve) {
		t.Errorf("incompatible game: %v", err)
	}
	if got := old.take(); len(got) != 0 {
		t.Errorf("refused observer was sent %v", got)
	}

	g.Resign(ctx, a.Name)
	if err := e.m.Observe(ctx, p.Name, c, g.ID()); !errors.Is(err, ErrCantObserve) {
		t.Errorf("ended game: %v", err)
	}
	if n := e.observingCount(); n != 0 {
		t.Errorf("refusals recorded %d observations", n)
	}
}

func TestSingleSessionCantObserveWhilePlaying(t *testing.T) {
	e := newEnv(t, Config{})
	e.start()
	_, a, ac, _, _ := e.start() // a plays game 2
	if err := e.m.Observe(context.Background(), a.Name, ac, 1); !errors.Is(err, ErrCantObserve) {
		t.Errorf("err %v", err)
	}
}

func TestSingleSessionStartingAGameStopsObserving(t *testing.T) {
	e := newEnv(t, Config{})
	g1, a1, _, b1, _ := e.start()
	ctx := context.Background()

	// p observes game 1, then starts game 2 by accepting q's seek. q was
	// observing too.
	p, pc := e.login(0)
	q, qc := e.login(0)
	e.m.Observe(ctx, p.Name, pc, g1.ID())
	e.m.Observe(ctx, q.Name, qc, g1.ID())
	sk := e.seek(q, qc, std5, "")
	if _, err := e.m.Accept(ctx, p, pc, sk.ID); err != nil {
		t.Fatal(err)
	}
	settle(g1)
	pc.take()
	qc.take()
	play(t, g1, a1.Name, b1.Name, "A1")
	for _, c := range []*client{pc, qc} {
		for _, ev := range c.take() {
			if ev, ok := ev.(protocol.MovePlayed); ok && ev.GameID == g1.ID() {
				t.Errorf("still observing game 1: %v", ev)
			}
		}
	}
}

func TestMultiSessionMayObserveWhilePlaying(t *testing.T) {
	e := newEnv(t, Config{AllowMultipleSessions: true})
	g1, a1, _, b1, _ := e.start()
	ctx := context.Background()
	p, pc := e.login(0)
	q, qc := e.login(0)
	e.m.Observe(ctx, p.Name, pc, g1.ID())
	sk := e.seek(q, qc, std5, "")
	g2, err := e.m.Accept(ctx, p, pc, sk.ID)
	if err != nil {
		t.Fatal(err)
	}
	settle(g1)
	pc.take()
	play(t, g1, a1.Name, b1.Name, "A1")
	if got := pc.take(); len(got) != 2 {
		t.Errorf("starting a game shouldn't stop observing: %v", got)
	}

	// And a player in a game can start observing another.
	tab := &client{}
	e.players.Attach(q, tab)
	if err := e.m.Observe(ctx, q.Name, tab, g1.ID()); err != nil {
		t.Errorf("observe while playing: %v", err)
	}
	// But never their own game.
	if err := e.m.Observe(ctx, q.Name, tab, g2.ID()); !errors.Is(err, ErrCantObserve) {
		t.Errorf("own game: %v", err)
	}
}

func TestObserveRacesGameEnd(t *testing.T) {
	// Observers come and go while moves are played and the game ends; the
	// race detector checks the bookkeeping, and nothing must be left over.
	e := newEnv(t, Config{})
	g, a, _, b, _ := e.start()
	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 50 {
			p, c := e.login(0)
			e.m.Observe(ctx, p.Name, c, g.ID())
			e.m.UnobserveAll(c)
		}
	}()
	for i, sq := range []string{"A1", "E5", "B1", "D5", "C1"} {
		name := a.Name
		if i%2 == 1 {
			name = b.Name
		}
		g.Place(ctx, name, sq, tak.Flat)
	}
	g.Resign(ctx, a.Name)
	<-done
	if n := e.observingCount(); n != 0 {
		t.Errorf("%d observations left", n)
	}
}
