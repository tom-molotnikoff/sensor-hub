package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const lampTimerJSON = `{"name":"Lamp timer","triggers":[{"type":"schedule","at":"19:00","days":["mon"]}],` +
	`"steps":[{"type":"set","sensor_id":14,"property":"state","value":"ON"}]}`

func automationBodyServer(t *testing.T, method, path string, got *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, method, r.Method)
		require.Equal(t, path, r.URL.Path)
		data, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		*got = string(data)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":3}`))
	}))
}

func TestAutomationsCreateCommand_SendsJSONFromStdinUnchanged(t *testing.T) {
	var got string
	server := automationBodyServer(t, http.MethodPost, "/api/automations", &got)
	defer server.Close()

	rootCmd.SetIn(strings.NewReader(lampTimerJSON))
	t.Cleanup(func() { rootCmd.SetIn(nil) })

	_, _, err := executeRootCommand(t, "--server", server.URL, "automations", "create", "--file", "-")
	require.NoError(t, err)
	assert.JSONEq(t, lampTimerJSON, got)
}

func TestAutomationsUpdateCommand_SendsJSONFromFileUnchanged(t *testing.T) {
	var got string
	server := automationBodyServer(t, http.MethodPut, "/api/automations/3", &got)
	defer server.Close()

	path := filepath.Join(t.TempDir(), "lamp-timer.json")
	require.NoError(t, os.WriteFile(path, []byte(lampTimerJSON), 0o600))

	_, _, err := executeRootCommand(t, "--server", server.URL, "automations", "update", "3", "--file", path)
	require.NoError(t, err)
	assert.JSONEq(t, lampTimerJSON, got)
}

func TestAutomationsHelp_ListsEverySubcommand(t *testing.T) {
	for _, sub := range []string{"list", "get", "create", "update", "delete", "enable", "disable", "run", "cancel", "runs", "margin-suggestion"} {
		t.Run(sub, func(t *testing.T) {
			help, _, err := executeRootCommand(t, "automations", sub, "--help")
			require.NoError(t, err)
			assert.Contains(t, help, "sensor-hub automations "+sub)
		})
	}
}
