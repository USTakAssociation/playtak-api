// Package transport defines how the session layer talks to client
// connections, independent of whether they arrive over telnet or WebSocket.
package transport

import (
	"sync"
	"time"
)

const (
	// IdleTimeout closes connections that send nothing for this long.
	// Clients are expected to PING more often (playtak-ui does every 10s).
	IdleTimeout = 60 * time.Second
	// MaxLineBytes is the longest line or message a client may send.
	MaxLineBytes = 64 * 1024
	// WriteTimeout bounds each write, so a stalled client can't hold a
	// writer goroutine forever.
	WriteTimeout = 10 * time.Second
	// SendQueueSize is how many outgoing lines may wait for a slow client
	// before it is disconnected.
	SendQueueSize = 1024
)

// CloseReason says why a connection is being closed. WebSocket clients
// receive it as a close code; telnet clients don't see it.
type CloseReason int

const (
	CloseNormal    CloseReason = iota // the client quit
	CloseKicked                       // replaced by another login, kicked or banned
	CloseGoingAway                    // idle timeout or server shutdown
	CloseError                        // server error or a client too slow to keep up
)

// Conn is one client connection.
type Conn interface {
	// Send queues a line for delivery without blocking. A client that falls
	// SendQueueSize lines behind is disconnected.
	Send(line string)
	// Close delivers any queued lines, then closes the connection.
	Close(reason CloseReason)
	RemoteAddr() string
}

// Peer receives what arrives on a Conn. HandleLine is called sequentially
// from the connection's read goroutine; Closed is called once, after the last
// HandleLine, when the connection has gone away for any reason.
type Peer interface {
	HandleLine(line string)
	Closed()
}

// Acceptor creates a Peer for each new connection.
type Acceptor interface {
	Accept(c Conn) Peer
}

// Queue buffers outgoing lines for a connection's single writer goroutine.
type Queue struct {
	mu     sync.Mutex
	lines  chan string
	closed bool
	reason CloseReason
}

func NewQueue() *Queue {
	return &Queue{lines: make(chan string, SendQueueSize)}
}

// Push queues line. It reports false if the queue was already closed or has
// just been closed because it is full.
func (q *Queue) Push(line string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return false
	}
	select {
	case q.lines <- line:
		return true
	default:
		q.closeLocked(CloseError)
		return false
	}
}

// Close stops accepting lines. The first reason given wins.
func (q *Queue) Close(reason CloseReason) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closeLocked(reason)
}

func (q *Queue) closeLocked(reason CloseReason) {
	if !q.closed {
		q.closed = true
		q.reason = reason
		close(q.lines)
	}
}

// Lines yields queued lines and ends once the queue is closed and drained.
func (q *Queue) Lines() <-chan string { return q.lines }

// Reason returns why the queue was closed.
func (q *Queue) Reason() CloseReason {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.reason
}
