package websocket

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/clientip"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport/transporttest"
)

func start(t *testing.T, idle time.Duration) (url string, echo *transporttest.Echo) {
	t.Helper()
	echo = transporttest.NewEcho()
	h := &Handler{Acceptor: echo, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), IdleTimeout: idle}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws", echo
}

func dial(t *testing.T, url string, subprotocols ...string) *websocket.Conn {
	t.Helper()
	d := websocket.Dialer{Subprotocols: subprotocols}
	ws, _, err := d.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ws.Close() })
	return ws
}

func expect(t *testing.T, ws *websocket.Conn, want string) {
	t.Helper()
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	typ, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if typ != websocket.BinaryMessage {
		t.Errorf("message type %d, want binary", typ)
	}
	if string(data) != want+"\n" {
		t.Errorf("got %q, want %q", data, want+"\n")
	}
}

func expectClose(t *testing.T, ws *websocket.Conn, code int) {
	t.Helper()
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, _, err := ws.ReadMessage()
		if err == nil {
			continue
		}
		var ce *websocket.CloseError
		if !errors.As(err, &ce) {
			t.Fatalf("expected close frame, got %v", err)
		}
		if ce.Code != code {
			t.Errorf("close code %d, want %d", ce.Code, code)
		}
		return
	}
}

func TestMessages(t *testing.T) {
	url, _ := start(t, 0)
	ws := dial(t, url, "binary")
	if ws.Subprotocol() != "binary" {
		t.Errorf("subprotocol %q, want binary", ws.Subprotocol())
	}
	expect(t, ws, "hello")

	ws.WriteMessage(websocket.BinaryMessage, []byte("PING\n"))
	expect(t, ws, "echo:PING\n")
	ws.WriteMessage(websocket.TextMessage, []byte("héllo 🐢"))
	expect(t, ws, "echo:héllo 🐢")
}

func TestConnectsWithoutSubprotocol(t *testing.T) {
	url, _ := start(t, 0)
	ws := dial(t, url)
	expect(t, ws, "hello")
}

func TestCloseCodes(t *testing.T) {
	for reason, code := range map[string]int{
		"normal": websocket.CloseNormalClosure,
		"kicked": websocket.ClosePolicyViolation,
		"away":   websocket.CloseGoingAway,
		"error":  websocket.CloseInternalServerErr,
	} {
		t.Run(reason, func(t *testing.T) {
			url, echo := start(t, 0)
			ws := dial(t, url, "binary")
			expect(t, ws, "hello")
			ws.WriteMessage(websocket.BinaryMessage, []byte("close:"+reason))
			expect(t, ws, "echo:close:"+reason) // queued lines arrive before the close
			expectClose(t, ws, code)
			<-echo.Closed
		})
	}
}

func TestIdleTimeoutSendsGoingAway(t *testing.T) {
	url, echo := start(t, 100*time.Millisecond)
	ws := dial(t, url, "binary")
	expect(t, ws, "hello")
	expectClose(t, ws, websocket.CloseGoingAway)
	<-echo.Closed
}

func TestClientDisconnectCallsClosed(t *testing.T) {
	url, echo := start(t, 0)
	ws := dial(t, url, "binary")
	expect(t, ws, "hello")
	ws.Close()
	select {
	case <-echo.Closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Closed not called")
	}
}

func TestClientIPBehindTrustedProxy(t *testing.T) {
	resolver, err := clientip.NewResolver("127.0.0.1/32,::1/128")
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{Acceptor: transporttest.NewEcho(), Log: slog.New(slog.NewTextHandler(io.Discard, nil)), ClientIP: resolver}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), http.Header{"X-Real-IP": {"198.51.100.9"}})
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	expect(t, ws, "hello")
	ws.WriteMessage(websocket.TextMessage, []byte("addr"))
	expect(t, ws, "echo:addr")
	expect(t, ws, "addr:198.51.100.9")
}

func TestClientIPWithoutResolver(t *testing.T) {
	url, _ := start(t, time.Minute)
	ws := dial(t, url)
	expect(t, ws, "hello")
	ws.WriteMessage(websocket.TextMessage, []byte("addr"))
	expect(t, ws, "echo:addr")
	_, data, err := ws.ReadMessage()
	if err != nil || !strings.HasPrefix(string(data), "addr:127.0.0.1:") {
		t.Errorf("got %q %v, want the connection address", data, err)
	}
}

func TestConnectionsPerIPCapped(t *testing.T) {
	resolver, _ := clientip.NewResolver("127.0.0.1/32,::1/128")
	limit, _ := transport.NewConnLimit(1, "")
	h := &Handler{Acceptor: transporttest.NewEcho(), Log: slog.New(slog.NewTextHandler(io.Discard, nil)), ClientIP: resolver, Limit: limit}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	from := func(ip string) (*websocket.Conn, *http.Response, error) {
		return websocket.DefaultDialer.Dial(url, http.Header{"X-Real-IP": {ip}})
	}

	ws, _, err := from("198.51.100.9")
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	expect(t, ws, "hello")
	// Judged by the client IP the proxy reports, not the proxy's own.
	if _, resp, err := from("198.51.100.9"); err == nil || resp == nil || resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("second connection from the same client: want 429, got %v %v", resp, err)
	}
	other, _, err := from("198.51.100.10")
	if err != nil {
		t.Fatalf("another client behind the same proxy should connect: %v", err)
	}
	other.Close()
}
