package mqtt

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/stretchr/testify/assert"
)

func TestConnectRateHook_AllowsTheLimitEachSecond(t *testing.T) {
	now := time.Unix(1_000, 0)
	hook := newConnectRateHook(nil, 2, slog.New(slog.NewTextHandler(io.Discard, nil)))
	hook.now = func() time.Time { return now }

	assert.True(t, hook.allow())
	assert.True(t, hook.allow())
	assert.False(t, hook.allow(), "the third CONNECT in a second is over a limit of 2")
	now = now.Add(999 * time.Millisecond)
	assert.False(t, hook.allow(), "still the same second")

	now = now.Add(time.Millisecond)
	assert.True(t, hook.allow(), "a new second starts the count again")
	assert.True(t, hook.allow())
	assert.False(t, hook.allow())
}

func TestConnectRateHook_RefusesBeforeAuthenticationAndCountsIt(t *testing.T) {
	reader := recordMetrics(t)
	address := startAuthBrokerWith(t, BrokerConfig{TCPAddress: "127.0.0.1:0", ConnectRateLimit: 1})

	// Start at the top of a second, so the CONNECTs below share one.
	time.Sleep(time.Until(time.Now().Truncate(time.Second).Add(time.Second)))
	assert.Equal(t, packets.CodeSuccess.Code, connackCode(t, address, 5, "plug", "right"))
	assert.Equal(t, packets.ErrConnectionRateExceeded.Code, connackCode(t, address, 5, "plug", "wrong"))
	assert.Equal(t, packets.Err3ServerUnavailable.Code, connackCode(t, address, 4, "plug", "wrong"))

	assert.Equal(t, map[string]int64{"rate_limit": 2}, refusalCounts(t, reader),
		"refused CONNECTs are never authenticated, so none counts as an auth refusal")
}
