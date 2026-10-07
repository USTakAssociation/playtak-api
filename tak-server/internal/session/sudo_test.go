package session

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport"
)

func (h *harness) addAdmin(name string) {
	h.t.Helper()
	h.addPlayer(name, false)
	h.store.SetFlag(context.Background(), name, players.FlagAdmin, true)
}

func TestSudoNeedsModOrAdmin(t *testing.T) {
	h := newHarness(t, Config{})
	h.addPlayer("patty", false)
	g := loginGuestWith(h, "")
	g.send("sudo list online")
	g.expect("NOK")
	p := loginAs(h, "patty")
	p.send("sudo list online")
	p.expect("NOK")
}

func TestSudoEchoAndReply(t *testing.T) {
	h := newHarness(t, Config{})
	h.addPlayer("mona", true)
	h.addPlayer("patty", false)
	m := loginAs(h, "mona")
	m.send("sudo gag patty")
	m.expect("sudoReply > sudo gag patty")
	m.expect("sudoReply patty gagged")
	m.send("sudo gagg patty") // typo
	m.expect("sudoReply > sudo gagg patty")
	m.expect("sudoReply Command not found")
	m.send("sudo broadcast hi")
	m.expect("sudoReply > sudo broadcast hi")
	m.expect("sudoReply Permission denied")
	m.send("sudo list gag")
	m.expect("sudoReply > sudo list gag")
	m.expect("sudoReply [patty]")
}

func TestSudoSetPasswordIsHidden(t *testing.T) {
	h := newHarness(t, Config{})
	h.addAdmin("adele")
	h.addPlayer("patty", false)
	a := loginAs(h, "adele")
	a.send("sudo set password patty n3wpassword")
	a.expect("sudoReply > sudo set password patty ******")
	a.expect("sudoReply Password set")
	p := h.connect()
	p.send("Login patty n3wpassword")
	p.expect("Welcome patty!")
}

func TestSudoBroadcastIsARawLine(t *testing.T) {
	h := newHarness(t, Config{})
	h.addAdmin("adele")
	g := loginGuestWith(h, "")
	a := loginAs(h, "adele")
	g.settle()
	a.send("sudo broadcast Message Server going down in 5 minutes")
	a.expect("sudoReply > sudo broadcast Message Server going down in 5 minutes")
	a.expect("Message Server going down in 5 minutes")
	g.expect("Message Server going down in 5 minutes")
	a.send("sudo broadcast Shout <Server> hello everyone")
	a.drainUntil("Shout <Server> hello everyone")
	g.expect("Shout <Server> hello everyone")
}

func TestSudoKickThroughProtocol(t *testing.T) {
	h := newHarness(t, Config{})
	h.addPlayer("mona", true)
	h.addPlayer("patty", false)
	p := loginAs(h, "patty")
	ps := p.conn.peer.(*Session)
	m := loginAs(h, "mona")
	m.send("sudo kick PATTY")
	m.expect("sudoReply > sudo kick PATTY")
	m.expect("sudoReply patty kicked")
	p.expectClosed(transport.CloseKicked)
	// Wait for patty's session to finish cleaning up, so they're offline.
	<-ps.Done()
	m.send("sudo kick patty")
	m.drainUntil("sudoReply Player not logged in")
}

func TestSudoBanEndsGameAndKicks(t *testing.T) {
	h := newHarness(t, Config{})
	h.addPlayer("mona", true)
	h.addPlayer("alice", false)
	h.addPlayer("bobby", false)
	a, b := loginAs(h, "alice"), loginAs(h, "bobby")
	a.drainUntil("OnlinePlayers")
	a.send("Seek 5 600 30 W 0 21 1 0 0 ")
	b.drainUntil("Seek new")
	b.send("Accept 1")
	a.drainUntil("Game Start")
	b.drainUntil("Game Start")
	m := loginAs(h, "mona")

	m.send("sudo ban bobby abusive chat")
	m.expect("sudoReply > sudo ban bobby abusive chat")
	m.drainUntil("sudoReply bobby kicked") // after the game list update
	m.expect("sudoReply bobby banned")
	// The game ends at once as a loss for bobby, with no reconnect window.
	a.drainUntil("Game#1 Over 1-0")
	b.drainUntil("Game#1 Over 1-0")
	b.expectClosed(transport.CloseKicked)
	waitForResult(t, h, 1, "1-0")

	// The email goes out in the background.
	deadline := time.Now().Add(2 * time.Second)
	for len(h.mail.all()) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := h.mail.all(); len(got) != 1 || !strings.Contains(got[0], "Reason: abusive chat") {
		t.Errorf("mail %q", got)
	}
	// And bobby can't log back in.
	b2 := h.connect()
	b2.send("Login bobby password")
	b2.expect("Authentication failure")
}
