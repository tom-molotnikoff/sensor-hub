package email

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"mime"
	"mime/quotedprintable"
	"strings"
	"time"
)

// buildMessage writes a plain-text message. Header values are stripped of
// line breaks, so text from a sensor name or an alert cannot add a header,
// and a subject outside ASCII is encoded. The body is quoted-printable, so
// any text and any line length survive the trip.
func buildMessage(from, to, subject, body string, now time.Time) []byte {
	var b bytes.Buffer
	header := func(name, value string) {
		b.WriteString(name + ": " + value + "\r\n")
	}
	header("From", headerValue(from))
	header("To", headerValue(to))
	header("Subject", mime.QEncoding.Encode("utf-8", headerValue(subject)))
	header("Date", now.Format(time.RFC1123Z))
	header("Message-ID", messageID(from))
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=utf-8")
	header("Content-Transfer-Encoding", "quoted-printable")
	b.WriteString("\r\n")

	qp := quotedprintable.NewWriter(&b)
	_, _ = qp.Write([]byte(body))
	_ = qp.Close()
	return b.Bytes()
}

func headerValue(value string) string {
	return strings.Join(strings.FieldsFunc(value, func(r rune) bool { return r == '\r' || r == '\n' }), " ")
}

// messageID makes a unique Message-ID under the sender's domain, which some
// receivers want before they accept a message.
func messageID(from string) string {
	domain := "sensor-hub.invalid"
	if _, d, ok := strings.Cut(from, "@"); ok && d != "" {
		domain = headerValue(d)
	}
	random := make([]byte, 16)
	_, _ = rand.Read(random)
	return "<" + hex.EncodeToString(random) + "@" + domain + ">"
}
