package player

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
