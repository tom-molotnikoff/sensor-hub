package api

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	appProps "example/sensorHub/application_properties"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitialiseAndListen_ServesMetricsOnlyOnTheMetricsAddress(t *testing.T) {
	cfg := &appProps.ApplicationConfiguration{
		HTTPListenAddress:    freeLoopbackAddress(t),
		MetricsListenAddress: freeLoopbackAddress(t),
	}
	prometheusHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("sensor_hub_metric 1\n"))
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- InitialiseAndListen(ctx, slog.Default(), cfg, prometheusHandler, nil) }()

	metrics := waitForGet(t, "http://"+cfg.MetricsListenAddress+"/metrics")
	assert.Equal(t, http.StatusOK, metrics.status)
	assert.Equal(t, "sensor_hub_metric 1\n", metrics.body)

	assert.Equal(t, http.StatusNotFound, waitForGet(t, "http://"+cfg.HTTPListenAddress+"/metrics").status)

	cancel()
	assert.NoError(t, <-done)
}

func TestNewEngine_NoTrustedProxiesIgnoresForwardingHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, err := NewEngine("")
	require.NoError(t, err)
	router.GET("/ip", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })

	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = "192.0.2.10:51234"
	req.Header.Set("X-Forwarded-For", "198.51.100.1")
	req.Header.Set("X-Real-IP", "198.51.100.2")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, "192.0.2.10", w.Body.String())
}

func freeLoopbackAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	return listener.Addr().String()
}

type getResult struct {
	status int
	body   string
}

// waitForGet retries until the server at url accepts connections.
func waitForGet(t *testing.T, url string) getResult {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := http.Get(url)
		if err == nil {
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			return getResult{status: resp.StatusCode, body: string(body)}
		}
		if time.Now().After(deadline) {
			t.Fatalf("GET %s: %v", url, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
