package session

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/auth"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/chat"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/dbtest"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/games"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/game"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/moderation"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/online"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/seek"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/wordfilter"
)

// fakeConn stands in for a transport connection. Closing it ends the
// "connection" the way a real transport would: the peer's Closed runs next.
type fakeConn struct {
	lines chan string
	peer  transport.Peer

	once   sync.Once
	reason transport.CloseReason
	closed chan struct{}
}

func (c *fakeConn) Send(line string) {
	select {
	case <-c.closed:
	default:
		c.lines <- line
	}
}

func (c *fakeConn) Close(reason transport.CloseReason) {
	c.once.Do(func() {
		c.reason = reason
		close(c.closed)
		go c.peer.Closed()
	})
}

func (c *fakeConn) RemoteAddr() string { return "test" }

type harness struct {
	t       *testing.T
	m       *Manager
	store   *players.Store
	gamesDB *sql.DB
	mail    *sentMail
}

// sentMail records emails instead of sending them.
type sentMail struct {
	mu   sync.Mutex
	sent []string // "to|subject|body"
}

func (m *sentMail) Send(_ context.Context, to, subject, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, to+"|"+subject+"|"+body)
	return nil
}

func (m *sentMail) all() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.sent...)
}

func newHarness(t *testing.T, cfg Config) *harness {
	return newHarnessWithGames(t, cfg, game.Config{ReconnectTimeout: time.Minute, CommandTimeout: 5 * time.Second, AllowMultipleSessions: cfg.AllowMultipleSessions})
}

func newHarnessWithGames(t *testing.T, cfg Config, gameCfg game.Config) *harness {
	t.Helper()
	conn := dbtest.NewPlayersDB(t)
	store := players.NewStore(conn)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mail := &sentMail{}
	authSvc := auth.NewService(store, players.NewTokenStore(conn), mail, &wordfilter.Filter{}, log)
	registry := online.NewRegistry()
	seeks := seek.NewManager()

	gamesDB := dbtest.NewGamesDB(t)
	gameStore := games.NewStore(gamesDB)
	results := games.NewResults(gameStore, filepath.Join(t.TempDir(), "pending.jsonl"), log)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go results.Run(ctx)
	gm := game.NewManager(gameCfg, gameStore, store, results, seeks, registry, log)

	reload := func(context.Context) (int, error) { return 0, nil }
	m := NewManager(context.Background(), Services{
		Auth:       authSvc,
		Players:    registry,
		Seeks:      seeks,
		Games:      gm,
		Chat:       chat.NewService(maskWord{"darn"}, log),
		Moderation: moderation.NewService(store, authSvc, gm, registry, mail, reload, log),
	}, cfg, log)
	return &harness{t: t, m: m, store: store, gamesDB: gamesDB, mail: mail}
}

// maskWord masks one word, standing in for the profanity filter.
type maskWord struct{ word string }

func (w maskWord) Mask(s string) string {
	parts := strings.Split(s, " ")
	for i, p := range parts {
		if p == w.word {
			parts[i] = "****"
		}
	}
	return strings.Join(parts, " ")
}

// addPlayer creates a registered player whose password is "password".
func (h *harness) addPlayer(name string, mod bool) {
	h.t.Helper()
	// bcrypt hash of "password"
	const hash = "$2a$10$JpGWa00gDsDtJK0Ttq/dD.F2zTj9kCkdcHwO5pIZFfWbT3CfDY/C6"
	if _, err := h.store.Create(context.Background(), name, name+"@example.com", hash); err != nil {
		h.t.Fatal(err)
	}
	if mod {
		h.store.SetFlag(context.Background(), name, players.FlagMod, true)
	}
}

type client struct {
	t    *testing.T
	conn *fakeConn
}

// connect opens a connection and consumes the greeting.
func (h *harness) connect() *client {
	h.t.Helper()
	fc := &fakeConn{lines: make(chan string, 100), closed: make(chan struct{})}
	fc.peer = h.m.Accept(fc)
	c := &client{t: h.t, conn: fc}
	c.expect("Welcome!")
	c.expect("Login or Register")
	return c
}

func (c *client) send(line string) { c.conn.peer.HandleLine(line) }

func (c *client) recv() string {
	c.t.Helper()
	select {
	case l := <-c.conn.lines:
		return l
	case <-time.After(5 * time.Second):
		c.t.Fatal("timed out waiting for a message")
		return ""
	}
}

func (c *client) expect(want string) {
	c.t.Helper()
	if got := c.recv(); got != want {
		c.t.Errorf("got %q, want %q", got, want)
	}
}

func (c *client) drainUntil(prefix string) string {
	c.t.Helper()
	for {
		if l := c.recv(); strings.HasPrefix(l, prefix) {
			return l
		}
	}
}

// settle discards messages until none arrives for a moment, e.g. presence
// updates from other logins.
func (c *client) settle() {
	for {
		select {
		case <-c.conn.lines:
		case <-time.After(50 * time.Millisecond):
			return
		}
	}
}

func (c *client) expectNothing() {
	c.t.Helper()
	select {
	case l := <-c.conn.lines:
		c.t.Errorf("unexpected message %q", l)
	case <-time.After(50 * time.Millisecond):
	}
}

func (c *client) expectClosed(reason transport.CloseReason) {
	c.t.Helper()
	select {
	case <-c.conn.closed:
		if c.conn.reason != reason {
			c.t.Errorf("closed with %v, want %v", c.conn.reason, reason)
		}
	case <-time.After(5 * time.Second):
		c.t.Fatal("connection not closed")
	}
}

func TestGuestLogin(t *testing.T) {
	h := newHarness(t, Config{})
	c := h.connect()
	c.send("Login Guest")
	c.expect("Welcome Guest1!")
	c.expect("Online 1")
	c.expect(`OnlinePlayers ["Guest1"]`)
}

func TestNamedLogin(t *testing.T) {
	h := newHarness(t, Config{})
	h.addPlayer("alice", false)
	h.addPlayer("modbob", true)

	a := h.connect()
	a.send("Login alice wrongpass")
	a.expect("Authentication failure")
	a.send("Login alice password")
	a.expect("Welcome alice!")
	a.expect("Online 1")
	a.expect(`OnlinePlayers ["alice"]`)

	b := h.connect()
	b.send("Login modbob password")
	b.expect("Welcome modbob!")
	b.expect("Is Mod")
	b.expect("Online 2")
	b.expect(`OnlinePlayers ["alice", "modbob"]`)
	a.expect("Online 2")
	a.expect(`OnlinePlayers ["alice", "modbob"]`)
}

func TestUnknownCommands(t *testing.T) {
	h := newHarness(t, Config{})
	c := h.connect()
	c.send("nonsense command xyz")
	c.expect("NOK")
	c.send("ChangePassword old123 new123") // only after login
	c.expect("NOK")
	c.send("Login Guest")
	c.drainUntil("OnlinePlayers")
	c.send("Login Guest") // not after login
	c.expect("NOK")
}

func TestPingAndProtocol(t *testing.T) {
	h := newHarness(t, Config{})
	c := h.connect()
	c.send("PING")
	c.expect("OK")
	c.send("Protocol 4")
	c.expect("OK")
	c.send("Client TakWeb-16.05.26")
	c.expect("Shout <Server> Your Playtak client is unfortunately no longer compatible. Please go to https://www.playtak.com in order to play.")
	c.send("Client TestSuite-1.0")
	c.expect("OK")
}

func TestSecondLoginKicksFirstByDefault(t *testing.T) {
	h := newHarness(t, Config{})
	h.addPlayer("alice", false)
	c1 := h.connect()
	c1.send("Login alice password")
	c1.drainUntil("OnlinePlayers")

	c2 := h.connect()
	c2.send("Login alice password")
	c1.expect("Message You've logged in from another window. Disconnecting")
	c1.expectClosed(transport.CloseKicked)
	c2.expect("Welcome alice!")
	c2.expect("Online 1")
}

func TestGuestTokenRejoinKicksFirst(t *testing.T) {
	h := newHarness(t, Config{})
	c1 := h.connect()
	c1.send("Login Guest abcdefghijklmnopqrst")
	c1.expect("Welcome Guest1!")
	c1.drainUntil("OnlinePlayers")

	c2 := h.connect()
	c2.send("Login Guest abcdefghijklmnopqrst")
	c1.expect("Message You've logged in from another window. Disconnecting")
	c1.expectClosed(transport.CloseKicked)
	c2.expect("Welcome Guest1!")
}

func TestMultipleSessionsWhenAllowed(t *testing.T) {
	h := newHarness(t, Config{AllowMultipleSessions: true})
	h.addPlayer("alice", false)
	c1 := h.connect()
	c1.send("Login alice password")
	c1.drainUntil("OnlinePlayers")

	c2 := h.connect()
	c2.send("Login alice password")
	c2.expect("Welcome alice!")
	c2.expect("Online 1") // distinct players, not connections
	c2.expect(`OnlinePlayers ["alice"]`)
	c1.expect("Online 1")
	c1.expect(`OnlinePlayers ["alice"]`)

	// Closing one session leaves alice online through the other.
	c1.send("quit")
	c1.expectClosed(transport.CloseNormal)
	c2.expect("Online 1")
}

func TestDisconnectUpdatesPresence(t *testing.T) {
	h := newHarness(t, Config{})
	c1 := h.connect()
	c1.send("Login Guest")
	c1.drainUntil("OnlinePlayers")
	c2 := h.connect()
	c2.send("Login Guest")
	c2.drainUntil("OnlinePlayers")
	c1.drainUntil("OnlinePlayers")

	c1.send("quit")
	c1.expectClosed(transport.CloseNormal)
	c2.expect("Online 1")
	c2.expect(`OnlinePlayers ["Guest2"]`)
}

func TestQuitIgnoresLaterLines(t *testing.T) {
	h := newHarness(t, Config{})
	c := h.connect()
	c.send("quit")
	c.send("PING")
	c.expectClosed(transport.CloseNormal)
	c.expectNothing()
}

func TestRegistration(t *testing.T) {
	h := newHarness(t, Config{})
	h.addPlayer("alice", false)
	c := h.connect()
	c.send("Register justausername")
	c.expect("Registration Error: Unknown format for username/email. Only [a-z][A-Z][0-9][_] allowed for username, it should be 4-16 characters and should start with letter")
	c.send("Register GuestPlayer test@example.com")
	c.expect("Registration Error: Can't register with guest in the name")
	c.send("Register a1ice test@example.com")
	c.expect("Registration Error: Username is already taken")
	c.send("Register newbie test@example.com")
	c.expect("Registered newbie. Check your email for password")
}

func TestPasswordCommands(t *testing.T) {
	h := newHarness(t, Config{})
	h.addPlayer("alice", false)
	c := h.connect()
	c.send("SendResetToken nobody a@example.com")
	c.expect("Reset Token Error: No such player")
	c.send("SendResetToken alice wrong@example.com")
	c.expect("Reset Token Error: Email address does not match")
	c.send("ResetPassword alice badtoken newpass1")
	c.expect("Wrong token")
	c.send("ResetPassword nobody badtoken newpass1")
	c.expect("No such player")

	c.send("Login alice password")
	c.drainUntil("OnlinePlayers")
	c.send("ChangePassword wrongold newpass1")
	c.expect("Wrong password")
	c.send("ChangePassword password newpass1")
	c.expect("Password changed")

	g := h.connect()
	g.send("Login Guest")
	g.drainUntil("OnlinePlayers")
	g.send("ChangePassword anything newpass1")
	g.expect("Wrong password")
}
