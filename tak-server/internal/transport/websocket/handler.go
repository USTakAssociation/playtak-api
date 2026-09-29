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

	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport"
)

type Handler struct {
	Acceptor    transport.Acceptor
	Log         *slog.Logger
	IdleTimeout time.Duration // defaults to transport.IdleTimeout
}

var upgrader = websocket.Upgrader{
	Subprotocols: []string{"binary"},
	// Clients connect from playtak.com, localhost and other hosts; there are
	// no cookies to protect, so any origin is allowed.
	CheckOrigin: func(*http.Request) bool { return true },
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade has already written an error response
	}
	idle := h.IdleTimeout
	if idle == 0 {
		idle = transport.IdleTimeout
	}
	ws.SetReadLimit(transport.MaxLineBytes)

	c := &conn{ws: ws, queue: transport.NewQueue()}
	go c.writeLoop()

	peer := h.Acceptor.Accept(c)
	defer peer.Closed()
	defer c.queue.Close(transport.CloseNormal)

	for {
		ws.SetReadDeadline(time.Now().Add(idle))
		_, data, err := ws.ReadMessage()
		if err != nil {
			var ne net.Error
			switch {
			case errors.Is(err, websocket.ErrReadLimit):
				h.Log.Info("websocket message too long", "remote", c.RemoteAddr())
				c.queue.Close(transport.CloseError)
			case errors.As(err, &ne) && ne.Timeout():
				c.queue.Close(transport.CloseGoingAway)
			}
			return
		}
		peer.HandleLine(string(data))
	}
}

type conn struct {
	ws    *websocket.Conn
	queue *transport.Queue
}

func (c *conn) Send(line string) {
	if !c.queue.Push(line) && c.queue.Reason() == transport.CloseError {
		c.ws.NetConn().Close()
	}
}

func (c *conn) Close(reason transport.CloseReason) { c.queue.Close(reason) }
func (c *conn) RemoteAddr() string                 { return c.ws.RemoteAddr().String() }

var closeCodes = map[transport.CloseReason]int{
	transport.CloseNormal:    websocket.CloseNormalClosure,
	transport.CloseKicked:    websocket.ClosePolicyViolation,
	transport.CloseGoingAway: websocket.CloseGoingAway,
	transport.CloseError:     websocket.CloseInternalServerErr,
}

// writeLoop sends queued lines until the queue closes, then sends a close
// frame with the close reason and closes the socket.
func (c *conn) writeLoop() {
	defer c.ws.Close()
	for line := range c.queue.Lines() {
		c.ws.SetWriteDeadline(time.Now().Add(transport.WriteTimeout))
		if err := c.ws.WriteMessage(websocket.BinaryMessage, []byte(line+"\n")); err != nil {
			c.ws.NetConn().Close()
			for range c.queue.Lines() {
				// drain so senders never see a stuck queue
			}
			return
		}
	}
	msg := websocket.FormatCloseMessage(closeCodes[c.queue.Reason()], "")
	c.ws.WriteControl(websocket.CloseMessage, msg, time.Now().Add(time.Second))
}
