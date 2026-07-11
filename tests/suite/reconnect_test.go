package suite_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"tak-tests/internal/client"
)

// reconnectWindow is a short wait used to simulate a brief disconnection.
// Must be less than the server's reconnection grace period (default 30s).
const reconnectWindow = 2 * time.Second

func TestReconnectDisconnectMessageSentToOpponent(t *testing.T) {
	u1, p1 := testUser(t, 1)
	u2, p2 := testUser(t, 2)

	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	client.LoginUser(t, c1, u1, p1)
	client.LoginUser(t, c2, u2, p2)

	seekID := postSeek(t, c1, 5, 600, 30)
	c2.Send(fmt.Sprintf("Accept %d", seekID))
	gameId := c1.DrainUntil("Game Start ")
	c2.DrainUntil("Game Start ")

	// Disconnect c1 abruptly
	c1.Close()

	// c2 should receive a disconnect notification
	msg := c2.DrainUntil("Message ")
	if !strings.Contains(msg, u1) || !strings.Contains(msg, "disconnected") {
		t.Errorf("opponent disconnect message should mention %q and 'disconnected', got %q", u1, msg)
	}
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c2, parseGameID(t, gameId))
}

func TestReconnectRejoinRestoresGameState(t *testing.T) {
	u1, p1 := testUser(t, 1)
	u2, p2 := testUser(t, 2)
	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	client.LoginUser(t, c1, u1, p1)
	client.LoginUser(t, c2, u2, p2)
	seekID := postSeek(t, c1, 5, 600, 30)
	c2.Send(fmt.Sprintf("Accept %d", seekID))
	gs1 := c1.DrainUntil("Game Start ")
	c2.DrainUntil("Game Start ")
	gameID := parseGameID(t, gs1)
	makeOpeningMoves(t, c1, c2, gameID)

	// Play one more move so there's history to replay after reconnect
	c1.Send(fmt.Sprintf("Game#%d P A1", gameID))
	c1.DrainUntil("OK")
	c2.DrainUntil(fmt.Sprintf("Game#%d P A1", gameID))

	// c1 disconnects
	c1.Close()
	c2.DrainUntil("Message ")

	// Wait briefly then reconnect as the same user
	time.Sleep(reconnectWindow)
	c1New := client.New(t, telnetAddr(t))
	client.LoginUser(t, c1New, u1, p1)

	// Server should replay the Game Start
	c1New.DrainUntil("Game Start ")
	c1New.ExpectAny("Message Your game is resumed")
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1New, gameID)
}

func TestReconnectRejoinNotifiesOpponent(t *testing.T) {
	u1, p1 := testUser(t, 1)
	u2, p2 := testUser(t, 2)

	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	client.LoginUser(t, c1, u1, p1)
	client.LoginUser(t, c2, u2, p2)

	seekID := postSeek(t, c1, 5, 600, 30)
	c2.Send(fmt.Sprintf("Accept %d", seekID))
	c1.DrainUntil("Game Start ")
	c2.DrainUntil("Game Start ")

	c1.Close()
	c2.DrainUntil("Message ")

	time.Sleep(reconnectWindow)

	c1New := client.New(t, telnetAddr(t))
	client.LoginUser(t, c1New, u1, p1)
	gameId := c1New.DrainUntil("Game Start ")

	// c2 should be told that c1 has reconnected
	msg := c2.DrainUntil("Message ")
	if !strings.Contains(msg, u1) || !strings.Contains(msg, "reconnected") {
		t.Errorf("reconnect message to opponent should mention %q and 'reconnected', got %q", u1, msg)
	}
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1New, parseGameID(t, gameId))
}

func TestReconnectDisconnectAfterGracePeriodAbandonsTournamentGame(t *testing.T) {
	// Tournament games have a 15-minute (900s) abandonment window.
	// We can't wait that long in a test, but we can verify the initial
	// disconnect message mentions the correct time window.
	u1, p1 := testUser(t, 1)
	u2, p2 := testUser(t, 2)

	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	client.LoginUser(t, c1, u1, p1)
	client.LoginUser(t, c2, u2, p2)

	pieces, caps := defaultPieces(5)
	// tournament=1
	c1.Send(fmt.Sprintf("Seek 5 600 30 A 0 %d %d 0 1 ", pieces, caps))
	seekMsg := c1.DrainUntil("Seek new ")
	seekID := parseSeekID(t, seekMsg)

	c2.Send(fmt.Sprintf("Accept %d", seekID))
	gameId := c1.DrainUntil("Game Start ")
	c2.DrainUntil("Game Start ")

	c1.Close()

	// Disconnect notice should mention 900 seconds for tournament games
	msg := c2.DrainUntil("Message ")
	if !strings.Contains(msg, "900") {
		t.Errorf("tournament disconnect message should mention 900 seconds, got %q", msg)
	}
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c2, parseGameID(t, gameId))
}

func TestReconnectSeeksRemovedOnDisconnect(t *testing.T) {
	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c1)
	client.LoginGuest(t, c2)

	c1.Send("Seek 5 600 30")
	seekMsg := c1.DrainUntil("Seek new ")
	seekID := parseSeekID(t, seekMsg)

	// c2 should have seen the seek
	c2.DrainUntil("Seek new ")

	// c1 disconnects — their seek should be removed
	c1.Close()

	// c2 should receive Seek remove
	msg := c2.DrainUntil("Seek remove ")
	if !strings.Contains(msg, fmt.Sprintf("%d", seekID)) {
		t.Errorf("Seek remove should reference seek ID %d, got %q", seekID, msg)
	}
}
