//go:build integration

package integration

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"

	appProps "example/sensorHub/application_properties"
	"example/sensorHub/testharness"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The harness trusts loopback as a proxy, as a packaged install trusts nginx,
// so the test client plays nginx: X-Forwarded-For carries whatever the client
// sent followed by the address nginx saw it connect from.
func TestTrustedProxy_LoginBackoffKeysOnTheRightmostUntrustedAddress(t *testing.T) {
	const clientAddr = "203.0.113.7"
	c := testharness.NewClient(t, env.ServerURL)
	forwardedFor := func(chain ...string) http.Header {
		return http.Header{"X-Forwarded-For": {strings.Join(chain, ", ")}}
	}

	threshold := appProps.AppConfig().AuthLoginBackoffThreshold
	for i := range threshold {
		forged := fmt.Sprintf("198.51.100.%d", i+1)
		status := c.LoginWithHeaders(fmt.Sprintf("proxy-backoff-%d", i), "wrong", forwardedFor(forged, clientAddr))
		require.Equal(t, http.StatusUnauthorized, status)
	}

	status := c.LoginWithHeaders("proxy-backoff-next", "wrong", forwardedFor("198.51.100.200", clientAddr))
	assert.Equal(t, http.StatusTooManyRequests, status, "a new forged address does not escape the backoff on the client's real one")

	status = c.LoginWithHeaders("proxy-backoff-other", "wrong", forwardedFor(clientAddr, "203.0.113.8"))
	assert.Equal(t, http.StatusUnauthorized, status, "forging the blocked address does not put another client under its backoff")
}

func TestWebSocket_RefusesAnOriginFromAnotherSite(t *testing.T) {
	t.Setenv("SENSOR_HUB_ALLOWED_ORIGIN", "http://ui.example:3000")
	serverHost := strings.TrimPrefix(env.ServerURL, "http://")

	cases := []struct {
		name   string
		origin string
		want   int
	}{
		{name: "no origin", origin: "", want: http.StatusSwitchingProtocols},
		{name: "the request's own host", origin: "http://" + serverHost, want: http.StatusSwitchingProtocols},
		{name: "the allowed origin", origin: "http://ui.example:3000", want: http.StatusSwitchingProtocols},
		{name: "another site", origin: "https://evil.example", want: http.StatusForbidden},
		{name: "the allowed host on another port", origin: "http://ui.example:4000", want: http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			headers := http.Header{}
			if tc.origin != "" {
				headers.Set("Origin", tc.origin)
			}
			conn, resp, err := client.DialWebSocketWithHeaders("/api/readings/ws/current", headers)
			if conn != nil {
				conn.Close()
			}
			if tc.want == http.StatusSwitchingProtocols {
				require.NoError(t, err)
			}
			require.NotNil(t, resp, "dial error: %v", err)
			assert.Equal(t, tc.want, resp.StatusCode)
		})
	}
}

func TestWebSocket_UpgradeLogLeavesOutTheSessionCookie(t *testing.T) {
	logs := &lockedBuffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	conn, _, err := client.DialWebSocket("/api/readings/ws/current")
	require.NoError(t, err)
	conn.Close()

	output := logs.String()
	require.Contains(t, output, "WebSocket upgrade request")
	cookies := client.Cookies()
	require.NotEmpty(t, cookies)
	for _, cookie := range cookies {
		assert.NotContains(t, output, cookie.Value, "cookie %s is in the log", cookie.Name)
	}
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "WebSocket") {
			assert.NotContains(t, strings.ToLower(line), "cookie")
		}
	}
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
