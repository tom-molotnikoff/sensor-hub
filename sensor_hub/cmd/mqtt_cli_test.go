package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// brokerAPI records the JSON bodies written to it and answers GET with a
// broker as the hub returns one: a password status and no password.
func brokerAPI(t *testing.T) (*httptest.Server, *[]map[string]any) {
	t.Helper()
	var written []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":7,"name":"home","type":"external","host":"mqtt.lan","port":1883,` +
				`"username":"hub","password_status":"set","enabled":false}`))
			return
		}
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		written = append(written, body)
		_, _ = w.Write([]byte(`{"id":7}`))
	}))
	t.Cleanup(server.Close)
	return server, &written
}

func withStdin(t *testing.T, input string) {
	t.Helper()
	rootCmd.SetIn(strings.NewReader(input))
	t.Cleanup(func() { rootCmd.SetIn(os.Stdin) })
}

func TestMQTTBrokersCreate_TakesNoPasswordFlag(t *testing.T) {
	_, _, err := executeRootCommand(t, "mqtt", "brokers", "create", "--name", "home", "--host", "mqtt.lan",
		"--username", "hub", "--password", "secret-pass")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown flag")
}

func TestMQTTBrokersCreate_ReadsThePasswordFromStdin(t *testing.T) {
	server, written := brokerAPI(t)
	withStdin(t, "broker-secret\n")

	_, _, err := executeRootCommand(t, "--server", server.URL, "mqtt", "brokers", "create",
		"--name", "home", "--host", "mqtt.lan", "--username", "hub", "--password-stdin")

	require.NoError(t, err)
	require.Len(t, *written, 1)
	assert.Equal(t, "hub", (*written)[0]["username"])
	assert.Equal(t, "broker-secret", (*written)[0]["password"])
}

func TestMQTTBrokersCreate_AsksForNoPasswordWithoutAUsername(t *testing.T) {
	server, written := brokerAPI(t)
	withStdin(t, "")

	_, _, err := executeRootCommand(t, "--server", server.URL, "mqtt", "brokers", "create",
		"--name", "home", "--host", "mqtt.lan", "--username", "", "--password-stdin=false")

	require.NoError(t, err)
	require.Len(t, *written, 1)
	assert.NotContains(t, (*written)[0], "password")
	assert.NotContains(t, (*written)[0], "username")
}

func TestMQTTBrokersEnableAndDisable_KeepTheUsernameAndPassword(t *testing.T) {
	for verb, want := range map[string]bool{"enable": true, "disable": false} {
		t.Run(verb, func(t *testing.T) {
			server, written := brokerAPI(t)

			_, _, err := executeRootCommand(t, "--server", server.URL, "mqtt", "brokers", verb, "7")

			require.NoError(t, err)
			require.Len(t, *written, 1)
			body := (*written)[0]
			assert.Equal(t, want, body["enabled"])
			assert.Equal(t, "hub", body["username"])
			assert.NotContains(t, body, "password", "an omitted password keeps the stored one")
		})
	}
}

func TestMQTTBrokersCreate_AnEmptyPasswordOnStdinCreatesTheBrokerWithoutOne(t *testing.T) {
	server, written := brokerAPI(t)
	withStdin(t, "\n")

	_, _, err := executeRootCommand(t, "--server", server.URL, "mqtt", "brokers", "create",
		"--name", "home", "--host", "mqtt.lan", "--username", "hub", "--password-stdin")

	require.NoError(t, err)
	require.Len(t, *written, 1)
	assert.Equal(t, "hub", (*written)[0]["username"])
	assert.NotContains(t, (*written)[0], "password")
}

func TestMQTTBrokersCreate_RefusesToPromptWithoutATerminal(t *testing.T) {
	server, written := brokerAPI(t)
	withStdin(t, "piped-but-not-asked-for\n")

	_, _, err := executeRootCommand(t, "--server", server.URL, "mqtt", "brokers", "create",
		"--name", "home", "--host", "mqtt.lan", "--username", "hub", "--password-stdin=false")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--password-stdin")
	assert.Empty(t, *written, "nothing is created")
}
