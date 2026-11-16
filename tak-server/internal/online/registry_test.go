package online

import (
	"testing"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

type fakeClient struct{ sent []protocol.Event }

func (c *fakeClient) Send(ev protocol.Event) { c.sent = append(c.sent, ev) }
func (c *fakeClient) Disconnect()            {}
func (c *fakeClient) Done() <-chan struct{}  { return nil }
func (c *fakeClient) ProtocolVersion() int   { return 0 }

func TestAcquireSharesOnlinePlayer(t *testing.T) {
	r := NewRegistry()
	rec := players.Record{ID: 1, Name: "alice"}

	a := r.Acquire(rec)
	if r.Lookup("alice") != nil {
		t.Error("an acquired player is not online until a client attaches")
	}
	r.Attach(a, &fakeClient{})

	rec.IsMod = true
	b := r.Acquire(rec)
	if a != b {
		t.Fatal("a second login must share the online player")
	}
	if !a.IsMod() {
		t.Error("Acquire should refresh flags from the record")
	}
}

func TestDetachDropsRegisteredPlayerWhenLastClientLeaves(t *testing.T) {
	r := NewRegistry()
	p := r.Acquire(players.Record{ID: 1, Name: "alice"})
	c1, c2 := &fakeClient{}, &fakeClient{}
	r.Attach(p, c1)
	r.Attach(p, c2)

	if got := p.Clients(); len(got) != 2 || got[0] != c1 || got[1] != c2 {
		t.Fatalf("Clients should be in login order: %v", got)
	}
	r.Detach(p, c1)
	if r.Lookup("alice") != p || len(p.Clients()) != 1 {
		t.Fatal("alice should stay online with one client")
	}
	r.Detach(p, c2)
	if r.Lookup("alice") != nil {
		t.Error("alice should be dropped once offline")
	}
}

func TestSendReachesEveryClient(t *testing.T) {
	r := NewRegistry()
	p := r.Acquire(players.Record{ID: 1, Name: "alice"})
	c1, c2 := &fakeClient{}, &fakeClient{}
	r.Attach(p, c1)
	r.Attach(p, c2)
	p.Send(protocol.OK{})
	if len(c1.sent) != 1 || len(c2.sent) != 1 {
		t.Errorf("sent = %d, %d", len(c1.sent), len(c2.sent))
	}
}

func TestGuests(t *testing.T) {
	r := NewRegistry()
	a := r.Guest("abcdefghijklmnopqrst")
	if r.Guest("abcdefghijklmnopqrst") != a {
		t.Error("same token should return the same guest")
	}
	if r.Guest("") == r.Guest("") {
		t.Error("tokenless guests must be distinct")
	}
	c := &fakeClient{}
	r.Attach(a, c)
	r.Detach(a, c)
	if r.Lookup(a.Name) != a {
		t.Error("offline guests are kept for rejoining")
	}
}

func TestOnlineListsOnlyConnectedPlayers(t *testing.T) {
	r := NewRegistry()
	alice := r.Acquire(players.Record{ID: 1, Name: "alice"})
	r.Attach(alice, &fakeClient{})
	r.Attach(alice, &fakeClient{})
	r.Guest("") // never connected

	online := r.Online()
	if len(online) != 1 || online[0] != alice {
		t.Errorf("Online = %v, want [alice]", online)
	}
}

func TestPruneGuests(t *testing.T) {
	r := NewRegistry()
	stale := r.Guest("staleeeeeeeeeeeeeeee")
	online := r.Guest("")
	recent := r.Guest("")
	r.Attach(online, &fakeClient{})

	old := time.Now().Add(-2 * time.Hour)
	stale.lastActive = old
	online.lastActive = old

	if n := r.PruneGuests(time.Hour); n != 1 {
		t.Errorf("pruned %d, want 1", n)
	}
	if r.Lookup(stale.Name) != nil {
		t.Error("stale guest kept")
	}
	if r.Guest("staleeeeeeeeeeeeeeee") == stale {
		t.Error("stale token should no longer resolve")
	}
	if r.Lookup(online.Name) == nil || r.Lookup(recent.Name) == nil {
		t.Error("pruned a guest that should be kept")
	}
}

func TestFindOnline(t *testing.T) {
	r := NewRegistry()
	alice := r.Acquire(players.Record{ID: 1, Name: "Alice"})
	if r.FindOnline("Alice") != nil {
		t.Error("not online until a client attaches")
	}
	c := &fakeClient{}
	r.Attach(alice, c)
	for _, name := range []string{"Alice", "alice", "ALICE"} {
		if r.FindOnline(name) != alice {
			t.Errorf("FindOnline(%q) should find Alice", name)
		}
	}
	if r.FindOnline("Alic") != nil || r.FindOnline("") != nil {
		t.Error("only whole names match")
	}

	// A guest who disconnected is kept for rejoining but isn't online.
	g := r.Guest("token")
	gc := &fakeClient{}
	r.Attach(g, gc)
	if r.FindOnline(g.Name) != g {
		t.Error("online guest")
	}
	r.Detach(g, gc)
	if r.FindOnline(g.Name) != nil || r.FindOnline("guest1") != nil {
		t.Error("an offline guest isn't online")
	}
	r.Detach(alice, c)
	if r.FindOnline("alice") != nil {
		t.Error("offline player")
	}
}

func TestRefresh(t *testing.T) {
	r := NewRegistry()
	rec := players.Record{ID: 1, Name: "alice"}
	alice := r.Acquire(rec)
	r.Attach(alice, &fakeClient{})
	rec.IsGagged, rec.IsMod = true, true
	r.Refresh(rec)
	if !alice.IsGagged() || !alice.IsMod() {
		t.Error("online player should pick up the new flags")
	}
	r.Refresh(players.Record{ID: 2, Name: "nobody", IsAdmin: true}) // not in memory: no-op

	// A guest is never refreshed from a record, even with the same name.
	g := r.Guest("")
	r.Attach(g, &fakeClient{})
	r.Refresh(players.Record{Name: g.Name, IsAdmin: true})
	if g.IsAdmin() {
		t.Error("guests must not take flags from a record")
	}
	g.SetGagged(true)
	if !g.IsGagged() {
		t.Error("SetGagged")
	}
}
