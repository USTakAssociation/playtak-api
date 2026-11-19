package game

import (
	"testing"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/tak"
)

var t0 = time.Unix(1_800_000_000, 0)

func TestClockStartsOnFirstPly(t *testing.T) {
	c := newClock(protocol.GameSettings{Time: 600})
	c.sync(tak.White, t0.Add(time.Hour)) // nothing played: no time passes
	c.moved(tak.White, t0.Add(10*time.Second))
	if c.remaining != [2]int64{600_000, 600_000} {
		t.Fatalf("white's first ply should be free: %v", c.remaining)
	}
	// Black thinks for 7s.
	c.moved(tak.Black, t0.Add(17*time.Second))
	if c.remaining != [2]int64{600_000, 593_000} {
		t.Errorf("after black's move: %v", c.remaining)
	}
	// White is to move; 3s pass.
	c.sync(tak.White, t0.Add(20*time.Second))
	if c.remaining != [2]int64{597_000, 593_000} {
		t.Errorf("after sync: %v", c.remaining)
	}
}

func TestClockIncrement(t *testing.T) {
	c := newClock(protocol.GameSettings{Time: 60, Increment: 5})
	c.moved(tak.White, t0)
	c.moved(tak.Black, t0.Add(2*time.Second))
	if c.remaining != [2]int64{65_000, 63_000} {
		t.Errorf("%v", c.remaining)
	}
}

func TestClockScaledIncrement(t *testing.T) {
	c := newClock(protocol.GameSettings{Time: 60, Increment: 2, IncrementScales: true})
	now := t0
	for range 3 {
		c.moved(tak.White, now)
		c.moved(tak.Black, now)
	}
	// White's moves earn 2+4+6, black's the same; no time was used.
	if c.remaining != [2]int64{72_000, 72_000} {
		t.Errorf("%v", c.remaining)
	}
}

func TestClockExtraTimeOnTriggerMove(t *testing.T) {
	c := newClock(protocol.GameSettings{Time: 60, ExtraTimeTrigger: 2, ExtraTimeAmount: 30})
	c.moved(tak.White, t0)
	c.moved(tak.Black, t0)
	if c.remaining != [2]int64{60_000, 60_000} {
		t.Fatalf("before the trigger move: %v", c.remaining)
	}
	c.moved(tak.White, t0)
	if c.remaining[tak.White] != 90_000 {
		t.Errorf("white's 2nd move should add 30s: %v", c.remaining)
	}
	c.moved(tak.Black, t0)
	c.moved(tak.White, t0)
	c.moved(tak.White, t0)
	if c.remaining[tak.White] != 90_000 {
		t.Errorf("the bonus is added once: %v", c.remaining)
	}
}

func TestClockEvent(t *testing.T) {
	c := newClock(protocol.GameSettings{Time: 10})
	if ev := c.event(7); ev != (protocol.GameTime{GameID: 7, White: 10_000, Black: 10_000}) {
		t.Errorf("%+v", ev)
	}
}

func TestClockUndoRewindsToBeforeThePly(t *testing.T) {
	c := newClock(protocol.GameSettings{Time: 60, Increment: 5, ExtraTimeTrigger: 2, ExtraTimeAmount: 30})
	c.moved(tak.White, t0)                     // free: 65s
	c.moved(tak.Black, t0.Add(4*time.Second))  // 60+5-4 = 61s
	c.moved(tak.White, t0.Add(10*time.Second)) // 65+5+30-6 = 94s, trigger counted
	if c.remaining != [2]int64{94_000, 61_000} || c.moves[tak.White] != 3 {
		t.Fatalf("before undo: %v %v", c.remaining, c.moves)
	}

	// The undo comes 20s later: white's thinking, increment and bonus are
	// all taken back, and black's clock restarts from the undo.
	c.undo(t0.Add(30 * time.Second))
	if c.remaining != [2]int64{65_000, 61_000} || c.moves != [2]int{1, 1} {
		t.Errorf("after undo: %v %v", c.remaining, c.moves)
	}
	c.sync(tak.White, t0.Add(33*time.Second))
	if c.remaining[tak.White] != 62_000 {
		t.Errorf("white's clock should run from the undo: %v", c.remaining)
	}
	// The bonus can be earned again on the replayed trigger move.
	c.moved(tak.White, t0.Add(33*time.Second))
	if c.remaining[tak.White] != 97_000 {
		t.Errorf("replayed trigger move: %v", c.remaining)
	}
}

func TestClockUndoFirstPlyKeepsRunning(t *testing.T) {
	c := newClock(protocol.GameSettings{Time: 60})
	c.moved(tak.White, t0)
	c.undo(t0.Add(time.Second))
	if c.remaining != [2]int64{60_000, 60_000} || !c.started {
		t.Fatalf("after undo: %v started=%v", c.remaining, c.started)
	}
	// As in the Java server, the clock doesn't stop again: white's replayed
	// first ply is charged.
	c.moved(tak.White, t0.Add(4*time.Second))
	if c.remaining[tak.White] != 57_000 {
		t.Errorf("replayed first ply: %v", c.remaining)
	}
}

func TestClockUndoWithNothingPlayed(t *testing.T) {
	c := newClock(protocol.GameSettings{Time: 60})
	c.undo(t0)
	if c.remaining != [2]int64{60_000, 60_000} || len(c.history) != 1 {
		t.Errorf("%v %v", c.remaining, c.history)
	}
}

func TestClockGiftIsLostOnUndo(t *testing.T) {
	c := newClock(protocol.GameSettings{Time: 60})
	c.moved(tak.White, t0)
	c.moved(tak.Black, t0)
	c.give(tak.Black, 15_000)
	if c.remaining[tak.Black] != 75_000 {
		t.Fatalf("after gift: %v", c.remaining)
	}
	c.undo(t0)
	if c.remaining[tak.Black] != 60_000 {
		t.Errorf("an undo past the gift takes it back, as in Java: %v", c.remaining)
	}
}
