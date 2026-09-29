// Package player tracks the players who are currently online, and guests.
// Registered players are loaded from the database when they log in and are
// dropped from memory when their last session ends.
package player

import (
	"sync"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

// Client is one logged-in connection a player can be reached through.
type Client interface {
	Send(ev protocol.Event)
	// Disconnect flushes queued events and closes the connection.
	Disconnect()
	// Done is closed once the connection has been fully cleaned up.
	Done() <-chan struct{}
}

type Player struct {
	ID    int // 0 for guests
	Name  string
	Guest bool

	mu         sync.Mutex
	admin      bool
	mod        bool
	bot        bool
	gagged     bool
	banned     bool
	clients    map[Client]struct{}
	lastActive time.Time
}

func newPlayer(rec players.Record) *Player {
	p := &Player{ID: rec.ID, Name: rec.Name, clients: make(map[Client]struct{})}
	p.update(rec)
	return p
}

// update refreshes permission flags from a freshly loaded record.
func (p *Player) update(rec players.Record) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.admin, p.mod, p.bot, p.gagged, p.banned = rec.IsAdmin, rec.IsMod, rec.IsBot, rec.IsGagged, rec.IsBanned
}

func (p *Player) IsAdmin() bool  { p.mu.Lock(); defer p.mu.Unlock(); return p.admin }
func (p *Player) IsMod() bool    { p.mu.Lock(); defer p.mu.Unlock(); return p.mod }
func (p *Player) IsBot() bool    { p.mu.Lock(); defer p.mu.Unlock(); return p.bot }
func (p *Player) IsGagged() bool { p.mu.Lock(); defer p.mu.Unlock(); return p.gagged }
func (p *Player) IsBanned() bool { p.mu.Lock(); defer p.mu.Unlock(); return p.banned }

// Clients returns the player's current connections.
func (p *Player) Clients() []Client {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Client, 0, len(p.clients))
	for c := range p.clients {
		out = append(out, c)
	}
	return out
}

// Online reports whether the player has at least one connection.
func (p *Player) Online() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.clients) > 0
}

// Send delivers ev to every connection the player is logged in on.
func (p *Player) Send(ev protocol.Event) {
	for _, c := range p.Clients() {
		c.Send(ev)
	}
}

// Touch marks the player as active now.
func (p *Player) Touch() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastActive = time.Now()
}

// idleSince reports whether the player is offline and has been inactive
// since before t.
func (p *Player) idleSince(t time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.clients) == 0 && p.lastActive.Before(t)
}
