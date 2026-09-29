package session

import (
	"testing"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport"
)

func TestSeekListSentOnLogin(t *testing.T) {
	h := newHarness(t, Config{})
	poster := h.connect()
	poster.send("Login Guest")
	poster.drainUntil("OnlinePlayers")
	poster.send("Seek 5 600 30 W 0 21 1 0 0 ")
	poster.expect("Seek new 1 Guest1 5 600 30 W 0 21 1 0 0 0 0 ")

	// The seek list arrives after Welcome and before the presence update.
	c := h.connect()
	c.send("Login Guest")
	c.expect("Welcome Guest2!")
	c.expect("Seek new 1 Guest1 5 600 30 W 0 21 1 0 0 0 0 ")
	c.expect("Online 2")
}

func TestSeekBroadcastAndCancel(t *testing.T) {
	h := newHarness(t, Config{})
	a, b := h.connect(), h.connect()
	a.send("Login Guest")
	a.drainUntil("OnlinePlayers")
	b.send("Login Guest")
	b.drainUntil("OnlinePlayers")
	a.drainUntil("OnlinePlayers")

	a.send("Seek 5 600 30")
	a.expect("Seek new 1 Guest1 5 600 30 A 0 21 1 0 0 0 0 ")
	b.expect("Seek new 1 Guest1 5 600 30 A 0 21 1 0 0 0 0 ")

	a.send("Seek 6 900 0") // replaces
	b.expect("Seek remove 1 Guest1 5 600 30 A 0 21 1 0 0 0 0 ")
	b.expect("Seek new 2 Guest1 6 900 0 A 0 30 1 0 0 0 0 ")

	b.send("List")
	b.expect("Seek new 2 Guest1 6 900 0 A 0 30 1 0 0 0 0 ")

	a.send("Seek 0 0 0")
	b.expect("Seek remove 2 Guest1 6 900 0 A 0 30 1 0 0 0 0 ")
}

func TestSeekRemovedOnDisconnect(t *testing.T) {
	h := newHarness(t, Config{})
	a, b := h.connect(), h.connect()
	a.send("Login Guest")
	a.drainUntil("OnlinePlayers")
	b.send("Login Guest")
	b.drainUntil("OnlinePlayers")

	a.send("Seek 5 600 30")
	b.drainUntil("Seek new 1 ")
	a.send("quit")
	a.expectClosed(transport.CloseNormal)
	b.expect("Seek remove 1 Guest1 5 600 30 A 0 21 1 0 0 0 0 ")
}

func TestSeekPerSessionWithMultipleSessions(t *testing.T) {
	h := newHarness(t, Config{AllowMultipleSessions: true})
	h.addPlayer("alice", false)
	tab1, tab2 := h.connect(), h.connect()
	tab1.send("Login alice password")
	tab1.drainUntil("OnlinePlayers")
	tab2.send("Login alice password")
	tab2.drainUntil("OnlinePlayers")

	tab1.send("Seek 5 600 30")
	tab2.drainUntil("Seek new 1 alice")
	tab2.send("Seek 6 600 30")
	tab2.expect("Seek new 2 alice 6 600 30 A 0 30 1 0 0 0 0 ") // no remove of seek 1: each tab has its own
	tab1.drainUntil("Seek new 2 alice")

	tab1.send("quit")
	tab1.expectClosed(transport.CloseNormal)
	tab2.expect("Seek remove 1 alice 5 600 30 A 0 21 1 0 0 0 0 ")
	tab2.expect("Online 1")
}

func TestSeekOpeningNeedsProtocol4(t *testing.T) {
	h := newHarness(t, Config{})
	old, modern := h.connect(), h.connect()
	old.send("Login Guest")
	old.drainUntil("OnlinePlayers")
	modern.send("Protocol 4")
	modern.expect("OK")
	modern.send("Login Guest")
	modern.drainUntil("OnlinePlayers")
	old.drainUntil("OnlinePlayers")

	// An old client can't post a double-black-stack seek...
	old.send("Seek 5 600 30 0 A 0 21 1 0 0 0 0 1 ")
	old.expect("NOK")
	// ...and isn't shown one.
	modern.send("Seek 5 600 30 0 A 0 21 1 0 0 0 0 1 ")
	modern.expect("Seek new 1 Guest2 5 600 30 0 A 0 21 1 0 0 0 0 0 0 1")
	old.send("List")
	old.expectNothing()
}

func TestAcceptNotYetSupported(t *testing.T) {
	h := newHarness(t, Config{})
	c := h.connect()
	c.send("Login Guest")
	c.drainUntil("OnlinePlayers")
	c.send("Accept 1")
	c.expect("NOK")
}

func TestRematchFlow(t *testing.T) {
	h := newHarness(t, Config{})
	h.addPlayer("alice", false)
	h.addPlayer("bobby", false)
	a, b := h.connect(), h.connect()
	a.send("Login alice password")
	a.drainUntil("OnlinePlayers")
	b.send("Login bobby password")
	b.drainUntil("OnlinePlayers")
	a.drainUntil("OnlinePlayers")

	a.send("Rematch 42 5 600 30 A 0 21 1 0 0 0 0 bobby")
	a.expect("Seek new 1 alice 5 600 30 A 0 21 1 0 0 0 0 bobby")
	a.expect("Rematch seek created with ID: 1")
	b.expect("Seek new 1 alice 5 600 30 A 0 21 1 0 0 0 0 bobby")

	b.send("Rematch 42 5 600 30 A 0 21 1 0 0 0 0 alice")
	b.expect("Accept Rematch 1")
}
