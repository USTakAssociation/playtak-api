package telnet

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport/transporttest"
)

func start(t *testing.T, idle time.Duration) (addr string, echo *transporttest.Echo) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	echo = transporttest.NewEcho()
	s := &Server{Acceptor: echo, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), IdleTimeout: idle}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go s.Serve(ctx, ln)
	return ln.Addr().String(), echo
}

type client struct {
	t  *testing.T
	nc net.Conn
	r  *bufio.Reader
}

func dial(t *testing.T, addr string) *client {
	t.Helper()
	nc, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { nc.Close() })
	return &client{t: t, nc: nc, r: bufio.NewReader(nc)}
}

func (c *client) write(s string) {
	c.t.Helper()
	if _, err := io.WriteString(c.nc, s); err != nil {
		c.t.Fatal(err)
	}
}

func (c *client) expect(want string) {
	c.t.Helper()
	c.nc.SetReadDeadline(time.Now().Add(5 * time.Second))
	got, err := c.r.ReadString('\n')
	if err != nil {
		c.t.Fatalf("read: %v", err)
	}
	if got != want+"\n" {
		c.t.Errorf("got %q, want %q", got, want+"\n")
	}
}

// expectEOF reads until the server closes the connection.
func (c *client) expectEOF() {
	c.t.Helper()
	c.nc.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.Copy(io.Discard, c.r); err != nil && !errors.Is(err, net.ErrClosed) && !strings.Contains(err.Error(), "reset") {
		c.t.Fatalf("expected EOF, got %v", err)
	}
}

func TestLines(t *testing.T) {
	addr, _ := start(t, 0)
	c := dial(t, addr)
	c.expect("hello")
	// Several lines in one write, a split line, and CRLF (passed through to the peer).
	c.write("one\ntwo\nthr")
	c.write("ee\r\n")
	c.expect("echo:one")
	c.expect("echo:two")
	c.expect("echo:three\r")
	c.write("héllo 🐢\n")
	c.expect("echo:héllo 🐢")
}

func TestCloseDeliversQueuedLinesFirst(t *testing.T) {
	addr, echo := start(t, 0)
	c := dial(t, addr)
	c.expect("hello")
	c.write("close:kicked\n")
	c.expect("echo:close:kicked")
	c.expectEOF()
	<-echo.Closed
}

func TestClientDisconnectCallsClosed(t *testing.T) {
	addr, echo := start(t, 0)
	c := dial(t, addr)
	c.expect("hello")
	c.nc.Close()
	select {
	case <-echo.Closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Closed not called")
	}
}

func TestLineTooLongCloses(t *testing.T) {
	addr, echo := start(t, 0)
	c := dial(t, addr)
	c.expect("hello")
	c.write(strings.Repeat("x", transport.MaxLineBytes+1))
	c.expectEOF()
	<-echo.Closed
}

func TestIdleTimeout(t *testing.T) {
	addr, echo := start(t, 100*time.Millisecond)
	c := dial(t, addr)
	c.expect("hello")
	c.expectEOF()
	<-echo.Closed
}

func TestSlowClientIsDisconnected(t *testing.T) {
	addr, echo := start(t, 0)
	c := dial(t, addr)
	c.expect("hello")
	c.write("flood\n") // never read: the queue overflows
	select {
	case <-echo.Closed:
	case <-time.After(15 * time.Second):
		t.Fatal("slow client not disconnected")
	}
}
