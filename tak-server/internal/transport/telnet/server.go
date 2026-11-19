// Package telnet serves the text protocol over raw TCP, one line per message.
package telnet

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport"
)

type Server struct {
	Acceptor    transport.Acceptor
	Log         *slog.Logger
	IdleTimeout time.Duration // defaults to transport.IdleTimeout
}

// Serve accepts connections on ln until ctx is cancelled.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	for {
		netConn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			return err
		}
		go s.handle(netConn)
	}
}

type conn struct {
	netConn net.Conn
	queue   *transport.Queue
}

func (c *conn) Send(line string) {
	if !c.queue.Push(line) && c.queue.Reason() == transport.CloseError {
		// Too slow to keep up: drop the socket now rather than draining a
		// full queue into it.
		c.netConn.Close()
	}
}

func (c *conn) Close(reason transport.CloseReason) { c.queue.Close(reason) }
func (c *conn) RemoteAddr() string                 { return c.netConn.RemoteAddr().String() }

func (s *Server) handle(netConn net.Conn) {
	idle := s.IdleTimeout
	if idle == 0 {
		idle = transport.IdleTimeout
	}
	connection := &conn{netConn: netConn, queue: transport.NewQueue()}
	go connection.writeLoop()

	peer := s.Acceptor.Accept(connection)
	defer peer.Closed()
	defer connection.queue.Close(transport.CloseNormal)

	r := bufio.NewReaderSize(netConn, transport.MaxLineBytes)
	for {
		netConn.SetReadDeadline(time.Now().Add(idle))
		line, err := r.ReadSlice('\n')
		if err != nil {
			var ne net.Error
			switch {
			case errors.Is(err, bufio.ErrBufferFull):
				s.Log.Info("telnet line too long", "remote", connection.RemoteAddr())
				connection.queue.Close(transport.CloseError)
			case errors.As(err, &ne) && ne.Timeout():
				connection.queue.Close(transport.CloseGoingAway)
			}
			return
		}
		peer.HandleLine(string(line[:len(line)-1]))
	}
}

// writeLoop sends queued lines until the queue closes, then closes the socket.
func (c *conn) writeLoop() {
	defer c.netConn.Close()
	writer := bufio.NewWriter(c.netConn)
	failed := false
	for line := range c.queue.Lines() {
		if failed {
			continue // drain so senders never see a stuck queue
		}
		c.netConn.SetWriteDeadline(time.Now().Add(transport.WriteTimeout))
		writer.WriteString(line)
		writer.WriteByte('\n')
		// Flush once the queue is momentarily empty, batching bursts.
		if len(c.queue.Lines()) == 0 {
			if err := writer.Flush(); err != nil {
				failed = true
				c.netConn.Close()
			}
		}
	}
	if !failed {
		c.netConn.SetWriteDeadline(time.Now().Add(transport.WriteTimeout))
		writer.Flush()
	}
}
