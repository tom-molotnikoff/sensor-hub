package email

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The ways a connection to the SMTP server is protected.
const (
	SecuritySTARTTLS    = "starttls"
	SecurityImplicitTLS = "implicit_tls"
	SecurityNone        = "none"
)

const (
	dialTimeout = 15 * time.Second
	// sendTimeout bounds a whole send, so a server that stops answering
	// cannot hold a sender for ever.
	sendTimeout = 60 * time.Second
)

// server is where and how a message is sent, with the password to log in.
type server struct {
	host     string
	port     int
	security string
	username string
	password string
}

// transport delivers messages over SMTP.
type transport struct {
	// rootCAs verifies the server's certificate. Nil means the system's
	// trusted roots, which is what the hub uses; tests trust their own CA.
	rootCAs *x509.CertPool
}

// send delivers one message to one recipient. With starttls it refuses to go
// on unless the server offers STARTTLS, and with either TLS mode it verifies
// the certificate before anything else, the password included, is sent.
func (t transport) send(ctx context.Context, srv server, from, to string, message []byte) error {
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()

	address := net.JoinHostPort(srv.host, strconv.Itoa(srv.port))
	tlsConfig := &tls.Config{ServerName: srv.host, RootCAs: t.rootCAs, MinVersion: tls.VersionTLS12}
	dialer := &net.Dialer{Timeout: dialTimeout}
	var conn net.Conn
	var err error
	if srv.security == SecurityImplicitTLS {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsConfig}).DialContext(ctx, "tcp", address)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return fmt.Errorf("could not connect to %s: %w", address, err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	client, err := smtp.NewClient(conn, srv.host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("the SMTP server at %s did not greet: %w", address, err)
	}
	defer client.Close()

	if srv.security == SecuritySTARTTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("the SMTP server does not offer STARTTLS, so nothing was sent; " +
				"use implicit_tls if it takes TLS from the start")
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("STARTTLS failed: %w", err)
		}
	}
	if srv.username != "" {
		auth, err := authFor(client, srv.username, srv.password)
		if err != nil {
			return err
		}
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("the SMTP server refused the login: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("the SMTP server refused the sender %s: %w", from, err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("the SMTP server refused the recipient %s: %w", to, err)
	}
	data, err := client.Data()
	if err != nil {
		return fmt.Errorf("the SMTP server refused the message: %w", err)
	}
	if _, err := data.Write(message); err != nil {
		_ = data.Close()
		return fmt.Errorf("could not send the message: %w", err)
	}
	if err := data.Close(); err != nil {
		return fmt.Errorf("the SMTP server refused the message: %w", err)
	}
	// The message is accepted once DATA ends; a failed QUIT changes nothing.
	_ = client.Quit()
	return nil
}

// authFor picks the login mechanism: PLAIN where the server offers it, which
// every common provider does, otherwise LOGIN.
func authFor(client *smtp.Client, username, password string) (smtp.Auth, error) {
	ok, offered := client.Extension("AUTH")
	if !ok {
		return nil, errors.New("the SMTP server does not offer a login, but a username is set")
	}
	mechanisms := strings.Fields(strings.ToUpper(offered))
	switch {
	case slices.Contains(mechanisms, "PLAIN"):
		return plainAuth{username: username, password: password}, nil
	case slices.Contains(mechanisms, "LOGIN"):
		return &loginAuth{username: username, password: password}, nil
	}
	return nil, fmt.Errorf("the SMTP server offers no login the hub supports (it offers %s; the hub uses PLAIN or LOGIN)", offered)
}

// plainAuth is AUTH PLAIN. net/smtp's own refuses to log in over an
// unencrypted connection to anywhere but localhost; the security setting is
// what decides that here, and with none the operator has chosen it.
type plainAuth struct {
	username, password string
}

func (a plainAuth) Start(*smtp.ServerInfo) (string, []byte, error) {
	return "PLAIN", []byte("\x00" + a.username + "\x00" + a.password), nil
}

func (a plainAuth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return nil, errors.New("the SMTP server sent an unexpected challenge during PLAIN login")
	}
	return nil, nil
}

// loginAuth is AUTH LOGIN: the server prompts for the username, then the
// password.
type loginAuth struct {
	username, password string
	step               int
}

func (a *loginAuth) Start(*smtp.ServerInfo) (string, []byte, error) {
	a.step = 0
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(_ []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	a.step++
	switch a.step {
	case 1:
		return []byte(a.username), nil
	case 2:
		return []byte(a.password), nil
	}
	return nil, errors.New("the SMTP server sent an unexpected challenge during LOGIN")
}
