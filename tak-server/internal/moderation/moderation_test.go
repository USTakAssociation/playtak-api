package moderation

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/dbtest"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/online"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

type fakeAccounts struct {
	mu        sync.Mutex
	passwords map[string]string
	revoked   []string
}

func (a *fakeAccounts) SetPassword(_ context.Context, name, pw string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.passwords[name] = pw
	return nil
}

func (a *fakeAccounts) RevokeAll(_ context.Context, name string) (int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.revoked = append(a.revoked, name)
	return 3, nil
}

type fakeGames struct {
	mu        sync.Mutex
	forfeited []string
}

func (g *fakeGames) Forfeit(_ context.Context, name string) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.forfeited = append(g.forfeited, name)
	return true, nil
}

type fakeMail struct{ sent chan string }

func (m *fakeMail) Send(_ context.Context, to, subject, body string) error {
	m.sent <- to + "|" + subject + "|" + body
	return nil
}

type conn struct {
	mu     sync.Mutex
	kicked bool
}

func (c *conn) Send(protocol.Event)   {}
func (c *conn) ProtocolVersion() int  { return 0 }
func (c *conn) Done() <-chan struct{} { return nil }
func (c *conn) Disconnect()           { c.mu.Lock(); c.kicked = true; c.mu.Unlock() }
func (c *conn) wasKicked() bool       { c.mu.Lock(); defer c.mu.Unlock(); return c.kicked }

type env struct {
	t        *testing.T
	s        *Service
	store    *players.Store
	registry *online.Registry
	accounts *fakeAccounts
	games    *fakeGames
	mail     *fakeMail
	log      *bytes.Buffer
}

// newEnv has players pat (player), mona and mike (mods), ada and abe
// (admins) and val, an admin who isn't also flagged mod.
func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	e := &env{t: t, store: players.NewStore(dbtest.NewPlayersDB(t)), registry: online.NewRegistry(),
		accounts: &fakeAccounts{passwords: map[string]string{}}, games: &fakeGames{},
		mail: &fakeMail{sent: make(chan string, 10)}, log: &bytes.Buffer{}}
	for _, n := range []string{"pat", "mona", "mike", "ada", "abe", "val"} {
		if _, err := e.store.Create(ctx, n, n+"@example.com", "hash"); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range []string{"mona", "mike", "ada", "abe"} {
		e.store.SetFlag(ctx, n, players.FlagMod, true)
	}
	for _, n := range []string{"ada", "abe", "val"} {
		e.store.SetFlag(ctx, n, players.FlagAdmin, true)
	}
	e.s = NewService(e.store, e.accounts, e.games, e.registry, e.mail, slog.New(slog.NewTextHandler(e.log, nil)))
	return e
}

// online brings a registered player online and returns their connection.
func (e *env) online(name string) (*online.Player, *conn) {
	e.t.Helper()
	rec, err := e.store.ByName(context.Background(), name)
	if err != nil {
		e.t.Fatal(err)
	}
	p := e.registry.Acquire(rec)
	c := &conn{}
	e.registry.Attach(p, c)
	return p, c
}

// actor returns an in-memory player for name to run commands as.
func (e *env) actor(name string) *online.Player {
	rec, _ := e.store.ByName(context.Background(), name)
	return e.registry.Acquire(rec)
}

func (e *env) run(actor, line string, a protocol.SudoAction, name, arg string) Result {
	return e.s.Run(context.Background(), e.actor(actor), protocol.Sudo{Text: line, Action: a, Name: name, Arg: arg})
}

func (e *env) record(name string) players.Record {
	rec, _ := e.store.ByName(context.Background(), name)
	return rec
}

func want(t *testing.T, got Result, lines ...string) {
	t.Helper()
	if !slices.Equal(got.Replies, lines) {
		t.Errorf("replies %q, want %q", got.Replies, lines)
	}
}

func TestAllowed(t *testing.T) {
	e := newEnv(t)
	for name, ok := range map[string]bool{"pat": false, "mona": true, "ada": true, "val": true} {
		if Allowed(e.actor(name)) != ok {
			t.Errorf("Allowed(%s) != %v", name, ok)
		}
	}
	if Allowed(e.registry.Guest("")) {
		t.Error("guests can't use sudo")
	}
}

func TestRanks(t *testing.T) {
	// Who may gag whom; kick, ban and unban use the same rule.
	e := newEnv(t)
	for _, tc := range []struct {
		actor, target string
		ok            bool
	}{
		{"mona", "pat", true},
		{"mona", "mike", false}, // same rank
		{"mona", "mona", false}, // self
		{"mona", "ada", false},
		{"mona", "val", false}, // an admin not flagged mod (Java let mods act on them)
		{"ada", "pat", true},
		{"ada", "mona", true},
		{"ada", "abe", false},
		{"ada", "val", false},
		{"ada", "ada", false},
		{"val", "mona", true}, // admin without the mod flag still outranks mods
	} {
		got := e.run(tc.actor, "sudo gag "+tc.target, protocol.SudoGag, tc.target, "")
		if ok := got.Replies[0] == tc.target+" gagged"; ok != tc.ok {
			t.Errorf("%s gag %s: %q", tc.actor, tc.target, got.Replies)
		}
		e.store.SetFlag(context.Background(), tc.target, players.FlagGagged, false)
	}
}

func TestGag(t *testing.T) {
	e := newEnv(t)
	// Offline, and the name matches ignoring case; replies use the
	// registered spelling.
	want(t, e.run("mona", "", protocol.SudoGag, "PAT", ""), "pat gagged")
	if !e.record("pat").IsGagged {
		t.Error("gag not saved")
	}
	want(t, e.run("mona", "", protocol.SudoGag, "pat", ""), "Player is already gagged")
	want(t, e.run("mona", "", protocol.SudoUngag, "pat", ""), "pat ungagged")
	want(t, e.run("mona", "", protocol.SudoUngag, "pat", ""), "Player is not gagged")
	want(t, e.run("mona", "", protocol.SudoGag, "nobody", ""), "No such player")

	// Online: the in-memory player changes too.
	p, _ := e.online("pat")
	e.run("mona", "", protocol.SudoGag, "pat", "")
	if !p.IsGagged() {
		t.Error("online player not gagged")
	}
	e.run("mona", "", protocol.SudoUngag, "pat", "")
	if p.IsGagged() {
		t.Error("online player not ungagged")
	}
}

func TestGagGuest(t *testing.T) {
	e := newEnv(t)
	g := e.registry.Guest("")
	c := &conn{}
	e.registry.Attach(g, c)
	want(t, e.run("mona", "", protocol.SudoGag, strings.ToLower(g.Name), ""), g.Name+" gagged")
	if !g.IsGagged() {
		t.Error("guest not gagged")
	}
	want(t, e.run("mona", "", protocol.SudoGag, g.Name, ""), "Player is already gagged")
	want(t, e.run("mona", "", protocol.SudoList, "", "gag"), "["+g.Name+"]")
	want(t, e.run("mona", "", protocol.SudoUngag, g.Name, ""), g.Name+" ungagged")
	e.registry.Detach(g, c)
	want(t, e.run("mona", "", protocol.SudoGag, g.Name, ""), "No such player") // offline guests are gone
}

func TestKick(t *testing.T) {
	e := newEnv(t)
	_, c := e.online("pat")
	_, c2 := e.online("pat") // a second connection
	want(t, e.run("mona", "", protocol.SudoKick, "Pat", ""), "pat kicked")
	if !c.wasKicked() || !c2.wasKicked() {
		t.Error("every connection should be closed")
	}
	// Kick only looks at who's online.
	want(t, e.run("mona", "", protocol.SudoKick, "mike", ""), "Player not logged in")
	want(t, e.run("mona", "", protocol.SudoKick, "nobody", ""), "Player not logged in")
	_, ac := e.online("ada")
	want(t, e.run("mona", "", protocol.SudoKick, "ada", ""), "You don't have rights")
	if ac.wasKicked() {
		t.Error("refused kick closed the connection")
	}
	g := e.registry.Guest("")
	gc := &conn{}
	e.registry.Attach(g, gc)
	want(t, e.run("mona", "", protocol.SudoKick, g.Name, ""), g.Name+" kicked")
	if !gc.wasKicked() {
		t.Error("guest not kicked")
	}
}

func TestBanOffline(t *testing.T) {
	e := newEnv(t)
	want(t, e.run("mona", "", protocol.SudoBan, "PAT", "spamming chat"), "pat banned")
	if !e.record("pat").IsBanned {
		t.Error("ban not saved")
	}
	if !slices.Equal(e.accounts.revoked, []string{"pat"}) {
		t.Errorf("tokens revoked for %v", e.accounts.revoked)
	}
	if !slices.Equal(e.games.forfeited, []string{"pat"}) {
		t.Errorf("games forfeited for %v", e.games.forfeited)
	}
	select {
	case m := <-e.mail.sent:
		if m != "pat@example.com|PlayTak.com Ban Notice|pat,\nYou have been banned from playtak.com!\nReason: spamming chat" {
			t.Errorf("mail %q", m)
		}
	case <-time.After(2 * time.Second):
		t.Error("no ban notice sent")
	}
	want(t, e.run("mona", "", protocol.SudoBan, "pat", "again"), "Player is already banned")
}

func TestBanOnline(t *testing.T) {
	e := newEnv(t)
	p, c := e.online("pat")
	want(t, e.run("mona", "", protocol.SudoBan, "pat", "cheating"), "pat kicked", "pat banned")
	if !c.wasKicked() || !p.IsBanned() {
		t.Errorf("kicked %v, banned in memory %v", c.wasKicked(), p.IsBanned())
	}
}

func TestBanRefusals(t *testing.T) {
	e := newEnv(t)
	g := e.registry.Guest("")
	e.registry.Attach(g, &conn{})
	want(t, e.run("mona", "", protocol.SudoBan, g.Name, "x"), "Guests can't be banned, use kick")
	want(t, e.run("mona", "", protocol.SudoBan, "nobody", "x"), "No such player")
	want(t, e.run("mona", "", protocol.SudoBan, "val", "x"), "You don't have rights")
	if len(e.accounts.revoked)+len(e.games.forfeited) != 0 {
		t.Error("refused bans must have no side effects")
	}

	// No address, no email.
	e.store.SetFlag(context.Background(), "pat", players.FlagBanned, false)
	e.store.Create(context.Background(), "noemail", "", "hash")
	want(t, e.run("mona", "", protocol.SudoBan, "noemail", "x"), "noemail banned")
	select {
	case m := <-e.mail.sent:
		t.Errorf("sent %q", m)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestUnban(t *testing.T) {
	e := newEnv(t)
	want(t, e.run("mona", "", protocol.SudoUnban, "pat", ""), "Player is not banned")
	e.run("mona", "", protocol.SudoBan, "pat", "x")
	want(t, e.run("mona", "", protocol.SudoUnban, "PAT", ""), "pat unbanned")
	if e.record("pat").IsBanned {
		t.Error("unban not saved")
	}
	want(t, e.run("mona", "", protocol.SudoUnban, "ada", ""), "You don't have rights")
	want(t, e.run("mona", "", protocol.SudoUnban, "nobody", ""), "No such player")
}

func TestList(t *testing.T) {
	e := newEnv(t)
	want(t, e.run("mona", "", protocol.SudoList, "", "mod"), "[abe, ada, mike, mona]")
	want(t, e.run("mona", "", protocol.SudoList, "", "admin"), "[abe, ada, val]")
	want(t, e.run("mona", "", protocol.SudoList, "", "ban"), "[]")
	want(t, e.run("mona", "", protocol.SudoList, "", "gag"), "[]")
	e.online("pat")
	e.online("pat") // two connections, listed once
	e.online("mona")
	want(t, e.run("mona", "", protocol.SudoList, "", "online"), "[mona, pat]")
	want(t, e.run("mona", "", protocol.SudoList, "", "nonsense"), "command not found")
}

func TestAdminOnlyCommands(t *testing.T) {
	e := newEnv(t)
	for _, a := range []protocol.SudoAction{protocol.SudoBroadcast, protocol.SudoMod,
		protocol.SudoUnmod, protocol.SudoAdmin, protocol.SudoUnadmin, protocol.SudoBot, protocol.SudoUnbot,
		protocol.SudoSetPassword, protocol.SudoRevoke} {
		got := e.run("mona", "", a, "pat", "secretpw")
		if !slices.Equal(got.Replies, []string{"Permission denied"}) || got.Broadcast != "" {
			t.Errorf("action %d by a mod: %+v", a, got)
		}
	}
	if len(e.accounts.passwords) != 0 || len(e.accounts.revoked) != 0 || e.record("pat").IsMod {
		t.Error("denied commands must do nothing")
	}
}

func TestBroadcast(t *testing.T) {
	e := newEnv(t)
	// Passed through untouched: the session sends it as a raw line.
	for _, arg := range []string{
		"Message Server going down in 5 minutes",
		"Error: Maintenance at noon",
		"Shout <Server> hello everyone",
		"ShoutRoom lobby <Server> hello room",
		"Tell <Server> hello",
	} {
		got := e.run("ada", "", protocol.SudoBroadcast, "", arg)
		if got.Broadcast != arg || len(got.Replies) != 0 {
			t.Errorf("broadcast %q: %+v", arg, got)
		}
	}
}

func TestRoles(t *testing.T) {
	e := newEnv(t)
	p, _ := e.online("pat")
	want(t, e.run("ada", "", protocol.SudoMod, "PAT", ""), "Added pat as moderator")
	if !e.record("pat").IsMod || !p.IsMod() {
		t.Error("mod not saved or not applied online")
	}
	want(t, e.run("ada", "", protocol.SudoMod, "pat", ""), "pat is already a moderator")
	want(t, e.run("ada", "", protocol.SudoUnmod, "pat", ""), "Removed pat as moderator")
	want(t, e.run("ada", "", protocol.SudoUnmod, "pat", ""), "pat is not a moderator")
	want(t, e.run("ada", "", protocol.SudoAdmin, "pat", ""), "Added pat as admin")
	want(t, e.run("ada", "", protocol.SudoAdmin, "pat", ""), "pat is already an admin")
	want(t, e.run("ada", "", protocol.SudoUnadmin, "pat", ""), "Removed pat as admin")
	want(t, e.run("ada", "", protocol.SudoUnadmin, "pat", ""), "pat is not an admin")
	want(t, e.run("ada", "", protocol.SudoBot, "pat", ""), "Added pat as bot")
	want(t, e.run("ada", "", protocol.SudoBot, "pat", ""), "pat is already a bot")
	want(t, e.run("ada", "", protocol.SudoUnbot, "pat", ""), "Removed pat as bot")
	if r := e.record("pat"); r.IsAdmin || r.IsBot || r.IsMod {
		t.Errorf("roles left: %+v", r)
	}
	// Admins may change other admins' roles, but not their own.
	want(t, e.run("ada", "", protocol.SudoUnadmin, "abe", ""), "Removed abe as admin")
	want(t, e.run("ada", "", protocol.SudoUnadmin, "ADA", ""), "You can't change your own roles")
	want(t, e.run("ada", "", protocol.SudoMod, "nobody", ""), "No such player")
}

func TestSetPassword(t *testing.T) {
	e := newEnv(t)
	want(t, e.run("ada", "sudo set password Pat ******", protocol.SudoSetPassword, "Pat", "n3wpassword"), "Password set")
	if e.accounts.passwords["pat"] != "n3wpassword" || !slices.Equal(e.accounts.revoked, []string{"pat"}) {
		t.Errorf("passwords %v, revoked %v", e.accounts.passwords, e.accounts.revoked)
	}
	want(t, e.run("ada", "", protocol.SudoSetPassword, "mona", "n3wpassword"), "Password set")
	want(t, e.run("ada", "", protocol.SudoSetPassword, "abe", "n3wpassword"), "You don't have rights")
	want(t, e.run("ada", "", protocol.SudoSetPassword, "ada", "n3wpassword"), "You don't have rights")
	want(t, e.run("ada", "", protocol.SudoSetPassword, "nobody", "n3wpassword"), "No such player")
	if strings.Contains(e.log.String(), "n3wpassword") {
		t.Errorf("password logged: %s", e.log.String())
	}
	if !strings.Contains(e.log.String(), `command="sudo set password Pat ******"`) {
		t.Errorf("audit log: %s", e.log.String())
	}
}

func TestRevoke(t *testing.T) {
	e := newEnv(t)
	want(t, e.run("ada", "", protocol.SudoRevoke, "PAT", ""), "Revoked 3 tokens for pat")
	want(t, e.run("ada", "", protocol.SudoRevoke, "abe", ""), "You don't have rights")
	want(t, e.run("ada", "", protocol.SudoRevoke, "nobody", ""), "No such player")
}

func TestUnknownCommand(t *testing.T) {
	e := newEnv(t)
	want(t, e.run("mona", "sudo gagg pat", protocol.SudoUnknown, "", ""), "Command not found")
}

func TestAuditLog(t *testing.T) {
	e := newEnv(t)
	e.run("mona", "sudo gag pat", protocol.SudoGag, "pat", "")
	if !strings.Contains(e.log.String(), `msg=sudo by=mona command="sudo gag pat" result="pat gagged"`) {
		t.Errorf("log: %s", e.log.String())
	}
}

type failingStore struct{ Store }

func (failingStore) ByNameFold(context.Context, string) (players.Record, error) {
	return players.Record{}, errors.New("database is locked")
}

func (failingStore) Flagged(context.Context, players.Flag) ([]string, error) {
	return nil, errors.New("database is locked")
}

func TestStoreFailures(t *testing.T) {
	e := newEnv(t)
	e.s.store = failingStore{e.store}
	for _, a := range []protocol.SudoAction{protocol.SudoGag, protocol.SudoBan, protocol.SudoUnban,
		protocol.SudoMod, protocol.SudoSetPassword, protocol.SudoRevoke} {
		if got := e.run("ada", "", a, "pat", "reason"); !slices.Equal(got.Replies, []string{"Server error, please try again later"}) {
			t.Errorf("action %d: %q", a, got.Replies)
		}
	}
	want(t, e.run("ada", "", protocol.SudoList, "", "mod"), "Server error, please try again later")
}
