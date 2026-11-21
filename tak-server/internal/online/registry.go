package online

import (
	"fmt"
	"slices"
	"strings"
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

// FindOnline returns the player named name, ignoring case, if they have at
// least one connection. Names are unique ignoring case.
func (r *Registry) FindOnline(name string) *Player {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p := r.byName[name]; p != nil && p.Online() {
		return p
	}
	for n, p := range r.byName {
		if strings.EqualFold(n, name) && p.Online() {
			return p
		}
	}
	return nil
}

// Acquire returns the in-memory player for record, creating it if the player is
// not online. Its flags are refreshed from record either way.
func (r *Registry) Acquire(record players.Record) *Player {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.byName[record.Name]; ok && !p.Guest {
		p.update(record)
		return p
	}
	return newPlayer(record)
}

// Refresh updates an online registered player's flags from record, after a
// change to their record. It does nothing if they aren't in memory.
func (r *Registry) Refresh(record players.Record) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.byName[record.Name]; ok && !p.Guest {
		p.update(record)
	}
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
	p.rejoinable = token != ""
	p.lastActive = time.Now()
	r.byName[p.Name] = p
	if token != "" {
		r.guestsByToken[token] = p
	}
	return p
}

// Attach records c as one of p's connections and makes p visible to Lookup.
func (r *Registry) Attach(player *Player, c Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	player.mu.Lock()
	if !slices.Contains(player.clients, c) {
		player.clients = append(player.clients, c)
	}
	player.lastActive = time.Now()
	player.mu.Unlock()
	r.byName[player.Name] = player
}

// Detach removes c from p's connections. A player with no connections left
// is dropped from memory, except a guest with a token, who is kept so they
// can rejoin with it until PruneGuests removes them. A guest without one can
// never rejoin, so keeping them would only let a connect-and-leave loop fill
// memory.
func (r *Registry) Detach(player *Player, c Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	player.mu.Lock()
	player.clients = slices.DeleteFunc(player.clients, func(x Client) bool { return x == c })
	remaining := len(player.clients)
	player.lastActive = time.Now()
	player.mu.Unlock()
	if remaining == 0 && !player.rejoinable && r.byName[player.Name] == player {
		delete(r.byName, player.Name)
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
