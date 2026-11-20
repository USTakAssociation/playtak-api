package session

import (
	"fmt"
	"testing"
)

// The text protocol shares auth's limits; old clients get the replies they
// already understand.

func TestSocketLoginRateLimited(t *testing.T) {
	h := newHarness(t, Config{})
	h.m.auth.EnableLimits(true)
	h.addPlayer("alice", false)
	for range 10 {
		c := h.connect()
		c.send("Login alice wrongpass")
		c.expect("Authentication failure")
	}
	c := h.connect()
	c.send("Login alice password") // right password, but the name is locked for this IP
	c.expect("Authentication failure")
	c.expectNothing()
}

func TestSocketRegisterRateLimited(t *testing.T) {
	h := newHarness(t, Config{})
	h.m.auth.EnableLimits(true)
	c := h.connect()
	for i := range 5 {
		c.send(fmt.Sprintf("Register newbie%d n%d@example.com", i, i))
		c.drainUntil("Registered")
	}
	c.send("Register another1 z@example.com")
	c.expect("Registration Error: Too many registrations, please try again later")
}

func TestSocketResetTokenRateLimited(t *testing.T) {
	h := newHarness(t, Config{})
	h.m.auth.EnableLimits(true)
	h.addPlayer("alice", false)
	c := h.connect()
	for range 3 {
		c.send("SendResetToken alice alice@example.com")
		c.expect("Reset token sent")
	}
	c.send("SendResetToken alice alice@example.com")
	c.expect("Reset Token Error: Too many requests, please try again later")
}
