package core

import (
	"fmt"
	"net"
	"net/textproto"
	"strconv"
	"time"
)

// respCode extracts the leading numeric SMTP status code from a raw
// "<code> <message>" response string (0 if none).
func respCode(s string) int {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0
	}
	n, _ := strconv.Atoi(s[:i])
	return n
}

// dialTimeout is the per-operation timeout for SMTP sessions.
var dialTimeout = 15 * time.Second

// smtpPort is the TCP port used for MX probing.
var smtpPort = "25"

// Session is a reusable SMTP connection. A single TCP connection and EHLO can
// serve many RCPT TO probes, which is far faster and quieter than opening a new
// connection per address. If the connection breaks, the next Probe transparently
// reconnects.
type Session struct {
	mx     string
	domain string
	conn   net.Conn
	tc     *textproto.Conn
}

// NewSession opens a connection, reads the greeting and sends EHLO.
func NewSession(mx, domain string) (*Session, error) {
	s := &Session{mx: mx, domain: domain}
	if err := s.connect(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Session) connect() error {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(s.mx, smtpPort), dialTimeout)
	if err != nil {
		return err
	}
	s.conn = conn
	s.tc = textproto.NewConn(conn)
	s.refreshDeadline()

	if _, _, err := s.tc.ReadResponse(220); err != nil {
		s.Close()
		return err
	}
	if err := cmd(s.tc, 250, "EHLO %s", s.domain); err != nil {
		s.Close()
		return err
	}
	return nil
}

func (s *Session) refreshDeadline() {
	if s.conn != nil {
		_ = s.conn.SetDeadline(time.Now().Add(dialTimeout))
	}
}

// Probe runs MAIL FROM / RCPT TO for one recipient and returns the raw
// "<code> <message>" RCPT response. On any I/O error it drops the connection so
// the next call reconnects, and returns the error.
func (s *Session) Probe(rcpt string) (string, error) {
	if s.tc == nil {
		if err := s.connect(); err != nil {
			return "", err
		}
	}
	s.refreshDeadline()

	if _, err := send(s.tc, "MAIL FROM:<test@%s>", s.domain); err != nil {
		s.Close()
		return "", err
	}
	result, err := send(s.tc, "RCPT TO:<%s>", rcpt)
	if err != nil {
		s.Close()
		return "", err
	}
	// Clear the transaction so the next MAIL FROM is accepted on this same
	// connection. Any response code is fine here.
	_ = cmd(s.tc, 0, "RSET")
	return result, nil
}

// VRFY issues a VRFY command to ask the server to verify a user/address.
func (s *Session) VRFY(user string) (string, error) { return s.command("VRFY %s", user) }

// EXPN issues an EXPN command to expand a mailing list / alias.
func (s *Session) EXPN(list string) (string, error) { return s.command("EXPN %s", list) }

// command runs a single bare SMTP command (no MAIL FROM transaction), returning
// the raw "<code> <message>" response. On error it drops the connection.
func (s *Session) command(format string, a ...any) (string, error) {
	if s.tc == nil {
		if err := s.connect(); err != nil {
			return "", err
		}
	}
	s.refreshDeadline()
	result, err := send(s.tc, format, a...)
	if err != nil {
		s.Close()
		return "", err
	}
	return result, nil
}

// Close shuts down the connection. Safe to call multiple times.
func (s *Session) Close() {
	if s.tc != nil {
		_ = s.tc.Close()
		s.tc = nil
	}
	s.conn = nil
}

// smtpResult is a one-shot probe used for connectivity and sanity checks.
func smtpResult(mx, domain, rcpt string) (string, error) {
	s, err := NewSession(mx, domain)
	if err != nil {
		return "", err
	}
	defer s.Close()
	return s.Probe(rcpt)
}

// cmd sends a command and requires the given status code (0 = accept any).
func cmd(tc *textproto.Conn, expect int, format string, a ...any) error {
	id, err := tc.Cmd(format, a...)
	if err != nil {
		return err
	}
	tc.StartResponse(id)
	defer tc.EndResponse(id)
	_, _, err = tc.ReadResponse(expect)
	return err
}

// send sends a command and returns the raw "<code> <message>" response,
// accepting any status code.
func send(tc *textproto.Conn, format string, a ...any) (string, error) {
	id, err := tc.Cmd(format, a...)
	if err != nil {
		return "", err
	}
	tc.StartResponse(id)
	defer tc.EndResponse(id)
	code, msg, err := tc.ReadResponse(0)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d %s", code, msg), nil
}
