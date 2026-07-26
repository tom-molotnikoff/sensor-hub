//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProperties_GetAll(t *testing.T) {
	resp, status := client.GetProperties()
	require.Equal(t, http.StatusOK, status)
	assert.NotEmpty(t, resp)
}

func TestProperties_SetAndGet(t *testing.T) {
	status := client.SetProperty("sensor.collection.interval", "600")
	require.Equal(t, http.StatusAccepted, status)

	// Restore original value
	defer client.SetProperty("sensor.collection.interval", "300")

	resp, status := client.GetProperties()
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(resp), "600")
}

// Whatever a client writes is what it reads back. No value is reserved, and no
// value is rewritten on the way out.
func TestProperties_ValuesRoundTripVerbatim(t *testing.T) {
	original := readProperty(t, "smtp.user")
	defer client.SetProperty("smtp.user", original)

	for _, value := range []string{"probe@example.com", "*****", "Mixed Case Value"} {
		status := client.SetProperty("smtp.user", value)
		require.Equal(t, http.StatusAccepted, status)

		assert.Equal(t, value, readProperty(t, "smtp.user"))
	}
}

func readProperty(t *testing.T, key string) string {
	t.Helper()

	resp, status := client.GetProperties()
	require.Equal(t, http.StatusOK, status)

	var properties map[string]string
	require.NoError(t, json.Unmarshal(resp, &properties))

	return properties[key]
}
