package email

import (
	"context"
	"crypto/tls"
	"io"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"
	"time"

	"example/sensorHub/testharness/smtpfake"
	"example/sensorHub/testharness/testca"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testUser     = "hub@example.com"
	testPassword = "smtp-app-password"
)

func newCA(t *testing.T) *testca.CA {
	t.Helper()
	ca, err := testca.New("mail CA")
	require.NoError(t, err)
	return ca
}

// startServer starts a fake SMTP server that takes testUser and testPassword
// and, given a certificate, offers TLS with it.
func startServer(t *testing.T, cert *tls.Certificate, implicitTLS bool) *smtpfake.Server {
	t.Helper()
	opts := smtpfake.Options{Username: testUser, Password: testPassword, ImplicitTLS: implicitTLS}
	if cert != nil {
		opts.TLS = &tls.Config{Certificates: []tls.Certificate{*cert}}
	}
	server, err := smtpfake.Start(opts)
	require.NoError(t, err)
	t.Cleanup(server.Close)
	return server
}

func serverCertificate(t *testing.T, ca *testca.CA, hosts ...string) *tls.Certificate {
	t.Helper()
	cert, err := ca.ServerCertificate(hosts...)
	require.NoError(t, err)
	return &cert
}

func serverFor(fake *smtpfake.Server, security, password string) server {
	return server{host: fake.Host(), port: fake.Port(), security: security, username: testUser, password: password}
}

func sendOne(t transport, srv server) error {
	message := buildMessage("hub@example.com", "admin@example.com", "Subject", "Body", time.Now())
	return t.send(context.Background(), srv, "hub@example.com", "admin@example.com", message)
}

func TestSend_STARTTLSUpgradesAndVerifiesTheServer(t *testing.T) {
	ca := newCA(t)
	fake := startServer(t, serverCertificate(t, ca, "127.0.0.1"), false)

	require.NoError(t, sendOne(transport{rootCAs: ca.Pool()}, serverFor(fake, SecuritySTARTTLS, testPassword)))

	messages := fake.Messages()
	require.Len(t, messages, 1)
	assert.True(t, messages[0].TLS, "the message was sent after STARTTLS")
	assert.Equal(t, []string{"admin@example.com"}, messages[0].To)
}

func TestSend_STARTTLSRefusesAServerThatDoesNotOfferIt(t *testing.T) {
	fake := startServer(t, nil, false)

	err := sendOne(transport{}, serverFor(fake, SecuritySTARTTLS, testPassword))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not offer STARTTLS")
	assert.Zero(t, fake.AuthAttempts(), "the password was never sent in the clear")
	assert.Empty(t, fake.Messages())
}

func TestSend_TLSRefusesACertificateThatDoesNotVerify(t *testing.T) {
	ca := newCA(t)
	cases := map[string]struct {
		cert     *tls.Certificate
		roots    transport
		implicit bool
		security string
	}{
		"STARTTLS, not issued by a trusted CA":     {serverCertificate(t, ca, "127.0.0.1"), transport{}, false, SecuritySTARTTLS},
		"STARTTLS, issued for another host":        {serverCertificate(t, ca, "mail.example.com"), transport{rootCAs: ca.Pool()}, false, SecuritySTARTTLS},
		"implicit TLS, not issued by a trusted CA": {serverCertificate(t, ca, "127.0.0.1"), transport{}, true, SecurityImplicitTLS},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fake := startServer(t, tc.cert, tc.implicit)

			err := sendOne(tc.roots, serverFor(fake, tc.security, testPassword))

			require.Error(t, err)
			assert.Contains(t, err.Error(), "certificate")
			assert.Zero(t, fake.AuthAttempts(), "the password was never sent")
			assert.Empty(t, fake.Messages())
		})
	}
}

func TestSend_ImplicitTLSOpensTLSFirst(t *testing.T) {
	ca := newCA(t)
	fake := startServer(t, serverCertificate(t, ca, "127.0.0.1"), true)

	require.NoError(t, sendOne(transport{rootCAs: ca.Pool()}, serverFor(fake, SecurityImplicitTLS, testPassword)))

	messages := fake.Messages()
	require.Len(t, messages, 1)
	assert.True(t, messages[0].TLS)
}

func TestSend_NoneLogsInAndSendsInTheClear(t *testing.T) {
	fake := startServer(t, nil, false)

	require.NoError(t, sendOne(transport{}, serverFor(fake, SecurityNone, testPassword)))

	messages := fake.Messages()
	require.Len(t, messages, 1)
	assert.False(t, messages[0].TLS)
}

func TestSend_ReturnsTheServersErrorWhenItRefusesTheLogin(t *testing.T) {
	fake := startServer(t, nil, false)

	err := sendOne(transport{}, serverFor(fake, SecurityNone, "wrong-password"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "535 5.7.8 Authentication credentials invalid")
	assert.Empty(t, fake.Messages())
}

func TestLoginAuth_AnswersTheUsernameThenThePassword(t *testing.T) {
	auth := &loginAuth{username: testUser, password: testPassword}
	mechanism, initial, err := auth.Start(nil)
	require.NoError(t, err)
	assert.Equal(t, "LOGIN", mechanism)
	assert.Nil(t, initial)

	username, err := auth.Next([]byte("Username:"), true)
	require.NoError(t, err)
	password, err := auth.Next([]byte("Password:"), true)
	require.NoError(t, err)
	assert.Equal(t, testUser, string(username))
	assert.Equal(t, testPassword, string(password))
	_, err = auth.Next([]byte("Again:"), true)
	assert.Error(t, err)
}

func TestBuildMessage_KeepsHeadersOnOneLineAndTheBodyIntact(t *testing.T) {
	body := "Temperature in the Living Room is 31.5°C\nwhich is above 25°C.\n" + strings.Repeat("x", 200)
	raw := buildMessage("hub@example.com", "admin@example.com", "[threshold_alert] Hot\r\nBcc: victim@example.com", body, time.Now())

	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	require.NoError(t, err)
	assert.Empty(t, msg.Header.Get("Bcc"), "a line break in the subject cannot add a header")
	subject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	require.NoError(t, err)
	assert.Equal(t, "[threshold_alert] Hot Bcc: victim@example.com", subject)
	assert.Equal(t, "hub@example.com", msg.Header.Get("From"))
	assert.Contains(t, msg.Header.Get("Message-ID"), "@example.com>")

	decoded, err := io.ReadAll(quotedprintable.NewReader(msg.Body))
	require.NoError(t, err)
	assert.Equal(t, strings.ReplaceAll(body, "\n", "\r\n"), string(decoded))
}
