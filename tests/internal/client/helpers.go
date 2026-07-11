package client

import (
	"fmt"
	"strings"
	"testing"
)

// LoginGuest performs a full guest login handshake over a telnet Client.
// Reads "Welcome!" and "Login or Register", sends "Login Guest",
// reads the "Welcome <name>!" response, then drains until the "Online" message.
// Returns the guest name assigned by the server.
func LoginGuest(t *testing.T, c *Client) string {
	t.Helper()
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send("Login Guest")
	welcome := c.ExpectPrefix("Welcome ")
	name := strings.TrimSuffix(strings.TrimPrefix(welcome, "Welcome "), "!")
	c.DrainUntil("Online ")
	c.Name = name
	return name
}

// LoginGuestAfterProtocol logs in as a guest assuming the Protocol handshake
// has already been completed (i.e. SetProtocol was called first, which already
// consumed "Welcome!" and "Login or Register").
func LoginGuestAfterProtocol(t *testing.T, c *Client) string {
	t.Helper()
	c.Send("Login Guest")
	welcome := c.ExpectPrefix("Welcome ")
	name := strings.TrimSuffix(strings.TrimPrefix(welcome, "Welcome "), "!")
	c.DrainUntil("Online ")
	return name
}

func LoginGuestAfterProtocolWS(t *testing.T, c *WSClient) string {
	t.Helper()
	c.Send("Login Guest")
	welcome := c.ExpectPrefix("Welcome ")
	name := strings.TrimSuffix(strings.TrimPrefix(welcome, "Welcome "), "!")
	c.DrainUntil("Online ")
	return name
}

// LoginGuestWS is the WebSocket variant of LoginGuest.
func LoginGuestWS(t *testing.T, c *WSClient) string {
	t.Helper()
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send("Login Guest")
	welcome := c.ExpectPrefix("Welcome ")
	name := strings.TrimSuffix(strings.TrimPrefix(welcome, "Welcome "), "!")
	c.DrainUntil("Online ")
	return name
}

// LoginUser performs a full named-user login handshake over a telnet Client.
// Drains until the "Online" message so the caller starts with a clean slate.
func LoginUser(t *testing.T, c *Client, username, password string) {
	t.Helper()
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send(fmt.Sprintf("Login %s %s", username, password))
	c.ExpectPrefix("Welcome " + username)
	//c.DrainUntil("Online ")
}

// LoginUserWS is the WebSocket variant of LoginUser.
func LoginUserWS(t *testing.T, c *WSClient, username, password string) {
	t.Helper()
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send(fmt.Sprintf("Login %s %s", username, password))
	c.ExpectPrefix("Welcome " + username)
	c.DrainUntil("Online ")
}

// SetProtocol reads the server handshake ("Welcome!" + "Login or Register"),
// sends "Protocol <version>", and expects "OK".
// Call this instead of LoginGuest/LoginUser when you need a specific protocol
// version — it replaces the handshake read that those helpers would otherwise do.
// After SetProtocol, call LoginGuest or LoginUser normally.
func SetProtocol(t *testing.T, c *Client, version int) {
	t.Helper()
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send(fmt.Sprintf("Protocol %d", version))
	c.Expect("OK")
}

// SetProtocolWS is the WebSocket variant of SetProtocol.
func SetProtocolWS(t *testing.T, c *WSClient, version int) {
	t.Helper()
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send(fmt.Sprintf("Protocol %d", version))
	c.Expect("OK")
}

// clean up active games for a given client, to avoid leaving them in the server's active game list after the test ends.
func CleanupActiveGames(t *testing.T, c *Client, gameID int) {
	c.Send(fmt.Sprintf("Game#%d Resign", gameID))
	c.ExpectAnyPrefix(fmt.Sprintf("Game#%d Over", gameID))
}

func CleanupActiveGamesWS(t *testing.T, c *WSClient, gameID int) {
	c.Send(fmt.Sprintf("Game#%d Resign", gameID))
	c.ExpectAnyPrefix(fmt.Sprintf("Game#%d Over", gameID))
}
