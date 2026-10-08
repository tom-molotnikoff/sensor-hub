package mqtt

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"time"

	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// connectRateHook sheds a CONNECT flood before any credential is checked. It
// counts CONNECTs across every client in the current second and refuses the
// ones past the limit. It keeps nothing about who connected, so a flood that
// uses the home device's username cannot lock the device out, and a refusal
// never outlasts the second it happened in.
type connectRateHook struct {
	mochi.HookBase
	server  *mochi.Server
	limit   int
	refused metric.Int64Counter
	logger  *slog.Logger
	now     func() time.Time

	mu     sync.Mutex
	second int64 // Unix second the count belongs to
	count  int
}

func newConnectRateHook(server *mochi.Server, limit int, logger *slog.Logger) *connectRateHook {
	return &connectRateHook{server: server, limit: limit, refused: connectRefusedCounter(), logger: logger, now: time.Now}
}

func (h *connectRateHook) ID() string {
	return "sensor-hub-connect-rate"
}

func (h *connectRateHook) Provides(b byte) bool {
	return bytes.Contains([]byte{mochi.OnConnect}, []byte{b})
}

// OnConnect refuses the CONNECT when the limit for this second is used up.
// Mochi runs OnConnect hooks in the order they were added, and this one is
// added before the client auth hook, so a refused CONNECT is never
// authenticated.
func (h *connectRateHook) OnConnect(cl *mochi.Client, pk packets.Packet) error {
	if h.allow() {
		return nil
	}
	h.refused.Add(context.Background(), 1, metric.WithAttributes(attribute.String("reason", "rate_limit")))
	h.logger.Debug("refused MQTT CONNECT over the rate limit", "client_id", cl.ID, "remote", cl.Net.Remote, "limit", h.limit)
	if err := h.server.SendConnack(cl, connectionRateExceeded(cl), false, nil); err != nil {
		h.logger.Debug("failed to send MQTT CONNACK refusal", "client_id", cl.ID, "error", err)
	}
	return packets.ErrConnectionRateExceeded
}

// allow counts a CONNECT against the current second and reports whether it
// is within the limit. The count starts again at each new second.
func (h *connectRateHook) allow() bool {
	second := h.now().Unix()
	h.mu.Lock()
	defer h.mu.Unlock()
	if second != h.second {
		h.second = second
		h.count = 0
	}
	h.count++
	return h.count <= h.limit
}

// connectionRateExceeded is the CONNACK code for a CONNECT over the rate
// limit: 0x9F on v5. v3.1.1 has no such code, so it gets server unavailable,
// which mochi rewrites to 0x03 on v3.1.1 and which tells the client to retry.
func connectionRateExceeded(cl *mochi.Client) packets.Code {
	if cl.Properties.ProtocolVersion < 5 {
		return packets.ErrServerUnavailable
	}
	return packets.ErrConnectionRateExceeded
}
