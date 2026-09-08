package api

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRouter_MetricsIsNotCompressed(t *testing.T) {
	metrics := strings.Repeat("sensor_hub_metric 1\n", 200)
	prometheusHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(metrics))
	})
	router := newRouter(slog.Default(), prometheusHandler, nil)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Content-Encoding"))
	assert.Equal(t, metrics, w.Body.String())
}
