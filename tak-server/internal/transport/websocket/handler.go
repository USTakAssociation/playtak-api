// Package websocket serves the text protocol over WebSocket. Each message
// carries one line; replies are sent as binary frames ending in "\n", which is
// what existing clients expect.
package websocket

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/clientip"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport"
)

type Handler struct {
	Acceptor    transport.Acceptor
	Log         *slog.Logger
	IdleTimeout time.Duration // defaults to transport.IdleTimeout
	// ClientIP finds the client's address behind a reverse proxy. Nil uses
	// the connection's own address.
	ClientIP *clientip.Resolver
}

var upgrader = websocket.Upgrader{
	Subprotocols: []string{"binary"},
	// Clients connect from playtak.com, localhost and other hosts; there are
	// no cookies to protect, so any origin is allowed.
	CheckOrigin: func(*http.Request) bool { return true },
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	socket, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade has already written an error response
	}
	idle := h.IdleTimeout
	if idle == 0 {
		idle = transport.IdleTimeout
	}
	socket.SetReadLimit(transport.MaxLineBytes)

	remote := r.RemoteAddr
	if h.ClientIP != nil {
		remote = h.ClientIP.IP(r)
	}
	connection := &conn{socket: socket, queue: transport.NewQueue(), remote: remote}
	go connection.writeLoop()

	peer := h.Acceptor.Accept(connection)
	defer peer.Closed()
	defer connection.queue.Close(transport.CloseNormal)

	for {
		socket.SetReadDeadline(time.Now().Add(idle))
		_, data, err := socket.ReadMessage()
		if err != nil {
			var ne net.Error
			switch {
			case errors.Is(err, websocket.ErrReadLimit):
				h.Log.Info("websocket message too long", "remote", connection.RemoteAddr())
				connection.queue.Close(transport.CloseError)
			case errors.As(err, &ne) && ne.Timeout():
				connection.queue.Close(transport.CloseGoingAway)
			}
			return
		}
		peer.HandleLine(string(data))
	}
}

type conn struct {
	socket *websocket.Conn
	queue  *transport.Queue
	remote string
}

func (c *conn) Send(line string) {
	if !c.queue.Push(line) && c.queue.Reason() == transport.CloseError {
		c.socket.NetConn().Close()
	}
}

func (c *conn) Close(reason transport.CloseReason) { c.queue.Close(reason) }
func (c *conn) RemoteAddr() string                 { return c.remote }

var closeCodes = map[transport.CloseReason]int{
	transport.CloseNormal:    websocket.CloseNormalClosure,
	transport.CloseKicked:    websocket.ClosePolicyViolation,
	transport.CloseGoingAway: websocket.CloseGoingAway,
	transport.CloseError:     websocket.CloseInternalServerErr,
}

// writeLoop sends queued lines until the queue closes, then sends a close
// frame with the close reason and closes the socket.
func (c *conn) writeLoop() {
	defer c.socket.Close()
	for line := range c.queue.Lines() {
		c.socket.SetWriteDeadline(time.Now().Add(transport.WriteTimeout))
		if err := c.socket.WriteMessage(websocket.BinaryMessage, []byte(line+"\n")); err != nil {
			c.socket.NetConn().Close()
			for range c.queue.Lines() {
				// drain so senders never see a stuck queue
			}
			return
		}
	}
	closeFrame := websocket.FormatCloseMessage(closeCodes[c.queue.Reason()], "")
	c.socket.WriteControl(websocket.CloseMessage, closeFrame, time.Now().Add(time.Second))
}
