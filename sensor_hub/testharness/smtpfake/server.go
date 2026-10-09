// Package smtpfake is an SMTP server for tests. It speaks enough of SMTP for
// a real client to deliver through it, STARTTLS, implicit TLS and AUTH PLAIN
// and LOGIN included, and records every message it accepts.
package smtpfake

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
)

// Options says what the server offers and requires.
type Options struct {
	// Username and Password, when Username is set, are the only credentials
	// the server accepts, and it refuses mail until a client has logged in
	// with them. Otherwise it takes mail without a login.
	Username string
	Password string
	// TLS, when set, is offered with STARTTLS, or from the first byte with
	// ImplicitTLS.
	TLS         *tls.Config
	ImplicitTLS bool
}

// Message is one message the server accepted.
type Message struct {
	From string
	To   []string
	// Data is the message as sent, headers and body, with the SMTP dot
	// stuffing undone.
	Data string
	// TLS says whether the connection was encrypted when it was sent.
	TLS bool
}

// Server is a running fake SMTP server.
type Server struct {
	opts     Options
	listener net.Listener

	mu           sync.Mutex
	messages     []Message
	authAttempts int
	conns        map[net.Conn]bool

	wg sync.WaitGroup
}

// Start listens on a free loopback port and serves until Close.
func Start(opts Options) (*Server, error) {
	if opts.ImplicitTLS && opts.TLS == nil {
		return nil, errors.New("implicit TLS needs a TLS config")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	if opts.ImplicitTLS {
		listener = tls.NewListener(listener, opts.TLS)
	}
	s := &Server{opts: opts, listener: listener, conns: map[net.Conn]bool{}}
	s.wg.Add(1)
	go s.serve()
	return s, nil
}

// Host is the address the server listens on, without the port.
func (s *Server) Host() string {
	return s.listener.Addr().(*net.TCPAddr).IP.String()
}

// Port is the port the server listens on.
func (s *Server) Port() int {
	return s.listener.Addr().(*net.TCPAddr).Port
}

// Messages returns a copy of the messages accepted so far.
func (s *Server) Messages() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Message(nil), s.messages...)
}

// AuthAttempts is how many AUTH commands clients have sent, accepted or not.
func (s *Server) AuthAttempts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.authAttempts
}

// Reset forgets the messages and AUTH attempts recorded so far.
func (s *Server) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = nil
	s.authAttempts = 0
}

// Close stops the server and drops every open connection.
func (s *Server) Close() {
	_ = s.listener.Close()
	s.mu.Lock()
	for conn := range s.conns {
		_ = conn.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *Server) serve() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.conns[conn] = true
		s.mu.Unlock()
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer func() {
				_ = conn.Close()
				s.mu.Lock()
				delete(s.conns, conn)
				s.mu.Unlock()
			}()
			newSession(s, conn).run()
		}()
	}
}

type session struct {
	server *Server
	conn   net.Conn
	r      *bufio.Reader
	w      *bufio.Writer
	tls    bool
	authed bool
	from   string
	to     []string
}

func newSession(server *Server, conn net.Conn) *session {
	return &session{
		server: server,
		conn:   conn,
		r:      bufio.NewReader(conn),
		w:      bufio.NewWriter(conn),
		tls:    server.opts.ImplicitTLS,
	}
}

func (s *session) reply(lines ...string) bool {
	for _, line := range lines {
		if _, err := s.w.WriteString(line + "\r\n"); err != nil {
			return false
		}
	}
	return s.w.Flush() == nil
}

func (s *session) readLine() (string, error) {
	line, err := s.r.ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}

func (s *session) offersSTARTTLS() bool {
	return s.server.opts.TLS != nil && !s.tls
}

func (s *session) run() {
	if !s.reply("220 fake.smtp ESMTP ready") {
		return
	}
	for {
		line, err := s.readLine()
		if err != nil {
			return
		}
		verb, arg, _ := strings.Cut(line, " ")
		switch strings.ToUpper(verb) {
		case "EHLO", "HELO":
			lines := []string{"250-fake.smtp greets " + arg}
			if s.offersSTARTTLS() {
				lines = append(lines, "250-STARTTLS")
			}
			lines = append(lines, "250-AUTH PLAIN LOGIN", "250 8BITMIME")
			if !s.reply(lines...) {
				return
			}
		case "STARTTLS":
			if !s.offersSTARTTLS() {
				s.reply("502 5.5.1 STARTTLS not offered")
				continue
			}
			if !s.reply("220 2.0.0 Ready to start TLS") {
				return
			}
			tlsConn := tls.Server(s.conn, s.server.opts.TLS)
			if err := tlsConn.Handshake(); err != nil {
				return
			}
			s.conn, s.tls, s.authed = tlsConn, true, false
			s.r, s.w = bufio.NewReader(tlsConn), bufio.NewWriter(tlsConn)
		case "AUTH":
			if !s.auth(arg) {
				return
			}
		case "MAIL":
			if s.server.opts.Username != "" && !s.authed {
				s.reply("530 5.7.0 Authentication required")
				continue
			}
			s.from, s.to = addressIn(arg), nil
			s.reply("250 2.1.0 OK")
		case "RCPT":
			s.to = append(s.to, addressIn(arg))
			s.reply("250 2.1.5 OK")
		case "DATA":
			if !s.data() {
				return
			}
		case "RSET":
			s.from, s.to = "", nil
			s.reply("250 2.0.0 OK")
		case "NOOP":
			s.reply("250 2.0.0 OK")
		case "QUIT":
			s.reply("221 2.0.0 Bye")
			return
		default:
			s.reply("500 5.5.2 Unknown command")
		}
	}
}

// auth runs an AUTH exchange and reports whether the connection is still
// usable.
func (s *session) auth(arg string) bool {
	s.server.mu.Lock()
	s.server.authAttempts++
	s.server.mu.Unlock()

	mechanism, initial, _ := strings.Cut(arg, " ")
	var username, password string
	switch strings.ToUpper(mechanism) {
	case "PLAIN":
		if initial == "" {
			if !s.reply("334 ") {
				return false
			}
			line, err := s.readLine()
			if err != nil {
				return false
			}
			initial = line
		}
		decoded, err := base64.StdEncoding.DecodeString(initial)
		if err != nil {
			return s.reply("501 5.5.2 Cannot decode the response")
		}
		parts := strings.Split(string(decoded), "\x00")
		if len(parts) != 3 {
			return s.reply("501 5.5.2 Malformed PLAIN response")
		}
		username, password = parts[1], parts[2]
	case "LOGIN":
		var ok bool
		if username, ok = s.challenge("Username:"); !ok {
			return false
		}
		if password, ok = s.challenge("Password:"); !ok {
			return false
		}
	default:
		return s.reply("504 5.5.4 Unrecognised authentication mechanism")
	}
	if username != s.server.opts.Username || password != s.server.opts.Password || s.server.opts.Username == "" {
		return s.reply("535 5.7.8 Authentication credentials invalid")
	}
	s.authed = true
	return s.reply("235 2.7.0 Authentication successful")
}

func (s *session) challenge(prompt string) (string, bool) {
	if !s.reply("334 " + base64.StdEncoding.EncodeToString([]byte(prompt))) {
		return "", false
	}
	line, err := s.readLine()
	if err != nil {
		return "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(line)
	if err != nil {
		return "", false
	}
	return string(decoded), true
}

func (s *session) data() bool {
	if s.from == "" || len(s.to) == 0 {
		return s.reply("503 5.5.1 MAIL and RCPT first")
	}
	if !s.reply("354 End data with <CR><LF>.<CR><LF>") {
		return false
	}
	var data strings.Builder
	for {
		line, err := s.readLine()
		if err != nil {
			return false
		}
		if line == "." {
			break
		}
		data.WriteString(strings.TrimPrefix(line, "."))
		data.WriteString("\r\n")
	}
	s.server.mu.Lock()
	s.server.messages = append(s.server.messages, Message{From: s.from, To: s.to, Data: data.String(), TLS: s.tls})
	s.server.mu.Unlock()
	s.from, s.to = "", nil
	return s.reply("250 2.0.0 OK: queued as " + strconv.Itoa(len(s.server.Messages())))
}

// addressIn takes the address out of a MAIL FROM:<a> or RCPT TO:<a> argument.
func addressIn(arg string) string {
	start, end := strings.Index(arg, "<"), strings.Index(arg, ">")
	if start < 0 || end < start {
		return ""
	}
	return arg[start+1 : end]
}
