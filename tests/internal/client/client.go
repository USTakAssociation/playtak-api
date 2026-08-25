package client

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const DefaultTimeout = 5 * time.Second
const NoMessageWindow = 500 * time.Millisecond

// Client is a raw TCP (telnet) connection to the Tak server.
// Each instance represents one independent connection.
type Client struct {
	t        *testing.T
	conn     net.Conn
	reader   *bufio.Reader
	mu       sync.Mutex
	received []string
	cId      int
	Name     string
}

var globalConnectionCounter uint64

// New dials the given address over TCP and returns a Client.
// The connection is automatically closed when the test ends via t.Cleanup.
func New(t *testing.T, addr string) *Client {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, DefaultTimeout)
	if err != nil {
		t.Fatalf("client: failed to connect to %s: %v", addr, err)
	}
	connectionID := atomic.AddUint64(&globalConnectionCounter, 1)
	c := &Client{
		t:      t,
		conn:   conn,
		reader: bufio.NewReader(conn),
		cId:    int(connectionID),
		Name:   "",
	}

	t.Cleanup(func() { c.Close() })
	return c
}

// Close closes the underlying TCP connection.
func (c *Client) Close() {
	c.conn.Close()
}

// Send writes a command to the server, appending a newline.
// The command is logged via t.Logf with a → prefix.
func (c *Client) Send(cmd string) {
	c.t.Helper()

	c.t.Logf("S → %s (TC #%d)", cmd, c.cId)
	if _, err := fmt.Fprintf(c.conn, "%s\n", cmd); err != nil {
		c.t.Fatalf("client: send failed: %v", err)
	}
}

// Recv reads the next newline-delimited message from the server.
// Blocks up to DefaultTimeout. The message is logged via t.Logf with a ← prefix.
func (c *Client) Recv() string {
	c.t.Helper()
	c.conn.SetReadDeadline(time.Now().Add(DefaultTimeout))
	line, err := c.reader.ReadString('\n')
	if err != nil {
		c.t.Fatalf("client %d: recv failed (did the server close the connection?): %v", c.cId, err)
	}
	line = strings.TrimRight(line, "\r\n")

	c.t.Logf("R ← %s (TC #%d)", line, c.cId)
	c.mu.Lock()
	c.received = append(c.received, line)
	c.mu.Unlock()
	return line
}

// Expect reads the next message and fails the test if it does not match exactly.
func (c *Client) Expect(want string) {
	c.t.Helper()
	got := c.Recv()
	if got != want {
		c.t.Errorf("client: Expect mismatch\n  want: %q\n   got: %q", want, got)
	}
}

// ExpectPrefix reads the next message and fails if it does not start with prefix.
// Returns the full received message.
func (c *Client) ExpectPrefix(prefix string) string {
	c.t.Helper()
	got := c.Recv()
	if !strings.HasPrefix(got, prefix) {
		c.t.Errorf("client: ExpectPrefix mismatch\n  want prefix: %q\n          got: %q", prefix, got)
	}
	return got
}

// ExpectAny reads messages until it finds one that matches exactly.
// Fails if DefaultTimeout is reached before a match.
func (c *Client) ExpectAny(want string) {
	c.t.Helper()
	deadline := time.Now().Add(DefaultTimeout)
	for time.Now().Before(deadline) {
		got := c.Recv()
		if got == want {
			return
		}
	}
	c.t.Errorf("client: never received %q within timeout", want)
}

// ExpectAnyPrefix reads messages until it finds one that starts with prefix.
// Fails if DefaultTimeout is reached before a match. Returns the matching message.
func (c *Client) ExpectAnyPrefix(prefix string) string {
	c.t.Helper()
	deadline := time.Now().Add(DefaultTimeout)
	for time.Now().Before(deadline) {
		got := c.Recv()
		if strings.HasPrefix(got, prefix) {
			return got
		}
	}
	c.t.Fatalf("client: never received prefix %q within timeout", prefix)
	return ""
}

// DrainUntil reads and discards messages until one starts with prefix.
// Returns the matching message. Fails with t.Fatalf on timeout.
func (c *Client) DrainUntil(prefix string) string {
	c.t.Helper()
	deadline := time.Now().Add(DefaultTimeout)
	for time.Now().Before(deadline) {
		got := c.Recv()
		// uncomment the following line to debug why a message didn't match the expected prefix
		// c.t.Logf("checking %q against prefix %q", got, prefix)
		if strings.HasPrefix(got, prefix) {
			return got
		}
	}
	c.t.Fatalf("client: timed out waiting for message with prefix %q", prefix)
	return ""
}

// ExpectNoMessage asserts the server sends nothing within the given window.
// A received message causes the test to fail.
func (c *Client) ExpectNoMessage(window time.Duration) {
	c.t.Helper()
	c.conn.SetReadDeadline(time.Now().Add(window))
	line, err := c.reader.ReadString('\n')
	if err == nil {
		c.t.Errorf("client: expected no message but received %q", strings.TrimRight(line, "\r\n"))
	}
}

// ExpectNoMessageWithPrefix reads all messages for the given window and fails
// if any of them start with prefix. Use this instead of ExpectNoMessage when
// the server may send unrelated messages (e.g. OnlinePlayers broadcasts) but
// must not send a specific type of message.
func (c *Client) ExpectNoMessageWithPrefix(prefix string, window time.Duration) {
	c.t.Helper()
	deadline := time.Now().Add(window)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return
		}
		c.conn.SetReadDeadline(time.Now().Add(remaining))
		line, err := c.reader.ReadString('\n')
		if err != nil {
			// timeout or closed — no matching message arrived
			return
		}
		line = strings.TrimRight(line, "\r\n")
		c.t.Logf("← %s", line)
		c.mu.Lock()
		c.received = append(c.received, line)
		c.mu.Unlock()
		if strings.HasPrefix(line, prefix) {
			c.t.Errorf("client: received unexpected message with prefix %q: %q", prefix, line)
			return
		}
	}
}

// Received returns a copy of all messages received on this connection so far.
func (c *Client) Received() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.received))
	copy(out, c.received)
	return out
}
