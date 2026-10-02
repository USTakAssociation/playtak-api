// Package game runs live games: players, clocks, the game list and saving
// results. The rules themselves are in package tak.
package game

import (
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/tak"
)

// clock tracks both players' remaining time in milliseconds. It starts on
// the first ply: white's first ply costs nothing.
type clock struct {
	increment int64 // per move, ms
	scales    bool  // increment multiplied by the mover's move number
	trigger   int   // move number at which extra is added, 0 for none
	extra     int64 // ms

	remaining [2]int64
	moves     [2]int // moves made by each player, counting the trigger bonus
	started   bool
	last      time.Time // when remaining was last brought up to date
}

func newClock(s protocol.GameSettings) *clock {
	start := int64(s.Time) * 1000
	return &clock{
		increment: int64(s.Increment) * 1000,
		scales:    s.IncrementScales,
		trigger:   s.ExtraTimeTrigger,
		extra:     int64(s.ExtraTimeAmount) * 1000,
		remaining: [2]int64{start, start},
	}
}

// moved charges mover for the ply they just finished and credits their
// increment and, on their trigger move, the extra time. firstPly is true for
// the game's first ply.
func (c *clock) moved(mover tak.Color, firstPly bool, now time.Time) {
	c.moves[mover]++
	if !c.started {
		c.started = true
		c.last = now
	}
	inc := c.increment
	if c.scales {
		inc *= int64(c.moves[mover])
	}
	c.remaining[mover] += inc
	if c.moves[mover] == c.trigger {
		c.remaining[mover] += c.extra
		// Counted as a move so the bonus can't trigger twice; this also
		// shifts the scaled increment by one, as the Java server does.
		c.moves[mover]++
	}
	if !firstPly {
		c.remaining[mover] -= now.Sub(c.last).Milliseconds()
	}
	c.last = now
}

// sync charges the player to move for the time since the last update. It
// does nothing before the first ply.
func (c *clock) sync(toMove tak.Color, now time.Time) {
	if !c.started {
		return
	}
	c.remaining[toMove] -= now.Sub(c.last).Milliseconds()
	c.last = now
}

func (c *clock) event(gameID int) protocol.GameTime {
	return protocol.GameTime{GameID: gameID, White: c.remaining[tak.White], Black: c.remaining[tak.Black]}
}
