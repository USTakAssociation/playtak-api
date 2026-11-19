package session

import (
	"context"
	"testing"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport"
)

// loginAs logs a registered player in and drains the login messages.
func loginAs(h *harness, name string) *client {
	c := h.connect()
	c.send("Login " + name + " password")
	c.drainUntil("OnlinePlayers")
	return c
}

func TestShoutThroughProtocol(t *testing.T) {
	h := newHarness(t, Config{})
	a := loginGuestWith(h, "")
	b := loginGuestWith(h, "")
	a.drainUntil("OnlinePlayers")
	lurker := h.connect() // not logged in: hears nothing

	// Chat isn't filtered: words reach everyone as sent.
	a.send("Shout hello darn world 😀")
	a.expect("Shout <Guest1> hello darn world 😀")
	b.expect("Shout <Guest1> hello darn world 😀")
	lurker.expectNothing()
}

func TestGaggedShoutThroughProtocol(t *testing.T) {
	h := newHarness(t, Config{})
	h.addPlayer("alice", false)
	h.store.SetFlag(context.Background(), "alice", players.FlagGagged, true)
	other := loginGuestWith(h, "")
	a := loginAs(h, "alice")
	other.drainUntil("OnlinePlayers")
	a.send("Shout hello")
	a.expect("Shout <alice> <Server: You have been muted for inappropriate chat behavior.>")
	other.expectNothing()

	a.send("Tell Guest1 hi")
	a.expect("Told <Guest1> <Server: You have been muted for inappropriate chat behavior.>")
	other.expectNothing()
}

func TestTellThroughProtocol(t *testing.T) {
	h := newHarness(t, Config{})
	h.addPlayer("Alice", false)
	h.addPlayer("bobby", false)
	a := loginAs(h, "Alice")
	b := loginAs(h, "bobby")
	a.drainUntil("OnlinePlayers")

	// Names match ignoring case; Told shows the name as registered.
	b.send("Tell alice psst darn")
	a.expect("Tell <bobby> psst darn")
	b.expect("Told <Alice> psst darn")

	b.send("Tell nobody1 hi")
	b.expect("Error:Player is not online")
	h.addPlayer("carol", false) // registered but offline
	b.send("Tell carol hi")
	b.expect("Error:Player is not online")
	a.expectNothing()
}

func TestTellReachesEverySessionInMultiSession(t *testing.T) {
	h := newHarness(t, Config{AllowMultipleSessions: true})
	h.addPlayer("alice", false)
	h.addPlayer("bobby", false)
	a1 := loginAs(h, "alice")
	a2 := loginAs(h, "alice")
	b1 := loginAs(h, "bobby")
	b2 := loginAs(h, "bobby")
	for _, c := range []*client{a1, a2, b1, b2} {
		c.settle()
	}

	b1.send("Tell alice hi")
	a1.expect("Tell <bobby> hi")
	a2.expect("Tell <bobby> hi")
	b1.expect("Told <alice> hi")
	b2.expectNothing() // the copy goes to the sending session only
}

func TestRoomsThroughProtocol(t *testing.T) {
	h := newHarness(t, Config{})
	a := loginGuestWith(h, "")
	b := loginGuestWith(h, "")
	c := loginGuestWith(h, "")
	a.drainUntil("OnlinePlayers")
	a.drainUntil("OnlinePlayers")
	b.drainUntil("OnlinePlayers")

	a.send("JoinRoom lobby")
	a.expect("Joined room lobby")
	b.send("JoinRoom lobby")
	b.expect("Joined room lobby")

	a.send("ShoutRoom lobby hi darn")
	a.expect("ShoutRoom lobby <Guest1> hi darn")
	b.expect("ShoutRoom lobby <Guest1> hi darn")
	c.expectNothing()

	c.send("ShoutRoom lobby let me in")
	c.expect("Error:You are not in that room")
	a.expectNothing()

	b.send("LeaveRoom lobby")
	b.expect("OK")
	a.send("ShoutRoom lobby bye")
	a.expect("ShoutRoom lobby <Guest1> bye")
	b.expectNothing()
	b.send("LeaveRoom lobby") // not a member: still OK, as Java
	b.expect("OK")
}

func TestDisconnectLeavesRooms(t *testing.T) {
	h := newHarness(t, Config{})
	a := loginGuestWith(h, "")
	b := loginGuestWith(h, "")
	a.drainUntil("OnlinePlayers")
	b.send("JoinRoom lobby")
	b.expect("Joined room lobby")
	s := b.conn.peer.(*Session)
	if got := h.m.chat.JoinedRooms(s); len(got) != 1 {
		t.Fatalf("rooms %v", got)
	}
	b.send("quit")
	b.expectClosed(transport.CloseNormal)
	<-s.Done()
	if got := h.m.chat.JoinedRooms(s); len(got) != 0 {
		t.Errorf("closed session still in %v", got)
	}
	a.settle()
	// The room was deleted with its last member, so joining creates it anew
	// and nobody else is in it.
	a.send("JoinRoom lobby")
	a.expect("Joined room lobby")
	a.send("ShoutRoom lobby anyone")
	a.expect("ShoutRoom lobby <Guest1> anyone")
	a.expectNothing()
}

func TestShoutFloodRefused(t *testing.T) {
	h := newHarness(t, Config{})
	a := loginGuestWith(h, "")
	b := loginGuestWith(h, "")
	a.drainUntil("OnlinePlayers")
	for range 10 {
		a.send("Shout hi")
		a.expect("Shout <Guest1> hi")
		b.expect("Shout <Guest1> hi")
	}
	a.send("Shout spam")
	a.expect("Error:You are sending messages too fast")
	b.expectNothing()
}
