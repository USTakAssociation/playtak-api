// Package transporttest provides a simple Acceptor for transport tests.
package transporttest

import (
	"strings"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport"
)

// Echo greets each connection with "hello" and replies to every line with
// "echo:<line>". The line "close:<normal|kicked|away|error>" then closes the
// connection with that reason, and "flood" queues more lines than a
// connection may have pending.
type Echo struct {
	Closed chan struct{} // receives once per closed connection
}

func NewEcho() *Echo { return &Echo{Closed: make(chan struct{}, 100)} }

func (e *Echo) Accept(c transport.Conn) transport.Peer {
	c.Send("hello")
	return &echoPeer{e: e, c: c}
}

var reasons = map[string]transport.CloseReason{
	"normal": transport.CloseNormal,
	"kicked": transport.CloseKicked,
	"away":   transport.CloseGoingAway,
	"error":  transport.CloseError,
}

type echoPeer struct {
	e *Echo
	c transport.Conn
}

func (p *echoPeer) HandleLine(line string) {
	p.c.Send("echo:" + line)
	if r, ok := strings.CutPrefix(line, "close:"); ok {
		p.c.Close(reasons[r])
	}
	if line == "flood" {
		for range 2 * transport.SendQueueSize {
			p.c.Send(strings.Repeat("x", 1000))
		}
	}
}

func (p *echoPeer) Closed() { p.e.Closed <- struct{}{} }
