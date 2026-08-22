//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// An external edit to a properties file (SSH, upgrade) must reach every open
// properties websocket subscriber without a page reload.
func TestProperties_ExternalFileEditReachesWebSocketSubscribers(t *testing.T) {
	original := readProperty(t, "sensor.collection.interval")
	defer client.SetProperty("sensor.collection.interval", original)

	path := filepath.Join(env.ConfigDir, "application.properties")

	// The watcher suppresses changes made while the app is writing or cooling
	// down after its own write, and an earlier test's async file write can
	// clobber the edit, so rewrite until the broadcast arrives - as a real
	// operator would retry an edit that did not take. Each attempt dials a
	// fresh connection: a read deadline kills a gorilla websocket conn, and
	// the handler pushes current values on connect, so an edit that landed
	// between attempts is still observed.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		editPropertyFile(t, path, "sensor.collection.interval", "444")

		if propertyArrivesOverWebSocket(t, "sensor.collection.interval", "444", 4*time.Second) {
			return
		}
	}

	t.Fatal("external file edit never reached the websocket subscriber")
}

// editPropertyFile rewrites key=value in a properties file in place.
func editPropertyFile(t *testing.T, path, key, value string) {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, key+"=") {
			lines[i] = key + "=" + value
			found = true
		}
	}
	if !found {
		lines = append(lines, key+"="+value)
	}
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644))
}

// propertyArrivesOverWebSocket subscribes to the properties websocket and
// reads messages until one carries key=value or the timeout passes.
func propertyArrivesOverWebSocket(t *testing.T, key, value string, timeout time.Duration) bool {
	t.Helper()

	conn, resp, err := client.DialWebSocket("/api/properties/ws")
	if resp != nil {
		defer resp.Body.Close()
	}
	require.NoError(t, err)
	defer conn.Close()

	deadline := time.Now().Add(timeout)
	require.NoError(t, conn.SetReadDeadline(deadline))

	for time.Now().Before(deadline) {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return false
		}

		var properties map[string]string
		if json.Unmarshal(payload, &properties) != nil {
			continue
		}
		if properties[key] == value {
			return true
		}
	}
	return false
}

// The definitions endpoint and the values endpoint are two sources of truth
// for the same page; on a real instance they must agree on the key set.
func TestProperties_DefinitionsCoverEveryValue(t *testing.T) {
	defs, status := client.GetPropertyDefinitions()
	require.Equal(t, http.StatusOK, status)

	assert.Len(t, defs.Definitions, 29)
	assert.Len(t, defs.Groups, 7)

	resp, status := client.GetProperties()
	require.Equal(t, http.StatusOK, status)

	var values map[string]string
	require.NoError(t, json.Unmarshal(resp, &values))

	defKeys := make(map[string]bool)
	for _, d := range defs.Definitions {
		defKeys[d.Key] = true
	}

	for key := range values {
		assert.True(t, defKeys[key], "value %q has no definition", key)
	}
	for key := range defKeys {
		_, ok := values[key]
		assert.True(t, ok, "definition %q has no value", key)
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
