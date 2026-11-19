package game

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/online"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/tak"
)

// blitz is a 60s game with no increment, so clocks are easy to follow.
var blitz = protocol.GameSettings{Size: 5, Time: 60, Pieces: 21, Capstones: 1, Color: protocol.White}

// startWith has b accept a's seek with settings s; a plays white.
func (e *env) startWith(s protocol.GameSettings) (g *Game, a *online.Player, ac *client, b *online.Player, bc *client) {
	e.t.Helper()
	a, ac = e.login(0)
	b, bc = e.login(0)
	sk := e.seek(a, ac, s, "")
	g, err := e.m.Accept(context.Background(), b, bc, sk.ID)
	if err != nil {
		e.t.Fatal(err)
	}
	ac.take()
	bc.take()
	return g, a, ac, b, bc
}

// skip moves the clock on without firing timers, as if a timer were late.
func (e *env) skip(d time.Duration) { e.mu.Lock(); e.now = e.now.Add(d); e.mu.Unlock() }

// timeoutAt reports when the only pending timer is due.
func (e *env) timeoutAt() time.Time {
	e.t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	var due []time.Time
	for _, t := range e.timers {
		if !t.done {
			due = append(due, t.at)
		}
	}
	if len(due) != 1 {
		e.t.Fatalf("%d timers pending, want 1", len(due))
	}
	return due[0]
}

// play makes moves alternately from white, failing the test on any error.
func play(t *testing.T, g *Game, white, black string, squares ...string) {
	t.Helper()
	for i, sq := range squares {
		name := white
		if i%2 == 1 {
			name = black
		}
		if err := g.Place(context.Background(), name, sq, tak.Flat); err != nil {
			t.Fatalf("%s %s: %v", name, sq, err)
		}
	}
}

func over(r string) protocol.Event { return protocol.GameOver{GameID: 1, Result: r} }

func lastResult(e *env) string {
	r := e.results.get()
	if len(r) == 0 {
		return ""
	}
	return r[len(r)-1].Result
}

// ---- Out of time ----

func TestWhiteRunsOutOfTime(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, ac, b, bc := e.startWith(blitz)
	play(t, g, a.Name, b.Name, "A1", "E5")
	ac.take()
	bc.take()

	e.advance(60*time.Second - time.Millisecond)
	settle(g)
	if lastResult(e) != "" {
		t.Fatal("ended on time with time left")
	}
	e.advance(time.Millisecond)
	settle(g)
	want := []protocol.Event{protocol.GameTime{GameID: 1, White: 0, Black: 60_000}, over("0-1")}
	if got := ac.take(); !slices.Equal(got, want) {
		t.Errorf("white got %v, want %v", got, want)
	}
	if got := bc.take(); !slices.Equal(got, want) {
		t.Errorf("black got %v, want %v", got, want)
	}
	if lastResult(e) != "0-1" || e.m.ForPlayer(a.Name) != nil {
		t.Errorf("result %q", lastResult(e))
	}
	if n := e.pending(); n != 0 {
		t.Errorf("%d timers left after the game", n)
	}
}

func TestBlackRunsOutOfTime(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, ac, b, _ := e.startWith(blitz)
	play(t, g, a.Name, b.Name, "A1", "E5", "B1")
	ac.take()
	e.advance(60 * time.Second)
	settle(g)
	if got := ac.take(); len(got) != 2 || got[1] != over("1-0") {
		t.Errorf("white got %v", got)
	}
	if lastResult(e) != "1-0" {
		t.Errorf("result %q", lastResult(e))
	}
}

func TestNoTimeoutBeforeTheClockStarts(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, _, _, _ := e.startWith(blitz)
	if n := e.pending(); n != 0 {
		t.Errorf("%d timers before the first ply", n)
	}
	e.advance(time.Hour)
	if err := g.Place(context.Background(), a.Name, "A1", tak.Flat); err != nil {
		t.Errorf("white's first ply is free however long it takes: %v", err)
	}
}

func TestMoveResetsTimeout(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, _, b, _ := e.startWith(blitz)
	play(t, g, a.Name, b.Name, "A1", "E5")
	e.advance(50 * time.Second)
	play(t, g, a.Name, b.Name, "B1") // white has 10s left; black's turn with 60s
	e.advance(59 * time.Second)      // past white's old deadline
	settle(g)
	if lastResult(e) != "" {
		t.Fatal("white's old timeout ended the game after they moved")
	}
	if n := e.pending(); n != 1 {
		t.Errorf("%d timers pending, want just black's", n)
	}
	e.advance(time.Second)
	settle(g)
	if lastResult(e) != "1-0" {
		t.Errorf("black should run out of time: %q", lastResult(e))
	}
}

func TestStaleTimeoutIsIgnored(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, _, b, _ := e.startWith(blitz)
	play(t, g, a.Name, b.Name, "A1", "E5")
	stale := e.timers[len(e.timers)-1]
	play(t, g, a.Name, b.Name, "B1")
	// The old timer fires anyway, as if Stop lost the race.
	stale.f()
	settle(g)
	if lastResult(e) != "" || e.pending() != 1 {
		t.Errorf("a stale timer must do nothing: result %q, %d timers", lastResult(e), e.pending())
	}
}

func TestEarlyTimeoutResets(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, _, b, _ := e.startWith(blitz)
	play(t, g, a.Name, b.Name, "A1", "E5")
	e.advance(20 * time.Second)
	current := e.timers[len(e.timers)-1]
	current.f() // fires 40s early
	settle(g)
	if lastResult(e) != "" {
		t.Fatal("ended on time with time left")
	}
	next := e.timers[len(e.timers)-1]
	if next == current || next.done || !next.at.Equal(t0.Add(60*time.Second)) {
		t.Errorf("should re-arm for the time left: %+v", next)
	}
}

func TestMoveAfterTimeRanOutIsRefused(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, ac, b, _ := e.startWith(blitz)
	play(t, g, a.Name, b.Name, "A1", "E5")
	ac.take()
	e.skip(61 * time.Second) // the timeout hasn't been handled yet
	if err := g.Place(context.Background(), a.Name, "B1", tak.Flat); !errors.Is(err, ErrNotPlaying) {
		t.Errorf("move after time ran out: %v", err)
	}
	if got := ac.take(); len(got) != 2 || got[1] != over("0-1") {
		t.Errorf("white got %v", got)
	}
	if notation := e.results.get()[0].Notation; notation != "P A1,P E5" {
		t.Errorf("the late move must not be recorded: %q", notation)
	}
}

func TestOutOfTimeWhileDisconnected(t *testing.T) {
	e := newEnv(t, Config{ReconnectTimeout: 2 * time.Minute})
	g, a, ac, b, _ := e.startWith(blitz)
	play(t, g, a.Name, b.Name, "A1", "E5", "B1")
	ac.take()
	e.disconnect(g, b)
	ac.take()
	e.advance(60 * time.Second)
	settle(g)
	// The clock runs out before the reconnect window: a loss on time, not
	// an abandonment.
	if got := ac.take(); len(got) != 2 || got[1] != over("1-0") {
		t.Errorf("white got %v", got)
	}
	e.advance(2 * time.Minute)
	if len(e.results.get()) != 1 {
		t.Error("the abandon timer must not end the game again")
	}
}

func TestAbandonBeforeOutOfTime(t *testing.T) {
	e := newEnv(t, Config{ReconnectTimeout: 30 * time.Second})
	g, a, ac, b, _ := e.startWith(blitz)
	play(t, g, a.Name, b.Name, "A1", "E5", "B1")
	e.disconnect(g, b)
	ac.take()
	e.advance(30 * time.Second)
	settle(g)
	if got := ac.take(); len(got) != 1 || got[0] != (protocol.GameAbandoned{GameID: 1, Quitter: b.Name}) {
		t.Errorf("white got %v", got)
	}
	if e.pending() != 0 {
		t.Error("the timeout should be stopped")
	}
}

// ---- Bot first move ----

func (e *env) startAgainstBot(s protocol.GameSettings, botColor protocol.Color) (g *Game, human *online.Player, hc *client) {
	e.t.Helper()
	human, hc = e.login(0)
	bot := e.players.Acquire(players.Record{ID: 50, Name: "TakBot", IsBot: true})
	bc := &client{}
	e.players.Attach(bot, bc)
	// The bot posts the seek in the color it wants.
	s.Color = botColor
	sk := e.seek(bot, bc, s, "")
	g, err := e.m.Accept(context.Background(), human, hc, sk.ID)
	if err != nil {
		e.t.Fatal(err)
	}
	hc.take()
	return g, human, hc
}

func TestBotWinsIfWhiteNeverMoves(t *testing.T) {
	e := newEnv(t, Config{})
	g, _, hc := e.startAgainstBot(std5, protocol.Black) // 600s game: the 5 minute limit
	e.advance(5*time.Minute - time.Millisecond)
	settle(g)
	if lastResult(e) != "" {
		t.Fatal("ended before the limit")
	}
	e.advance(time.Millisecond)
	settle(g)
	if got := hc.take(); !slices.Equal(got, []protocol.Event{over("0-1")}) {
		t.Errorf("human got %v", got)
	}
	if lastResult(e) != "0-1" {
		t.Errorf("result %q", lastResult(e))
	}
}

func TestBotLimitIsGameTimeWhenShorter(t *testing.T) {
	e := newEnv(t, Config{})
	g, _, _ := e.startAgainstBot(blitz, protocol.Black)
	e.advance(60 * time.Second)
	settle(g)
	if lastResult(e) != "0-1" {
		t.Errorf("a 60s game's limit is 60s: %q", lastResult(e))
	}
}

func TestBotTimerStopsOnFirstMove(t *testing.T) {
	e := newEnv(t, Config{})
	g, human, _ := e.startAgainstBot(std5, protocol.Black)
	e.advance(4 * time.Minute)
	if err := g.Place(context.Background(), human.Name, "A1", tak.Flat); err != nil {
		t.Fatal(err)
	}
	// Undoing back to the start doesn't revive the rule.
	if err := g.RequestUndo(context.Background(), human.Name); err != nil {
		t.Fatal(err)
	}
	if err := g.RequestUndo(context.Background(), "TakBot"); err != nil {
		t.Fatal(err)
	}
	e.advance(2 * time.Minute)
	settle(g)
	if lastResult(e) != "" {
		t.Errorf("game ended: %q", lastResult(e))
	}
}

func TestStaleBotTimerAfterUndoToStart(t *testing.T) {
	e := newEnv(t, Config{})
	g, human, _ := e.startAgainstBot(std5, protocol.Black)
	botTimer := e.timers[0] // the only timer before the first move
	ctx := context.Background()
	if err := g.Place(ctx, human.Name, "A1", tak.Flat); err != nil {
		t.Fatal(err)
	}
	g.RequestUndo(ctx, human.Name)
	g.RequestUndo(ctx, "TakBot") // back to ply 0
	// The callback fires anyway, as if it had started before Stop.
	botTimer.f()
	settle(g)
	if lastResult(e) != "" {
		t.Errorf("a stale bot timer ended the game: %q", lastResult(e))
	}
}

func TestNoBotTimerWhenBotIsWhite(t *testing.T) {
	e := newEnv(t, Config{})
	e.startAgainstBot(std5, protocol.White)
	if n := e.pending(); n != 0 {
		t.Errorf("%d timers armed; the rule is only for a bot playing black", n)
	}
}

func TestNoBotTimerBetweenHumans(t *testing.T) {
	e := newEnv(t, Config{})
	e.start()
	if n := e.pending(); n != 0 {
		t.Errorf("%d timers armed", n)
	}
}

// ---- Undo ----

func TestUndoNeedsAPly(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, ac, _, bc := e.start()
	if err := g.RequestUndo(context.Background(), a.Name); !errors.Is(err, ErrNothingToUndo) {
		t.Errorf("err %v", err)
	}
	if len(ac.take())+len(bc.take()) != 0 {
		t.Error("nothing should be sent")
	}
}

func TestUndoAgreed(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, ac, b, bc := e.startWith(blitz)
	ctx := context.Background()
	play(t, g, a.Name, b.Name, "A1", "E5")
	e.advance(10 * time.Second)
	play(t, g, a.Name, b.Name, "B1") // white 50s
	ac.take()
	bc.take()

	// Black asks; asking again changes nothing.
	for range 2 {
		if err := g.RequestUndo(ctx, b.Name); err != nil {
			t.Fatal(err)
		}
	}
	if got := ac.take(); !slices.Equal(got, []protocol.Event{protocol.UndoRequested{GameID: 1}}) {
		t.Errorf("white got %v", got)
	}
	if got := bc.take(); len(got) != 0 {
		t.Errorf("black got %v", got)
	}

	// White agrees 5s later: B1 is gone and white's 10s are refunded.
	e.advance(5 * time.Second)
	if err := g.RequestUndo(ctx, a.Name); err != nil {
		t.Fatal(err)
	}
	want := []protocol.Event{protocol.GameTime{GameID: 1, White: 60_000, Black: 60_000}, protocol.Undone{GameID: 1}}
	if got := ac.take(); !slices.Equal(got, want) {
		t.Errorf("white got %v, want %v", got, want)
	}
	if got := bc.take(); !slices.Equal(got, want) {
		t.Errorf("black got %v, want %v", got, want)
	}
	if tps, _ := g.TPS(ctx, a.Name); tps != "x4,1/x5/x5/x5/2,x4 1 2" {
		t.Errorf("TPS %q", tps)
	}
	// White is to move again, with a fresh timeout from the undo.
	if at := e.timeoutAt(); !at.Equal(t0.Add(75 * time.Second)) {
		t.Errorf("timeout due at +%v, want +75s", at.Sub(t0))
	}
	e.advance(60 * time.Second)
	settle(g)
	if lastResult(e) != "0-1" {
		t.Errorf("result %q", lastResult(e))
	}
	if n := e.results.get()[0].Notation; n != "P A1,P E5" {
		t.Errorf("notation %q", n)
	}
}

func TestUndoRequestWithdrawn(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, ac, b, bc := e.start()
	ctx := context.Background()
	play(t, g, a.Name, b.Name, "A1")
	ac.take()
	bc.take()

	// Withdrawing a request you didn't make does nothing.
	g.RemoveUndo(ctx, a.Name)
	if got := bc.take(); len(got) != 0 {
		t.Errorf("black got %v", got)
	}
	g.RequestUndo(ctx, a.Name)
	bc.take()
	if err := g.RemoveUndo(ctx, a.Name); err != nil {
		t.Fatal(err)
	}
	if got := bc.take(); !slices.Equal(got, []protocol.Event{protocol.UndoRemoved{GameID: 1}}) {
		t.Errorf("black got %v", got)
	}
	// Black's request is now a new request, not an agreement.
	g.RequestUndo(ctx, b.Name)
	if got := ac.take(); !slices.Equal(got, []protocol.Event{protocol.UndoRequested{GameID: 1}}) {
		t.Errorf("white got %v", got)
	}
}

func TestMoveClearsUndoRequest(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, ac, b, bc := e.start()
	ctx := context.Background()
	play(t, g, a.Name, b.Name, "A1")
	g.RequestUndo(ctx, a.Name)
	play(t, g, b.Name, a.Name, "E5") // black moves; white's request lapses
	ac.take()
	bc.take()
	g.RequestUndo(ctx, b.Name)
	if got := ac.take(); !slices.Equal(got, []protocol.Event{protocol.UndoRequested{GameID: 1}}) {
		t.Errorf("should be a fresh request: %v", got)
	}
	if tps, _ := g.TPS(ctx, a.Name); tps != "x4,1/x5/x5/x5/2,x4 1 2" {
		t.Errorf("nothing should be undone: %q", tps)
	}
}

func TestUndoByStranger(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, _, b, _ := e.start()
	play(t, g, a.Name, b.Name, "A1")
	if err := g.RequestUndo(context.Background(), "stranger"); !errors.Is(err, ErrNotPlaying) {
		t.Errorf("err %v", err)
	}
}

// ---- Draw ----

func TestDrawAgreed(t *testing.T) {
	e := newEnv(t, Config{})
	watcher := &client{}
	e.m.Subscribe(watcher)
	g, a, ac, b, bc := e.start()
	watcher.take()
	ctx := context.Background()
	play(t, g, a.Name, b.Name, "A1")
	ac.take()
	bc.take()

	for range 2 { // offering twice changes nothing
		if err := g.OfferDraw(ctx, a.Name); err != nil {
			t.Fatal(err)
		}
	}
	if got := bc.take(); !slices.Equal(got, []protocol.Event{protocol.DrawOffered{GameID: 1}}) {
		t.Errorf("black got %v", got)
	}
	if got := ac.take(); len(got) != 0 {
		t.Errorf("white got %v", got)
	}
	// The offer survives moves.
	play(t, g, b.Name, a.Name, "E5", "B1")
	ac.take()
	bc.take()
	if err := g.OfferDraw(ctx, b.Name); err != nil {
		t.Fatal(err)
	}
	for _, c := range []*client{ac, bc} {
		if got := c.take(); !slices.Equal(got, []protocol.Event{over("1/2-1/2")}) {
			t.Errorf("got %v", got)
		}
	}
	if got := watcher.take(); len(got) != 1 {
		t.Errorf("game list should drop the game: %v", got)
	}
	if r := e.results.get(); len(r) != 1 || r[0].Result != "1/2-1/2" || r[0].Notation != "P A1,P E5,P B1" {
		t.Errorf("saved %+v", r)
	}
	if e.pending() != 0 {
		t.Error("timers left after the draw")
	}
}

func TestDrawOfferWithdrawn(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, ac, b, bc := e.start()
	ctx := context.Background()
	g.RemoveDraw(ctx, a.Name) // nothing to withdraw
	g.OfferDraw(ctx, a.Name)
	g.RemoveDraw(ctx, b.Name) // not black's offer
	if got := bc.take(); !slices.Equal(got, []protocol.Event{protocol.DrawOffered{GameID: 1}}) {
		t.Errorf("black got %v", got)
	}
	if err := g.RemoveDraw(ctx, a.Name); err != nil {
		t.Fatal(err)
	}
	if got := bc.take(); !slices.Equal(got, []protocol.Event{protocol.DrawRemoved{GameID: 1}}) {
		t.Errorf("black got %v", got)
	}
	// Black's offer is now new, not an acceptance.
	g.OfferDraw(ctx, b.Name)
	if got := ac.take(); !slices.Equal(got, []protocol.Event{protocol.DrawOffered{GameID: 1}}) {
		t.Errorf("white got %v", got)
	}
	if len(e.results.get()) != 0 {
		t.Error("game should continue")
	}
}

// ---- Give time ----

func TestGiveTimeNeedsTheClock(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, ac, _, bc := e.start()
	if err := g.GiveTime(context.Background(), a.Name); !errors.Is(err, ErrClockNotStarted) {
		t.Errorf("err %v", err)
	}
	if len(ac.take())+len(bc.take()) != 0 {
		t.Error("nothing should be sent")
	}
}

func TestGiveTime(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, ac, b, bc := e.startWith(blitz)
	ctx := context.Background()
	play(t, g, a.Name, b.Name, "A1", "E5")
	e.advance(10 * time.Second) // white has 50s
	ac.take()
	bc.take()

	// Black gives white time on white's turn: the timeout moves back 15s.
	if err := g.GiveTime(ctx, b.Name); err != nil {
		t.Fatal(err)
	}
	want := []protocol.Event{
		protocol.TimeGiven{GameID: 1, To: protocol.White, Millis: 15_000},
		protocol.GameTime{GameID: 1, White: 65_000, Black: 60_000},
	}
	for _, c := range []*client{ac, bc} {
		if got := c.take(); !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	}
	if at := e.timeoutAt(); !at.Equal(t0.Add(75 * time.Second)) {
		t.Errorf("timeout due at +%v, want +75s", at.Sub(t0))
	}
	e.advance(50 * time.Second)
	settle(g)
	if lastResult(e) != "" {
		t.Fatal("ended at the old deadline")
	}
	e.advance(15 * time.Second)
	settle(g)
	if lastResult(e) != "0-1" {
		t.Errorf("result %q", lastResult(e))
	}
}

func TestGiveTimeCooldown(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, _, b, _ := e.start()
	ctx := context.Background()
	play(t, g, a.Name, b.Name, "A1", "E5")
	if err := g.GiveTime(ctx, a.Name); err != nil {
		t.Fatal(err)
	}
	// The cooldown is per player.
	if err := g.GiveTime(ctx, b.Name); err != nil {
		t.Errorf("black's first gift: %v", err)
	}
	e.advance(14*time.Second + 1*time.Millisecond)
	var cd *CooldownError
	if err := g.GiveTime(ctx, a.Name); !errors.As(err, &cd) || cd.Seconds() != 1 || err.Error() != "You can give time again in 1 seconds" {
		t.Errorf("err %v", err)
	}
	e.advance(999 * time.Millisecond)
	if err := g.GiveTime(ctx, a.Name); err != nil {
		t.Errorf("after 15s: %v", err)
	}
	// And it starts again after every gift.
	if err := g.GiveTime(ctx, a.Name); !errors.As(err, &cd) || cd.Seconds() != 15 {
		t.Errorf("err %v", err)
	}
}

func TestCooldownSecondsRoundUp(t *testing.T) {
	for left, want := range map[time.Duration]int64{
		time.Millisecond:              1,
		time.Second:                   1,
		time.Second + time.Nanosecond: 2,
		15 * time.Second:              15,
	} {
		if got := (&CooldownError{Left: left}).Seconds(); got != want {
			t.Errorf("%v: %d, want %d", left, got, want)
		}
	}
}

func TestGiveTimeToDisconnectedOpponent(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, ac, b, bc := e.start()
	ctx := context.Background()
	play(t, g, a.Name, b.Name, "A1", "E5")
	e.disconnect(g, b)
	ac.take()
	bc.take()
	if err := g.GiveTime(ctx, a.Name); err != nil {
		t.Fatal(err)
	}
	if got := ac.take(); len(got) != 2 {
		t.Errorf("giver got %v", got)
	}
	if got := bc.take(); len(got) != 0 {
		t.Errorf("disconnected player got %v", got)
	}
}

// ---- Reconnect ----

func TestReconnectResendsPendingRequests(t *testing.T) {
	e := newEnv(t, Config{})
	g, a, _, b, _ := e.start()
	ctx := context.Background()
	play(t, g, a.Name, b.Name, "A1")
	g.OfferDraw(ctx, a.Name)
	g.RequestUndo(ctx, a.Name)
	e.disconnect(g, b)

	fresh := &client{}
	if err := g.Reconnected(ctx, b, fresh); err != nil {
		t.Fatal(err)
	}
	got := fresh.take()
	tail := got[len(got)-3:]
	want := []protocol.Event{protocol.ServerMessage{Text: "Your game is resumed"}, protocol.DrawOffered{GameID: 1}, protocol.UndoRequested{GameID: 1}}
	if !slices.Equal(tail, want) {
		t.Errorf("got %v, want it to end %v", got, want)
	}

	// The player's own requests aren't sent back to them.
	e.disconnect(g, a)
	fresh = &client{}
	g.Reconnected(ctx, a, fresh)
	got = fresh.take()
	if last := got[len(got)-1]; last != (protocol.ServerMessage{Text: "Your game is resumed"}) {
		t.Errorf("got %v", got)
	}
}
