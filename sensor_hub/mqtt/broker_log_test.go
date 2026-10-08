package mqtt

import (
	"bytes"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncBuffer is a bytes.Buffer that the broker's goroutines can log to while
// the test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// startLoggedBroker starts an auth broker with a CONNECT rate limit of 1,
// logging at debug into the returned buffer.
func startLoggedBroker(t *testing.T) (string, *syncBuffer) {
	t.Helper()
	logs := &syncBuffer{}
	logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return startAuthBrokerWith(t, BrokerConfig{TCPAddress: "127.0.0.1:0", ConnectRateLimit: 1}, logger), logs
}

func linesAt(logs *syncBuffer, level string) []string {
	var lines []string
	for _, line := range strings.Split(logs.String(), "\n") {
		if strings.Contains(line, "level="+level) {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestEmbeddedBroker_LogsConnectionsRefusedBeforeAuthenticationAtDebug(t *testing.T) {
	address, logs := startLoggedBroker(t)

	time.Sleep(time.Until(time.Now().Truncate(time.Second).Add(time.Second)))
	connackCode(t, address, 5, "plug", "wrong")
	connackCode(t, address, 5, "plug", "right")

	// The connection's end is logged once mochi has finished with it.
	assert.Eventually(t, func() bool {
		debug := strings.Join(linesAt(logs, "DEBUG"), "\n")
		return strings.Contains(debug, `msg="MQTT connection ended before it authenticated"`) &&
			strings.Contains(debug, packets.ErrNotAuthorized.Reason) &&
			strings.Contains(debug, packets.ErrConnectionRateExceeded.Reason)
	}, 5*time.Second, 10*time.Millisecond, "both refusals are logged at debug")
	assert.Empty(t, linesAt(logs, "WARN"), "a refused connection is not a warning")
}

func TestEmbeddedBroker_LogsAnAuthenticatedConnectionsErrorAtWarn(t *testing.T) {
	address, logs := startLoggedBroker(t)

	conn, code := dialAndConnect(t, address, 5, "plug", "right")
	require.Equal(t, packets.CodeSuccess.Code, code)
	// A device dropping without a DISCONNECT ends its connection in a read
	// error, which the listener logs.
	require.NoError(t, conn.Close())

	assert.Eventually(t, func() bool {
		return strings.Contains(strings.Join(linesAt(logs, "WARN"), "\n"), "listener=sensor-hub-tcp error=")
	}, 5*time.Second, 10*time.Millisecond, "an authenticated connection ending in an error is a warning")
}

// dialAndConnect sends a CONNECT and returns the open connection with the
// CONNACK's code, for a test that goes on using the connection.
func dialAndConnect(t *testing.T, address string, version byte, username, password string) (net.Conn, byte) {
	t.Helper()
	conn, err := net.Dial("tcp", address)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn, sendConnect(t, conn, version, username, password)
}
