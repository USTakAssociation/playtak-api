// Package email sends mail through an SMTP server over implicit TLS.
package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"time"
)

const defaultPort = "465"

// SMTP sends mail through one server, connecting per message.
type SMTP struct {
	addr, host     string
	user, password string
	from           string
}

// NewSMTP returns a sender for hostport ("host" or "host:port", port 465 if
// omitted).
func NewSMTP(hostport, user, password, from string) *SMTP {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		host, port = hostport, defaultPort
	}
	return &SMTP{addr: net.JoinHostPort(host, port), host: host, user: user, password: password, from: from}
}

func (s *SMTP) Send(ctx context.Context, to, subject, body string) error {
	if strings.ContainsAny(to+subject, "\r\n") {
		return fmt.Errorf("send mail: header contains a line break")
	}
	dialer := &tls.Dialer{NetDialer: &net.Dialer{Timeout: 10 * time.Second}, Config: &tls.Config{ServerName: s.host}}
	conn, err := dialer.DialContext(ctx, "tcp", s.addr)
	if err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}

	c, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	defer c.Close()
	if err := c.Auth(smtp.PlainAuth("", s.user, s.password, s.host)); err != nil {
		return fmt.Errorf("send mail: auth: %w", err)
	}
	if err := c.Mail(s.from); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n",
		s.from, to, subject, body)
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	return c.Quit()
}

// Log is used when no SMTP server is configured. It logs each message
// instead of sending it, which makes emailed passwords visible in local
// development.
type Log struct {
	Logger *slog.Logger
}

func (l Log) Send(_ context.Context, to, subject, body string) error {
	l.Logger.Info("email not sent (no SMTP server configured)", "to", to, "subject", subject, "body", body)
	return nil
}
