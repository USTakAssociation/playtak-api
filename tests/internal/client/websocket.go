package client

import (
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
	"sync/atomic"
	"github.com/gorilla/websocket"
)

// WSClient is a WebSocket connection to the Tak server.
// It mirrors the API of Client so helpers work with both transports.
type WSClient struct {
	t        *testing.T
	conn     *websocket.Conn
	writeMu  sync.Mutex
	mu       sync.Mutex
	received []string
	cId      int
}

// NewWS dials the server at addr over WebSocket and returns a WSClient.
// The connection is automatically closed when the test ends via t.Cleanup.
func NewWS(t *testing.T, addr string) *WSClient {
	t.Helper()

	u := url.URL{Scheme: "ws", Host: addr, Path: "/"}
	dialer := websocket.Dialer{
		HandshakeTimeout: DefaultTimeout,
		Subprotocols:     []string{"binary"},
	}

	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("wsclient: failed to connect to %s: %v", addr, err)
	}
	connectionID := atomic.AddUint64(&globalConnectionCounter, 1)
	c := &WSClient{t: t, conn: conn, cId: int(connectionID)}
	t.Cleanup(func() { c.Close() })
	return c
}

// Close closes the WebSocket connection.
func (c *WSClient) Close() {
	c.conn.Close()
}

// Send writes a command to the server as a WebSocket binary message.
// A newline is appended to match the server's line-oriented protocol.
func (c *WSClient) Send(cmd string) {
	c.t.Helper()
	c.t.Logf("S → %s (WC #%d)", cmd, c.cId)
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.conn.WriteMessage(websocket.BinaryMessage, []byte(cmd+"\n")); err != nil {
		c.t.Fatalf("wsclient: send failed: %v", err)
	}
}

// Recv reads the next message from the WebSocket connection.
// Blocks up to DefaultTimeout.
func (c *WSClient) Recv() string {
	c.t.Helper()
	c.conn.SetReadDeadline(time.Now().Add(DefaultTimeout))
	_, msg, err := c.conn.ReadMessage()
	if err != nil {
		c.t.Fatalf("wsclient: recv failed: %v", err)
	}
	line := strings.TrimRight(string(msg), "\r\n")
	c.t.Logf("R ← %s (WC #%d)", line, c.cId)
	c.mu.Lock()
	c.received = append(c.received, line)
	c.mu.Unlock()
	return line
}

// Expect reads the next message and fails if it does not match exactly.
func (c *WSClient) Expect(want string) {
	c.t.Helper()
	got := c.Recv()
	if got != want {
		c.t.Errorf("wsclient: Expect mismatch\n  want: %q\n   got: %q", want, got)
	}
}

// ExpectPrefix reads the next message and fails if it does not start with prefix.
// Returns the full received message.
func (c *WSClient) ExpectPrefix(prefix string) string {
	c.t.Helper()
	got := c.Recv()
	if !strings.HasPrefix(got, prefix) {
		c.t.Errorf("wsclient: ExpectPrefix mismatch\n  want prefix: %q\n          got: %q", prefix, got)
	}
	return got
}

// ExpectAny reads messages until it finds one that matches exactly.
func (c *WSClient) ExpectAny(want string) {
	c.t.Helper()
	deadline := time.Now().Add(DefaultTimeout)
	for time.Now().Before(deadline) {
		got := c.Recv()
		if got == want {
			return
		}
	}
	c.t.Errorf("wsclient: never received %q within timeout", want)
}

// ExpectAnyPrefix reads messages until it finds one that starts with prefix.
// Returns the matching message.
func (c *WSClient) ExpectAnyPrefix(prefix string) string {
	c.t.Helper()
	deadline := time.Now().Add(DefaultTimeout)
	for time.Now().Before(deadline) {
		got := c.Recv()
		if strings.HasPrefix(got, prefix) {
			return got
		}
	}
	c.t.Fatalf("wsclient: never received prefix %q within timeout", prefix)
	return ""
}

// DrainUntil reads and discards messages until one starts with prefix.
// Returns the matching message.
func (c *WSClient) DrainUntil(prefix string) string {
	c.t.Helper()
	deadline := time.Now().Add(DefaultTimeout)
	for time.Now().Before(deadline) {
		got := c.Recv()
		if strings.HasPrefix(got, prefix) {
			return got
		}
	}
	c.t.Fatalf("wsclient: timed out waiting for message with prefix %q", prefix)
	return ""
}

// ExpectNoMessage asserts the server sends nothing within the given window.
func (c *WSClient) ExpectNoMessage(window time.Duration) {
	c.t.Helper()
	c.conn.SetReadDeadline(time.Now().Add(window))
	_, _, err := c.conn.ReadMessage()
	if err == nil {
		c.t.Errorf("wsclient: expected no message but connection still delivered one")
	}
}

// Received returns a copy of all messages received so far.
func (c *WSClient) Received() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.received))
	copy(out, c.received)
	return out
}
