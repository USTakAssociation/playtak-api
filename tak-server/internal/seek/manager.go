// Package seek manages open seeks: offers to play that another player can
// accept to start a game.
package seek

import (
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

// ErrIncompatible means the posting session's protocol version can't
// display a game with the requested settings.
var ErrIncompatible = errors.New("seek settings not supported by client protocol version")

const defaultSize = 5

// Listener is a session that owns seeks and receives seek updates.
type Listener interface {
	Send(ev protocol.Event)
	ProtocolVersion() int
}

// Request describes a seek to post.
type Request struct {
	Creator  string
	Bot      bool
	Opponent string
	Settings protocol.GameSettings
	// PNTID links the seek to a tournament game, 0 if none.
	PNTID int
}

// Seek is an open seek and the session that posted it.
type Seek struct {
	protocol.Seek
	Owner Listener
	PNTID int
	// RematchOf is the finished game this seek offers a rematch of, 0 if none.
	RematchOf int
}

// Manager holds every open seek, at most one per owning session. Updates are
// broadcast while holding the lock so listeners always see a seek's "new"
// before its "remove".
type Manager struct {
	mu        sync.Mutex
	lastID    int
	seeks     map[int]*Seek
	byOwner   map[Listener]*Seek
	listeners map[Listener]struct{}
}

func NewManager() *Manager {
	return &Manager{
		seeks:     make(map[int]*Seek),
		byOwner:   make(map[Listener]*Seek),
		listeners: make(map[Listener]struct{}),
	}
}

// Subscribe starts sending seek updates to l, beginning with every current
// seek it can display.
func (m *Manager) Subscribe(l Listener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners[l] = struct{}{}
	m.sendListLocked(l)
}

// Unsubscribe stops sending seek updates to l.
func (m *Manager) Unsubscribe(l Listener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.listeners, l)
}

// SendList sends l every current seek it can display.
func (m *Manager) SendList(l Listener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendListLocked(l)
}

// Post replaces owner's seek with a new one.
func (m *Manager) Post(owner Listener, r Request) (Seek, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Checked before touching the existing seek, so a rejected post has no
	// side effects.
	if owner.ProtocolVersion() < r.Settings.RequiredProtocolVersion() {
		return Seek{}, ErrIncompatible
	}
	return m.addLocked(owner, r, 0), nil
}

// Cancel removes owner's seek, if it has one.
func (m *Manager) Cancel(owner Listener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeOwnerLocked(owner)
}

// RematchResult is the outcome of Rematch: either the opponent had already
// offered a rematch (AcceptID is their seek), or a new rematch seek was
// posted (Created).
type RematchResult struct {
	AcceptID int
	Created  Seek
}

// Rematch offers a rematch of gameID, replacing owner's seek. If a rematch
// seek for that game is already addressed to the requester, it is left for
// them to accept instead of posting a second one.
func (m *Manager) Rematch(owner Listener, gameID int, r Request) (RematchResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if owner.ProtocolVersion() < r.Settings.RequiredProtocolVersion() {
		return RematchResult{}, ErrIncompatible
	}
	m.removeOwnerLocked(owner)
	for _, s := range m.seeks {
		if s.RematchOf == gameID && strings.EqualFold(s.Opponent, r.Creator) {
			return RematchResult{AcceptID: s.ID}, nil
		}
	}
	return RematchResult{Created: m.addLocked(owner, r, gameID)}, nil
}

// Get returns the seek with the given ID.
func (m *Manager) Get(id int) (Seek, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.seeks[id]
	if !ok {
		return Seek{}, false
	}
	return *s, true
}

// List returns every open seek, ordered by ID.
func (m *Manager) List() []Seek {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sortedLocked()
}

func (m *Manager) addLocked(owner Listener, r Request, rematchOf int) Seek {
	m.removeOwnerLocked(owner)
	m.lastID++
	s := &Seek{
		Seek: protocol.Seek{
			ID:           m.lastID,
			Creator:      r.Creator,
			Bot:          r.Bot,
			Opponent:     r.Opponent,
			GameSettings: normalize(r.Settings),
		},
		Owner:     owner,
		PNTID:     r.PNTID,
		RematchOf: rematchOf,
	}
	m.seeks[s.ID] = s
	m.byOwner[owner] = s
	m.broadcastLocked(protocol.SeekAdded{Seek: s.Seek}, s.Seek)
	return *s
}

func (m *Manager) removeOwnerLocked(owner Listener) {
	s, ok := m.byOwner[owner]
	if !ok {
		return
	}
	delete(m.byOwner, owner)
	delete(m.seeks, s.ID)
	m.broadcastLocked(protocol.SeekRemoved{Seek: s.Seek}, s.Seek)
}

// broadcastLocked sends ev to every listener that can display s. Protocol
// versions and seek settings never change, so a listener that was skipped
// for a seek's "new" is skipped for its "remove" too.
func (m *Manager) broadcastLocked(ev protocol.Event, s protocol.Seek) {
	required := s.RequiredProtocolVersion()
	for l := range m.listeners {
		if l.ProtocolVersion() >= required {
			l.Send(ev)
		}
	}
}

func (m *Manager) sendListLocked(l Listener) {
	for _, s := range m.sortedLocked() {
		if l.ProtocolVersion() >= s.RequiredProtocolVersion() {
			l.Send(protocol.SeekAdded{Seek: s.Seek})
		}
	}
}

func (m *Manager) sortedLocked() []Seek {
	out := make([]Seek, 0, len(m.seeks))
	for _, s := range m.seeks {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// normalize clamps settings to what the server supports.
func normalize(g protocol.GameSettings) protocol.GameSettings {
	if g.Size < 3 || g.Size > 8 {
		g.Size = defaultSize
	}
	g.Komi = min(g.Komi, 8)
	g.Pieces = max(min(g.Pieces, 80), 10)
	g.Capstones = min(g.Capstones, 5)
	return g
}
