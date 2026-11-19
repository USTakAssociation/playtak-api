// Package online tracks the players who are currently online, and guests.
// Registered players are loaded from the database when they log in and are
// dropped from memory when their last session ends.
package online

import (
	"slices"
	"sync"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

// Client is one logged-in connection a player can be reached through.
type Client interface {
	Send(event protocol.Event)
	ProtocolVersion() int
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
	moderator  bool
	bot        bool
	gagged     bool
	banned     bool
	clients    []Client // in the order they logged in
	lastActive time.Time
}

func newPlayer(record players.Record) *Player {
	p := &Player{ID: record.ID, Name: record.Name}
	p.update(record)
	return p
}

// update refreshes permission flags from a freshly loaded record.
func (p *Player) update(record players.Record) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.admin, p.moderator, p.bot, p.gagged, p.banned = record.IsAdmin, record.IsMod, record.IsBot, record.IsGagged, record.IsBanned
}

func (p *Player) IsAdmin() bool  { p.mu.Lock(); defer p.mu.Unlock(); return p.admin }
func (p *Player) IsMod() bool    { p.mu.Lock(); defer p.mu.Unlock(); return p.moderator }
func (p *Player) IsBot() bool    { p.mu.Lock(); defer p.mu.Unlock(); return p.bot }
func (p *Player) IsGagged() bool { p.mu.Lock(); defer p.mu.Unlock(); return p.gagged }
func (p *Player) IsBanned() bool { p.mu.Lock(); defer p.mu.Unlock(); return p.banned }

// SetGagged gags or ungags the player in memory only. It's for guests, who
// have no database record; registered players are refreshed with Refresh.
func (p *Player) SetGagged(on bool) { p.mu.Lock(); defer p.mu.Unlock(); p.gagged = on }

// Clients returns the player's current connections, oldest first.
func (p *Player) Clients() []Client {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.clients)
}

// Online reports whether the player has at least one connection.
func (p *Player) Online() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.clients) > 0
}

// Send delivers event to every connection the player is logged in on.
func (p *Player) Send(event protocol.Event) {
	for _, c := range p.Clients() {
		c.Send(event)
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
