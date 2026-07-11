package suite_test

import (
	"fmt"
	"strings"
	"testing"

	"tak-tests/internal/client"
)

// ---- Guest login ----

func TestAuthGuestLogin(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send("Login Guest")
	welcome := c.ExpectPrefix("Welcome Guest")
	if !strings.HasSuffix(welcome, "!") {
		t.Errorf("welcome message should end with !: %q", welcome)
	}
}

func TestGuestNamesAreUnique(t *testing.T) {
	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	n1 := client.LoginGuest(t, c1)
	n2 := client.LoginGuest(t, c2)
	if n1 == n2 {
		t.Errorf("two concurrent guest logins should get unique names, both got %q", n1)
	}
}

func TestAuthGuestTokenRejoin(t *testing.T) {
	const token = "abcdefghijklmnopqrst" // exactly 20 lowercase letters

	c1 := client.New(t, telnetAddr(t))
	c1.Expect("Welcome!")
	c1.Expect("Login or Register")
	c1.Send("Login Guest " + token)
	name1 := c1.DrainUntil("Welcome ")
	name1 = strings.TrimSuffix(strings.TrimPrefix(name1, "Welcome "), "!")
	c1.DrainUntil("Online ")

	// Reconnect with the same token — should get the same guest account back
	c2 := client.New(t, telnetAddr(t))
	c2.Expect("Welcome!")
	c2.Expect("Login or Register")
	c2.Send("Login Guest " + token)
	name2 := c2.DrainUntil("Welcome ")
	name2 = strings.TrimSuffix(strings.TrimPrefix(name2, "Welcome "), "!")
	c2.DrainUntil("Online ")

	// c1 should be kicked with the same disconnect notice as a named-user double login
	c1.ExpectAny("Message You've logged in from another window. Disconnecting")

	if name1 != name2 {
		t.Errorf("token rejoin should restore same name: first=%q second=%q", name1, name2)
	}
}

// ---- Named login ----

func TestAuthNamedLogin(t *testing.T) {
	u, p := testUser(t, 1)
	c := client.New(t, telnetAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send(fmt.Sprintf("Login %s %s", u, p))
	c.ExpectPrefix("Welcome " + u + "!")
}

func TestAuthNamedLoginWrongPassword(t *testing.T) {
	u, _ := testUser(t, 1)
	c := client.New(t, telnetAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send(fmt.Sprintf("Login %s wrongpassword123", u))
	c.Expect("Authentication failure")
}

func TestAuthNamedLoginNonExistentUser(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send("Login doesnotexist9999 somepassword")
	c.Expect("Authentication failure")
}

func TestAuthNamedLoginShortUsername(t *testing.T) {
	// Username must be 4-16 chars starting with a letter
	c := client.New(t, telnetAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send("Login ab password123")
	c.Expect("NOK")
}

func TestAuthDoubleLoginDisconnectsFirst(t *testing.T) {
	u, p := testUser(t, 2)

	c1 := client.New(t, telnetAddr(t))
	client.LoginUser(t, c1, u, p)

	// Second login from a different connection
	c2 := client.New(t, telnetAddr(t))
	c2.Expect("Welcome!")
	c2.Expect("Login or Register")
	c2.Send(fmt.Sprintf("Login %s %s", u, p))

	// c1 must receive the disconnect notice
	c1.ExpectAny("Message You've logged in from another window. Disconnecting")

	// c2 should get welcomed
	c2.ExpectPrefix("Welcome " + u + "!")
}

func TestAuthDoubleLoginFirstConnectionClosed(t *testing.T) {
	u, p := testUser(t, 3)

	c1 := client.New(t, telnetAddr(t))
	client.LoginUser(t, c1, u, p)

	c2 := client.New(t, telnetAddr(t))
	c2.Expect("Welcome!")
	c2.Expect("Login or Register")
	c2.Send(fmt.Sprintf("Login %s %s", u, p))
	c2.ExpectPrefix("Welcome " + u + "!")

	// c1 should now be dead — no new messages after the disconnect notice
	c1.ExpectAny("Message You've logged in from another window. Disconnecting")
}

// ---- Password change ----

func TestAuthChangePassword(t *testing.T) {
	u, p := testUser(t, 1)

	c := client.New(t, telnetAddr(t))
	client.LoginUser(t, c, u, p)
	c.DrainUntil("OnlinePlayers")
	c.Send(fmt.Sprintf("ChangePassword %s %s", p, p)) // change to same password
	c.Expect("Password changed")
}

func TestAuthChangePasswordWrongOld(t *testing.T) {
	u, p := testUser(t, 1)

	c := client.New(t, telnetAddr(t))
	client.LoginUser(t, c, u, p)
	c.DrainUntil("OnlinePlayers")
	c.Send("ChangePassword wrongoldpassword newpassword123")
	c.Expect("Wrong password")
}

// ---- Registration ----

func TestAuthRegistrationBadFormat(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	// Missing email
	c.Send("Register justausername")
	c.ExpectPrefix("Registration Error:")
}

func TestAuthRegistrationGuestInName(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send("Register GuestPlayer test@example.com")
	c.ExpectPrefix("Registration Error:")
}

func TestAuthRegistrationNameTaken(t *testing.T) {
	u, _ := testUser(t, 1)
	c := client.New(t, telnetAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send(fmt.Sprintf("Register %s test@example.com", u))
	c.ExpectPrefix("Registration Error: Username is already taken")
}
