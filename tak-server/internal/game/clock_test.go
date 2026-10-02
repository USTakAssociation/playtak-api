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
	c.moved(tak.White, true, t0.Add(10*time.Second))
	if c.remaining != [2]int64{600_000, 600_000} {
		t.Fatalf("white's first ply should be free: %v", c.remaining)
	}
	// Black thinks for 7s.
	c.moved(tak.Black, false, t0.Add(17*time.Second))
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
	c.moved(tak.White, true, t0)
	c.moved(tak.Black, false, t0.Add(2*time.Second))
	if c.remaining != [2]int64{65_000, 63_000} {
		t.Errorf("%v", c.remaining)
	}
}

func TestClockScaledIncrement(t *testing.T) {
	c := newClock(protocol.GameSettings{Time: 60, Increment: 2, IncrementScales: true})
	now := t0
	for range 3 {
		c.moved(tak.White, now.Equal(t0), now)
		c.moved(tak.Black, false, now)
	}
	// White's moves earn 2+4+6, black's the same; no time was used.
	if c.remaining != [2]int64{72_000, 72_000} {
		t.Errorf("%v", c.remaining)
	}
}

func TestClockExtraTimeOnTriggerMove(t *testing.T) {
	c := newClock(protocol.GameSettings{Time: 60, ExtraTimeTrigger: 2, ExtraTimeAmount: 30})
	c.moved(tak.White, true, t0)
	c.moved(tak.Black, false, t0)
	if c.remaining != [2]int64{60_000, 60_000} {
		t.Fatalf("before the trigger move: %v", c.remaining)
	}
	c.moved(tak.White, false, t0)
	if c.remaining[tak.White] != 90_000 {
		t.Errorf("white's 2nd move should add 30s: %v", c.remaining)
	}
	c.moved(tak.Black, false, t0)
	c.moved(tak.White, false, t0)
	c.moved(tak.White, false, t0)
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
