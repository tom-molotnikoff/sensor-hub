package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func compressionTestRouter(body string) *gin.Engine {
	router := gin.New()
	router.Use(Compression())
	router.GET("/test", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, body)
	})
	return router
}

func TestCompression_CompressesResponse(t *testing.T) {
	body := strings.Repeat("a", 2048)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	compressionTestRouter(body).ServeHTTP(w, req)

	assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))
	assert.Less(t, w.Body.Len(), len(body))

	reader, err := gzip.NewReader(w.Body)
	require.NoError(t, err)
	decompressed, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, body, string(decompressed))
}

func TestCompression_LeavesWebSocketUpgradeAlone(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	w := httptest.NewRecorder()

	body := strings.Repeat("a", 2048)
	compressionTestRouter(body).ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Content-Encoding"))
	assert.Equal(t, body, w.Body.String())
}

func TestCompression_LeavesUnacceptingClientAlone(t *testing.T) {
	body := strings.Repeat("a", 2048)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	compressionTestRouter(body).ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Content-Encoding"))
	assert.Equal(t, body, w.Body.String())
}
