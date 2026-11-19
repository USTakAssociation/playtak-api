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

	// history holds the state after each ply, starting with the initial
	// state, so an undo can rewind exactly.
	history []clockState
}

type clockState struct {
	remaining [2]int64
	moves     [2]int
}

func newClock(s protocol.GameSettings) *clock {
	start := int64(s.Time) * 1000
	c := &clock{
		increment: int64(s.Increment) * 1000,
		scales:    s.IncrementScales,
		trigger:   s.ExtraTimeTrigger,
		extra:     int64(s.ExtraTimeAmount) * 1000,
		remaining: [2]int64{start, start},
	}
	c.history = []clockState{{c.remaining, c.moves}}
	return c
}

// moved charges mover for the ply they just finished and credits their
// increment and, on their trigger move, the extra time. The ply that starts
// the clock is free.
func (c *clock) moved(mover tak.Color, now time.Time) {
	running := c.started
	c.moves[mover]++
	c.started = true
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
	if running {
		c.remaining[mover] -= now.Sub(c.last).Milliseconds()
	}
	c.last = now
	c.history = append(c.history, clockState{c.remaining, c.moves})
}

// undo rewinds to the state before the last ply: thinking time is refunded
// and increments and bonuses are taken back. The player to move starts
// again from now. Once started, the clock keeps running even if the first
// ply is undone, as in the Java server.
func (c *clock) undo(now time.Time) {
	if len(c.history) < 2 {
		return
	}
	c.history = c.history[:len(c.history)-1]
	prev := c.history[len(c.history)-1]
	c.remaining, c.moves = prev.remaining, prev.moves
	c.last = now
}

// give adds ms to c's time. Gifts aren't recorded in the history, so an undo
// past the move they were given on takes them back, as in the Java server.
func (c *clock) give(to tak.Color, ms int64) {
	c.remaining[to] += ms
}

// sync charges the player to move for the time since the last update. It
// does nothing before the clock starts.
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
