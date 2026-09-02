package suite_test

import (
	"testing"
	"time"

	"tak-tests/internal/client"
)

func TestConnectionWelcomeMessage(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
}

func TestConnectionWelcomeMessageOrdering(t *testing.T) {
	// "Welcome!" must come before "Login or Register" — order matters for clients
	c := client.New(t, telnetAddr(t))
	first := c.Recv()
	if first != "Welcome!" {
		t.Errorf("first message must be %q, got %q", "Welcome!", first)
	}
	second := c.Recv()
	if second != "Login or Register" {
		t.Errorf("second message must be %q, got %q", "Login or Register", second)
	}
}

func TestConnectionQuitBeforeLogin(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send("quit")
	c.ExpectNoMessage(time.Second)
}

func TestConnectionQuitAfterLogin(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)
	c.DrainUntil("OnlinePlayers ")
	c.Send("quit")
	c.ExpectNoMessage(time.Second)
}

func TestConnectionUnknownCommandPreAuth(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send("nonsense command xyz")
	c.Expect("NOK")
}

func TestConnectionUnknownCommandPostAuth(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)
	c.DrainUntil("OnlinePlayers ")
	c.Send("nonsense command xyz")
	c.Expect("NOK")
}

func TestConnectionClientNameOK(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send("Client TestSuite-1.0.0")
	c.Expect("OK")
}

func TestConnectionClientNameBeforeLogin(t *testing.T) {
	// Client name can be sent before login and should not trigger authentication
	c := client.New(t, telnetAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send("Client MyClient-2.0")
	c.Expect("OK")
	// Server should still be waiting for login
	c.Send("Login Guest")
	c.ExpectPrefix("Welcome Guest")
}

func TestConnectionPingPreAuth(t *testing.T) {
	// PING before login should still respond with OK
	c := client.New(t, telnetAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send("PING")
	c.Expect("OK")
}

func TestConnectionPingPostAuth(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)
	c.DrainUntil("OnlinePlayers ")
	c.Send("PING")
	c.Expect("OK")
}

func TestConnectionMultiplePings(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)
	c.DrainUntil("OnlinePlayers ")
	for i := 0; i < 5; i++ {
		c.Send("PING")
		c.Expect("OK")
	}
}

func TestConnectionOnlineCountOnConnect(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)
	// Online message is sent after login — we drained it in LoginGuest,
	// but a second client connecting should trigger an Online update to all
	c2 := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c2)
	// c should have received an Online update when c2 logged in
	c.DrainUntil("Online ")
}

func TestConnectionOnlinePlayers(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)
	// A second login triggers an OnlinePlayers broadcast
	c2 := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c2)
	c.DrainUntil("OnlinePlayers ")
}
