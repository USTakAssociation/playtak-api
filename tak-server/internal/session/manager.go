// Package session connects client connections to the services behind them.
// Manager tracks every open connection; each Session holds one connection's
// state and turns its commands into service calls.
package session

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/auth"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/chat"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/game"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/moderation"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/online"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/seek"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport"
)

// kickWait is how long a new login waits for the sessions it replaces to
// finish cleaning up.
const kickWait = time.Second

type Config struct {
	// AllowMultipleSessions lets a player stay logged in on several
	// connections at once. When false, a new login disconnects the old ones.
	AllowMultipleSessions bool
}

type Manager struct {
	ctx        context.Context
	auth       *auth.Service
	players    *online.Registry
	seeks      *seek.Manager
	games      *game.Manager
	chat       *chat.Service
	moderation *moderation.Service
	config     Config
	log        *slog.Logger

	nextID atomic.Int64

	mu       sync.Mutex
	sessions map[*Session]struct{}

	// loginMu serializes binding sessions to players, so two simultaneous
	// logins for the same player can't both skip kicking the other.
	loginMu sync.Mutex
}

// Services are what sessions turn client commands into calls on.
type Services struct {
	Auth       *auth.Service
	Players    *online.Registry
	Seeks      *seek.Manager
	Games      *game.Manager
	Chat       *chat.Service
	Moderation *moderation.Service
}

// NewManager returns a Manager whose sessions stop their work when ctx ends.
func NewManager(ctx context.Context, services Services, config Config, log *slog.Logger) *Manager {
	return &Manager{
		ctx:        ctx,
		auth:       services.Auth,
		players:    services.Players,
		seeks:      services.Seeks,
		games:      services.Games,
		chat:       services.Chat,
		moderation: services.Moderation,
		config:     config,
		log:        log,
		sessions:   make(map[*Session]struct{}),
	}
}

// Accept starts a session for a new connection and greets the client.
func (m *Manager) Accept(c transport.Conn) transport.Peer {
	id := m.nextID.Add(1)
	session := &Session{
		id:      id,
		manager: m,
		conn:    c,
		log:     m.log.With("session", id, "remote", c.RemoteAddr()),
		done:    make(chan struct{}),
	}
	m.mu.Lock()
	m.sessions[session] = struct{}{}
	m.mu.Unlock()

	session.log.Debug("connected")
	session.Send(protocol.Greeting{})
	return session
}

// CloseAll disconnects every session, e.g. on shutdown.
func (m *Manager) CloseAll(reason transport.CloseReason) {
	for _, s := range m.snapshot() {
		s.conn.Close(reason)
	}
}

func (m *Manager) remove(s *Session) {
	m.mu.Lock()
	delete(m.sessions, s)
	m.mu.Unlock()
}

func (m *Manager) snapshot() []*Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Session, 0, len(m.sessions))
	for s := range m.sessions {
		out = append(out, s)
	}
	return out
}

// bind logs session s in as p. Unless multiple sessions are allowed, p's
// other sessions are told why and disconnected first.
func (m *Manager) bind(session *Session, player *online.Player) {
	m.loginMu.Lock()
	defer m.loginMu.Unlock()

	// single-session: a new login replaces the old connection.
	if !m.config.AllowMultipleSessions {
		old := player.Clients()
		for _, c := range old {
			c.Send(protocol.ServerMessage{Text: "You've logged in from another window. Disconnecting"})
			c.Disconnect()
		}
		deadline := time.After(kickWait)
		for _, c := range old {
			select {
			case <-c.Done():
			case <-deadline:
			}
		}
	}
	m.players.Attach(player, session)
	session.player.Store(player)
}

// readySessions returns the sessions that have finished logging in.
func (m *Manager) readySessions() []*Session {
	var out []*Session
	for _, s := range m.snapshot() {
		if s.ready.Load() {
			out = append(out, s)
		}
	}
	return out
}

// broadcastPresence tells every logged-in session how many players are
// online and who they are (bots excluded from the list).
func (m *Manager) broadcastPresence() {
	online := m.players.Online()
	var names []string
	for _, p := range online {
		if !p.IsBot() {
			names = append(names, p.Name)
		}
	}
	slices.Sort(names)

	targets := m.readySessions()
	for _, s := range targets {
		s.Send(protocol.OnlineCount{Count: len(online)})
	}
	for _, s := range targets {
		s.Send(protocol.OnlinePlayers{Names: names})
	}
}
