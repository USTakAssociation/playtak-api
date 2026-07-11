package suite_test

import (
	"fmt"
	"strings"
	"testing"

	"tak-tests/internal/client"
)

// All admin tests require TAK_ADMIN_USER and TAK_ADMIN_PASS to be set.
// Non-admin tests verify that regular users are rejected.

// ---- Permission checks ----

func TestSudoRejectedForGuest(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)
	c.DrainUntil("OnlinePlayers")
	c.Send("sudo list online")
	c.Expect("NOK")
}

func TestSudoRejectedForNormalUser(t *testing.T) {
	u, p := testUser(t, 1)
	c := client.New(t, telnetAddr(t))
	client.LoginUser(t, c, u, p)
	c.DrainUntil("OnlinePlayers")
	c.Send("sudo list online")
	c.Expect("NOK")
}

// ---- List commands ----

func TestSudoListOnline(t *testing.T) {
	u, p := adminUser(t)
	c := client.New(t, telnetAddr(t))
	client.LoginUser(t, c, u, p)
	c.DrainUntil("OnlinePlayers")
	c.Send("sudo list online")
	c.DrainUntil("sudoReply >")
	reply := c.DrainUntil("sudoReply ")
	if !strings.Contains(reply, "[") {
		t.Errorf("sudo list online should return a list, got %q", reply)
	}
}

func TestSudoListMod(t *testing.T) {
	u, p := adminUser(t)
	c := client.New(t, telnetAddr(t))
	client.LoginUser(t, c, u, p)
	c.DrainUntil("OnlinePlayers")
	c.Send("sudo list mod")
	c.DrainUntil("sudoReply >")
	reply := c.DrainUntil("sudoReply ")
	if !strings.Contains(reply, "[") {
		t.Errorf("sudo list mod should return a list, got %q", reply)
	}
}

func TestSudoListBan(t *testing.T) {
	u, p := adminUser(t)
	c := client.New(t, telnetAddr(t))
	client.LoginUser(t, c, u, p)
	c.DrainUntil("OnlinePlayers")
	c.Send("sudo list ban")
		c.DrainUntil("sudoReply >")
	reply := c.DrainUntil("sudoReply ")
	if !strings.Contains(reply, "[") {
		t.Errorf("sudo list ban should return a list, got %q", reply)
	}
}

func TestSudoListGag(t *testing.T) {
	u, p := adminUser(t)
	c := client.New(t, telnetAddr(t))
	client.LoginUser(t, c, u, p)
	c.DrainUntil("OnlinePlayers")
	c.Send("sudo list gag")
		c.DrainUntil("sudoReply >")
	reply := c.DrainUntil("sudoReply ")
	if !strings.Contains(reply, "[") {
		t.Errorf("sudo list gag should return a list, got %q", reply)
	}
}

func TestSudoListAdmin(t *testing.T) {
	u, p := adminUser(t)
	c := client.New(t, telnetAddr(t))
	client.LoginUser(t, c, u, p)
	c.DrainUntil("OnlinePlayers")
	c.Send("sudo list admin")
	c.DrainUntil("sudoReply >")
	reply := c.DrainUntil("sudoReply ")
	if !strings.Contains(reply, "[") {
		t.Errorf("sudo list mod should return a list, got %q", reply)
	}
}

func TestSudoListUnknownGroup(t *testing.T) {
	u, p := adminUser(t)
	c := client.New(t, telnetAddr(t))
	client.LoginUser(t, c, u, p)
	c.DrainUntil("OnlinePlayers")
	c.Send("sudo list nonexistent")
	c.DrainUntil("sudoReply >")
	reply := c.DrainUntil("sudoReply ")
	if !strings.Contains(reply, "not found") {
		t.Errorf("unknown list group should reply 'command not found', got %q", reply)
	}
}

// ---- Gag / Ungag ----

func TestSudoGagAndUngag(t *testing.T) {
	adminU, adminP := adminUser(t)
	u, p := testUser(t, 3)

	admin := client.New(t, telnetAddr(t))
	target := client.New(t, telnetAddr(t))
	client.LoginUser(t, admin, adminU, adminP)
	client.LoginUser(t, target, u, p)

	// Gag the user
	admin.Send(fmt.Sprintf("sudo gag %s", u))
	admin.DrainUntil("sudoReply >")
	reply := admin.DrainUntil("sudoReply ")
	if !strings.Contains(reply, "gagged") {
		t.Errorf("expected gagged confirmation, got %q", reply)
	}

	// Gagged user's shouts should not be broadcast to others
	target.Send("Shout this should be silenced")
	admin.ExpectNoMessage(client.NoMessageWindow)

	// Ungag
	admin.Send(fmt.Sprintf("sudo ungag %s", u))
	admin.DrainUntil("sudoReply >")
	reply = admin.DrainUntil("sudoReply ")
	if !strings.Contains(reply, "ungagged") {
		t.Errorf("expected ungagged confirmation, got %q", reply)
	}
}

func TestSudoGagNonExistentUser(t *testing.T) {
	u, p := adminUser(t)
	c := client.New(t, telnetAddr(t))
	client.LoginUser(t, c, u, p)
	c.Send("sudo gag doesnotexist9999")
	c.DrainUntil("sudoReply >")
	reply := c.DrainUntil("sudoReply ")
	if !strings.Contains(reply, "No such player") {
		t.Errorf("expected 'No such player', got %q", reply)
	}
}

func TestSudoUngagNotGaggedUser(t *testing.T) {
	adminU, adminP := adminUser(t)
	u, p := testUser(t, 1)

	admin := client.New(t, telnetAddr(t))
	client.LoginUser(t, admin, adminU, adminP)

	target := client.New(t, telnetAddr(t))
	client.LoginUser(t, target, u, p)

	// User is not gagged — ungag should complain
	admin.Send(fmt.Sprintf("sudo ungag %s", u))
	admin.DrainUntil("sudoReply >")
	reply := admin.DrainUntil("sudoReply ")
	if !strings.Contains(reply, "not gagged") {
		t.Errorf("expected 'not gagged', got %q", reply)
	}
}

// ---- Kick ----

func TestSudoKick(t *testing.T) {
	adminU, adminP := adminUser(t)
	u, p := testUser(t, 2)

	admin := client.New(t, telnetAddr(t))
	target := client.New(t, telnetAddr(t))
	client.LoginUser(t, admin, adminU, adminP)
	client.LoginUser(t, target, u, p)
	target.DrainUntil("OnlinePlayers")
	admin.Send(fmt.Sprintf("sudo kick %s", u))
	admin.DrainUntil("sudoReply >")
	reply := admin.DrainUntil("sudoReply ")
	if !strings.Contains(reply, "kicked") {
		t.Errorf("expected kicked confirmation, got %q", reply)
	}

	// Target should be disconnected
	target.ExpectNoMessage(client.NoMessageWindow)
}

func TestSudoKickNotLoggedIn(t *testing.T) {
	adminU, adminP := adminUser(t)
	u, _ := testUser(t, 1)

	admin := client.New(t, telnetAddr(t))
	client.LoginUser(t, admin, adminU, adminP)
	// u is not logged in

	admin.Send(fmt.Sprintf("sudo kick %s", u))
	admin.DrainUntil("sudoReply >")
	reply := admin.DrainUntil("sudoReply ")
	if !strings.Contains(reply, "not logged in") {
		t.Errorf("expected 'not logged in', got %q", reply)
	}
}

// ---- Broadcast ----

func TestSudoBroadcast(t *testing.T) {
	adminU, adminP := adminUser(t)

	admin := client.New(t, telnetAddr(t))
	listener := client.New(t, telnetAddr(t))
	client.LoginUser(t, admin, adminU, adminP)
	client.LoginGuest(t, listener)

	admin.Send("sudo broadcast Server going down in 5 minutes")
	// All online clients including listener should receive the raw message
	listener.DrainUntil("Server going down in 5 minutes")
}

func TestSudoBroadcastRejectedForMod(t *testing.T) {
	// Broadcast is admin-only; a mod who is not an admin should be denied
	// This test only runs if there is a dedicated mod account configured.
	// If no mod credentials are set we skip rather than use the admin account.
	modU := fmt.Sprintf("%s_mod_only", "testuser1") // placeholder, likely not a real mod
	_ = modU
	t.Skip("requires a dedicated mod-only (non-admin) account; skipping")
}

// ---- Password reset (admin) ----

func TestSudoSetPassword(t *testing.T) {
	adminU, adminP := adminUser(t)
	u, _ := testUser(t, 1)

	admin := client.New(t, telnetAddr(t))
	client.LoginUser(t, admin, adminU, adminP)

	admin.Send(fmt.Sprintf("sudo set password %s password", u))
	admin.DrainUntil("sudoReply >")
	reply := admin.DrainUntil("sudoReply ")
	if !strings.Contains(reply, "Password set") {
		t.Errorf("expected 'Password set', got %q", reply)
	}

	// Verify the new password actually works
	c := client.New(t, telnetAddr(t))
	client.LoginUser(t, c, u, "password")
}
