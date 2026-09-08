//go:build integration

package integration

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWireFormat_LargeResponseIsCompressed(t *testing.T) {
	resp, body, err := client.GetRaw("/api/properties/definitions", http.Header{"Accept-Encoding": []string{"gzip"}})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "gzip", resp.Header.Get("Content-Encoding"))

	reader, err := gzip.NewReader(bytes.NewReader(body))
	require.NoError(t, err)
	decompressed, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Greater(t, len(decompressed), 1024)
	assert.True(t, json.Valid(decompressed))
}

func TestWireFormat_JsonIsNotIndented(t *testing.T) {
	resp, body, err := client.GetRaw("/api/properties/definitions", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.True(t, json.Valid(body))

	var compact bytes.Buffer
	require.NoError(t, json.Compact(&compact, body))
	assert.Equal(t, compact.String(), string(bytes.TrimSpace(body)))
}
