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
		nc, err := ln.Accept()
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
		go s.handle(nc)
	}
}

type conn struct {
	nc    net.Conn
	queue *transport.Queue
}

func (c *conn) Send(line string) {
	if !c.queue.Push(line) && c.queue.Reason() == transport.CloseError {
		// Too slow to keep up: drop the socket now rather than draining a
		// full queue into it.
		c.nc.Close()
	}
}

func (c *conn) Close(reason transport.CloseReason) { c.queue.Close(reason) }
func (c *conn) RemoteAddr() string                 { return c.nc.RemoteAddr().String() }

func (s *Server) handle(nc net.Conn) {
	idle := s.IdleTimeout
	if idle == 0 {
		idle = transport.IdleTimeout
	}
	c := &conn{nc: nc, queue: transport.NewQueue()}
	go c.writeLoop()

	peer := s.Acceptor.Accept(c)
	defer peer.Closed()
	defer c.queue.Close(transport.CloseNormal)

	r := bufio.NewReaderSize(nc, transport.MaxLineBytes)
	for {
		nc.SetReadDeadline(time.Now().Add(idle))
		line, err := r.ReadSlice('\n')
		if err != nil {
			var ne net.Error
			switch {
			case errors.Is(err, bufio.ErrBufferFull):
				s.Log.Info("telnet line too long", "remote", c.RemoteAddr())
				c.queue.Close(transport.CloseError)
			case errors.As(err, &ne) && ne.Timeout():
				c.queue.Close(transport.CloseGoingAway)
			}
			return
		}
		peer.HandleLine(string(line[:len(line)-1]))
	}
}

// writeLoop sends queued lines until the queue closes, then closes the socket.
func (c *conn) writeLoop() {
	defer c.nc.Close()
	w := bufio.NewWriter(c.nc)
	failed := false
	for line := range c.queue.Lines() {
		if failed {
			continue // drain so senders never see a stuck queue
		}
		c.nc.SetWriteDeadline(time.Now().Add(transport.WriteTimeout))
		w.WriteString(line)
		w.WriteByte('\n')
		// Flush once the queue is momentarily empty, batching bursts.
		if len(c.queue.Lines()) == 0 {
			if err := w.Flush(); err != nil {
				failed = true
				c.nc.Close()
			}
		}
	}
	if !failed {
		c.nc.SetWriteDeadline(time.Now().Add(transport.WriteTimeout))
		w.Flush()
	}
}
