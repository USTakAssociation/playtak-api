package player

import (
	"fmt"
	"sync"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
)

// Registry holds online registered players and all guests created since
// startup, keyed by their case-sensitive name.
type Registry struct {
	mu            sync.Mutex
	byName        map[string]*Player
	guestsByToken map[string]*Player
	guestSeq      int
}

func NewRegistry() *Registry {
	return &Registry{
		byName:        make(map[string]*Player),
		guestsByToken: make(map[string]*Player),
	}
}

// Lookup returns the online player or guest with exactly this name, or nil.
func (r *Registry) Lookup(name string) *Player {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.byName[name]
}

// Acquire returns the in-memory player for rec, creating it if the player is
// not online. Its flags are refreshed from rec either way.
func (r *Registry) Acquire(rec players.Record) *Player {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.byName[rec.Name]; ok && !p.Guest {
		p.update(rec)
		return p
	}
	return newPlayer(rec)
}

// Guest returns the guest for token, creating one if needed. An empty token
// always creates a new guest, which cannot be rejoined later.
func (r *Registry) Guest(token string) *Player {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.guestsByToken[token]; ok && token != "" {
		return p
	}
	r.guestSeq++
	p := newPlayer(players.Record{Name: fmt.Sprintf("Guest%d", r.guestSeq)})
	p.Guest = true
	p.lastActive = time.Now()
	r.byName[p.Name] = p
	if token != "" {
		r.guestsByToken[token] = p
	}
	return p
}

// Attach records c as one of p's connections and makes p visible to Lookup.
func (r *Registry) Attach(p *Player, c Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p.mu.Lock()
	p.clients[c] = struct{}{}
	p.lastActive = time.Now()
	p.mu.Unlock()
	r.byName[p.Name] = p
}

// Detach removes c from p's connections. A registered player with no
// connections left is dropped from memory; guests are kept so they can rejoin
// with their token until PruneGuests removes them.
func (r *Registry) Detach(p *Player, c Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p.mu.Lock()
	delete(p.clients, c)
	remaining := len(p.clients)
	p.lastActive = time.Now()
	p.mu.Unlock()
	if remaining == 0 && !p.Guest && r.byName[p.Name] == p {
		delete(r.byName, p.Name)
	}
}

// Online returns every player with at least one connection.
func (r *Registry) Online() []*Player {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*Player
	for _, p := range r.byName {
		if p.Online() {
			out = append(out, p)
		}
	}
	return out
}

// PruneGuests forgets offline guests inactive for longer than idle and
// returns how many were removed.
func (r *Registry) PruneGuests(idle time.Duration) int {
	cutoff := time.Now().Add(-idle)
	r.mu.Lock()
	defer r.mu.Unlock()
	removed := 0
	for name, p := range r.byName {
		if p.Guest && p.idleSince(cutoff) {
			delete(r.byName, name)
			removed++
		}
	}
	for token, p := range r.guestsByToken {
		if r.byName[p.Name] != p {
			delete(r.guestsByToken, token)
		}
	}
	return removed
}
