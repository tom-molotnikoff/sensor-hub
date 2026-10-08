package mqtt

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/stretchr/testify/assert"
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

func TestEmbeddedBroker_LogsRefusedConnectionsAtDebugNotWarn(t *testing.T) {
	var logs syncBuffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	address := startAuthBrokerWith(t, BrokerConfig{TCPAddress: "127.0.0.1:0", ConnectRateLimit: 1}, logger)

	startOfNextSecond := time.Until(time.Now().Truncate(time.Second).Add(time.Second))
	time.Sleep(startOfNextSecond)
	connackCode(t, address, 5, "plug", "wrong")
	connackCode(t, address, 5, "plug", "right")

	// Mochi logs a connection it did not establish once the connection ends.
	assert.Eventually(t, func() bool {
		return strings.Contains(logs.String(), packets.ErrNotAuthorized.Reason) &&
			strings.Contains(logs.String(), packets.ErrConnectionRateExceeded.Reason)
	}, 5*time.Second, 10*time.Millisecond, "both refusals are logged")
	for _, line := range strings.Split(logs.String(), "\n") {
		assert.NotContains(t, line, "level=WARN", "a refused connection is not a warning")
	}
}
