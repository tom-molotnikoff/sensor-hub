// Package secrets holds the credentials the hub presents to other systems,
// such as outbound MQTT broker passwords. Each is encrypted with AES-256-GCM
// under a key kept outside the database, so a copy of the database or a
// backup gives nothing usable.
package secrets

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
)

// KeySize is the length of the key in bytes.
const KeySize = 32

// Key is the secret-store key. It never prints or logs its bytes; Encode is
// the only way to get them out.
type Key struct {
	bytes [KeySize]byte
}

func GenerateKey() (Key, error) {
	var k Key
	if _, err := rand.Read(k.bytes[:]); err != nil {
		return Key{}, fmt.Errorf("failed to generate a secret-store key: %w", err)
	}
	return k, nil
}

// ParseKey reads a key file's content: 32 bytes as one line of standard
// base64, with or without the trailing newline.
func ParseKey(text []byte) (Key, error) {
	line := bytes.TrimSuffix(bytes.TrimSuffix(text, []byte("\n")), []byte("\r"))
	if bytes.ContainsAny(line, "\r\n") {
		return Key{}, errors.New("the key must be one line of base64")
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(string(line))
	if err != nil {
		return Key{}, fmt.Errorf("the key is not valid base64: %w", err)
	}
	if len(raw) != KeySize {
		return Key{}, fmt.Errorf("the key must be %d bytes, got %d", KeySize, len(raw))
	}
	var k Key
	copy(k.bytes[:], raw)
	return k, nil
}

// Encode gives the key as a key file holds it: one line of standard base64.
func (k Key) Encode() string {
	return base64.StdEncoding.EncodeToString(k.bytes[:])
}

func (k Key) String() string { return "[secret-store key]" }

func (k Key) GoString() string { return k.String() }

func (k Key) LogValue() slog.Value { return slog.StringValue(k.String()) }
