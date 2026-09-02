package suite_test

import (
	"fmt"
	"strings"
	"testing"

	"tak-tests/internal/client"
)

// WebSocket tests verify that the WS transport delivers the same
// protocol behaviour as Telnet. Every Telnet test has a WS equivalent
// here for the most critical flows.

func TestWSWelcomeMessage(t *testing.T) {
	c := client.NewWS(t, wsAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
}

func TestWSGuestLogin(t *testing.T) {
	c := client.NewWS(t, wsAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send("Login Guest")
	welcome := c.ExpectPrefix("Welcome Guest")
	if !strings.HasSuffix(welcome, "!") {
		t.Errorf("welcome message should end with !: %q", welcome)
	}
}

func TestWSNamedLogin(t *testing.T) {
	u, p := testUser(t, 1)
	c := client.NewWS(t, wsAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send(fmt.Sprintf("Login %s %s", u, p))
	c.ExpectPrefix("Welcome " + u + "!")
}

func TestWSPing(t *testing.T) {
	c := client.NewWS(t, wsAddr(t))
	client.LoginGuestWS(t, c)
	c.DrainUntil("OnlinePlayers")
	c.Send("PING")
	c.Expect("OK")
}

func TestWSProtocolVersion(t *testing.T) {
	c := client.NewWS(t, wsAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send("Protocol 4")
	c.Expect("OK")
}

func TestWSUnknownCommand(t *testing.T) {
	c := client.NewWS(t, wsAddr(t))
	client.LoginGuestWS(t, c)
	c.DrainUntil("OnlinePlayers")
	c.Send("nonsense xyz")
	c.Expect("NOK")
}

func TestWSSeekAndAccept(t *testing.T) {
	c1 := client.NewWS(t, wsAddr(t))
	c2 := client.NewWS(t, wsAddr(t))
	client.LoginGuestWS(t, c1)
	client.LoginGuestWS(t, c2)

	pieces, caps := defaultPieces(5)
	c1.Send(fmt.Sprintf("Seek 5 600 30 A 0 %d %d 0 0 ", pieces, caps))
	seekMsg := c1.DrainUntil("Seek new ")

	var seekID int
	fmt.Sscanf(strings.TrimPrefix(seekMsg, "Seek new "), "%d", &seekID)

	c2.Send(fmt.Sprintf("Accept %d", seekID))
	gameId := c1.DrainUntil("Game Start ")
	c2.DrainUntil("Game Start ")
	client.CleanupActiveGamesWS(t, c1, parseGameID(t, gameId))
}

func TestWSPlaceMove(t *testing.T) {
	c1 := client.NewWS(t, wsAddr(t))
	c2 := client.NewWS(t, wsAddr(t))
	client.LoginGuestWS(t, c1)
	client.LoginGuestWS(t, c2)
	c1.DrainUntil("OnlinePlayers")
	c2.DrainUntil("OnlinePlayers")
	pieces, caps := defaultPieces(5)
	c1.Send(fmt.Sprintf("Seek 5 600 30 W 0 %d %d 0 0 ", pieces, caps))
	seekMsg := c1.DrainUntil("Seek new ")
	var seekID int
	fmt.Sscanf(strings.TrimPrefix(seekMsg, "Seek new "), "%d", &seekID)

	c2.Send(fmt.Sprintf("Accept %d", seekID))
	gs1 := c1.DrainUntil("Game Start ")
	c2.DrainUntil("Game Start ")

	var gameID int
	fmt.Sscanf(strings.TrimPrefix(gs1, "Game Start "), "%d", &gameID)

	c1.Send(fmt.Sprintf("Game#%d P A1", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK")
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.Expect(fmt.Sprintf("Game#%d P A1", gameID))
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGamesWS(t, c1, gameID)
}

func TestWSAndTelnetInteroperate(t *testing.T) {
	// A WS client and a Telnet client can play the same game.
	telnet := client.New(t, telnetAddr(t))
	ws := client.NewWS(t, wsAddr(t))

	client.LoginGuest(t, telnet)
	client.LoginGuestWS(t, ws)

	pieces, caps := defaultPieces(5)
	telnet.Send(fmt.Sprintf("Seek 5 600 30 W 0 %d %d 0 0 ", pieces, caps))
	seekMsg := telnet.DrainUntil("Seek new ")
	var seekID int
	fmt.Sscanf(strings.TrimPrefix(seekMsg, "Seek new "), "%d", &seekID)

	ws.Send(fmt.Sprintf("Accept %d", seekID))
	gs := telnet.DrainUntil("Game Start ")
	ws.DrainUntil("Game Start ")

	var gameID int
	fmt.Sscanf(strings.TrimPrefix(gs, "Game Start "), "%d", &gameID)

	// Telnet player places first move
	telnet.Send(fmt.Sprintf("Game#%d P A1", gameID))
	telnet.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	telnet.Expect("OK")
	// WS player receives it
	ws.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	ws.Expect(fmt.Sprintf("Game#%d P A1", gameID))
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGamesWS(t, ws, gameID)
}

func TestWSResign(t *testing.T) {
	c1 := client.NewWS(t, wsAddr(t))
	c2 := client.NewWS(t, wsAddr(t))
	client.LoginGuestWS(t, c1)
	client.LoginGuestWS(t, c2)

	pieces, caps := defaultPieces(5)
	c1.Send(fmt.Sprintf("Seek 5 600 30 W 0 %d %d 0 0 ", pieces, caps))
	seekMsg := c1.DrainUntil("Seek new ")
	var seekID int
	fmt.Sscanf(strings.TrimPrefix(seekMsg, "Seek new "), "%d", &seekID)

	c2.Send(fmt.Sprintf("Accept %d", seekID))
	gs1 := c1.DrainUntil("Game Start ")
	c2.DrainUntil("Game Start ")
	var gameID int
	fmt.Sscanf(strings.TrimPrefix(gs1, "Game Start "), "%d", &gameID)

	// Opening swap
	c1.Send(fmt.Sprintf("Game#%d P A1", gameID));
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK");
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.Send(fmt.Sprintf("Game#%d P E5", gameID));
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))

	c2.Expect("OK");
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))

	c1.Send(fmt.Sprintf("Game#%d Resign", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d Over", gameID))
	c2.DrainUntil(fmt.Sprintf("Game#%d Over", gameID))
}

func TestWSShout(t *testing.T) {
	c1 := client.NewWS(t, wsAddr(t))
	c2 := client.NewWS(t, wsAddr(t))
	name1 := client.LoginGuestWS(t, c1)
	client.LoginGuestWS(t, c2)

	c1.Send("Shout hello from websocket")
	c1.DrainUntil(fmt.Sprintf("Shout <%s> hello from websocket", name1))
	c2.DrainUntil(fmt.Sprintf("Shout <%s> hello from websocket", name1))
}

func TestWSProtocol4GameListFormat(t *testing.T) {
	// Verify protocol 4 over WS produces the correct GameList field count
	c1 := client.NewWS(t, wsAddr(t))
	c2 := client.NewWS(t, wsAddr(t))
	client.SetProtocolWS(t, c1, 4)
	client.LoginGuestAfterProtocolWS(t, c1)
	client.SetProtocolWS(t, c2, 4)
	client.LoginGuestAfterProtocolWS(t, c2)
	pieces, caps := defaultPieces(5)
	c1.Send(fmt.Sprintf("Seek 5 600 30 0 A 0 %d %d 0 0 0 0 0 ", pieces, caps))
	seekMsg := c1.DrainUntil("Seek new ")
	var seekID int
	fmt.Sscanf(strings.TrimPrefix(seekMsg, "Seek new "), "%d", &seekID)

	c2.Send(fmt.Sprintf("Accept %d", seekID))
	gs := c1.DrainUntil("Game Start ")
	c2.DrainUntil("Game Start ")
	var gameID int
	fmt.Sscanf(strings.TrimPrefix(gs, "Game Start "), "%d", &gameID)

	observer := client.NewWS(t, wsAddr(t))
	client.SetProtocolWS(t, observer, 4)
	client.LoginGuestAfterProtocolWS(t, observer)
	observer.Send("GameList")
	msg := observer.DrainUntil(fmt.Sprintf("GameList Add %d ", gameID))

	parts := strings.Fields(msg)
	if len(parts) != 17 {
		t.Errorf("protocol 4 WS GameList Add should have 17 fields, got %d: %q", len(parts), msg)
	}
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGamesWS(t, c1, gameID)
}
