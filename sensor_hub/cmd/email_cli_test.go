package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// emailAPI answers GET with stored settings as the hub returns them, a
// password status and no password, and records the bodies written to it.
func emailAPI(t *testing.T) (*httptest.Server, *[]map[string]any) {
	t.Helper()
	var written []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"host":"smtp.gmail.com","port":587,"security":"starttls","username":"me@example.com",` +
				`"from_address":"me@example.com","password_status":"set","last_error":null,"last_sent_at":null}`))
			return
		}
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		written = append(written, body)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	return server, &written
}

func TestEmailSet_TakesNoPasswordFlag(t *testing.T) {
	_, _, err := executeRootCommand(t, "email", "set", "--password", "secret")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown flag")
}

func TestEmailSet_ChangesOnlyTheGivenSettingsAndReadsThePasswordFromStdin(t *testing.T) {
	server, written := emailAPI(t)
	withStdin(t, "smtp-secret\n")

	_, _, err := executeRootCommand(t, "--server", server.URL, "email", "set",
		"--host", "email-smtp.eu-west-1.amazonaws.com", "--port", "465", "--security", "implicit_tls", "--password-stdin")

	require.NoError(t, err)
	require.Len(t, *written, 1)
	assert.Equal(t, map[string]any{
		"host":         "email-smtp.eu-west-1.amazonaws.com",
		"port":         float64(465),
		"security":     "implicit_tls",
		"username":     "me@example.com",
		"from_address": "me@example.com",
		"password":     "smtp-secret",
	}, (*written)[0])
}

func TestEmailSet_KeepsTheStoredPasswordWithoutATerminal(t *testing.T) {
	server, written := emailAPI(t)
	withStdin(t, "")

	_, _, err := executeRootCommand(t, "--server", server.URL, "email", "set", "--from-address", "alerts@example.com")

	require.NoError(t, err)
	require.Len(t, *written, 1)
	assert.Equal(t, "alerts@example.com", (*written)[0]["from_address"])
	assert.NotContains(t, (*written)[0], "password")
}

func TestEmailSet_RefusesAnEmptyPasswordOnStdin(t *testing.T) {
	server, written := emailAPI(t)
	withStdin(t, "\n")

	_, _, err := executeRootCommand(t, "--server", server.URL, "email", "set", "--password-stdin")

	require.Error(t, err)
	assert.Empty(t, *written)
}
