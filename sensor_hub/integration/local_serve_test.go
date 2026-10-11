//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	database "example/sensorHub/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runningHub is a 'local serve' process started by a test.
type runningHub struct {
	cmd    *exec.Cmd
	output *lockedBuffer
}

// startHub runs 'local serve' as the packaged unit does and waits until it
// answers. The hub is stopped at the end of the test, or sooner with stop.
// Its output is written by the process's copying goroutines while the test
// reads it, hence the lock.
func startHub(t *testing.T, configDir, httpAddress string) *runningHub {
	t.Helper()
	hub := &runningHub{output: &lockedBuffer{}}
	hub.cmd = exec.Command(buildSensorHub(t), "local", "serve", "--config-dir", configDir)
	hub.cmd.Stdout, hub.cmd.Stderr = hub.output, hub.output
	require.NoError(t, hub.cmd.Start())
	t.Cleanup(hub.stop)

	healthURL := fmt.Sprintf("http://%s/api/health", httpAddress)
	require.Eventually(t, func() bool {
		resp, err := http.Get(healthURL)
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 60*time.Second, 200*time.Millisecond, "the hub did not come up:\n%s", hub.output)
	return hub
}

// stop shuts the hub down as systemd does and waits for it to exit.
func (h *runningHub) stop() {
	if h.cmd.ProcessState != nil {
		return
	}
	_ = h.cmd.Process.Signal(syscall.SIGTERM)
	_ = h.cmd.Wait()
}

func TestLocalServe_LeavesFilesAndKeysItHasNoUseForAlone(t *testing.T) {
	configDir, dbPath := freshConfigDir(t)
	httpAddress := freeLocalAddress(t)
	properties, err := os.OpenFile(filepath.Join(configDir, "application.properties"), os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = fmt.Fprintf(properties, "http.listen.address=%s\nmetrics.listen.address=\nmqtt.broker.enabled=false\n"+
		"oauth.credentials.file.path=credentials.json\noauth.token.file.path=token.json\noauth.token.refresh.interval.minutes=30\n", httpAddress)
	require.NoError(t, err)
	require.NoError(t, properties.Close())
	files := map[string]string{
		"smtp.properties":  "smtp.user=alerts@example.com\n",
		"credentials.json": `{"installed":{"client_id":"1.apps.googleusercontent.com","client_secret":"client-secret"}}`,
		"token.json":       `{"access_token":"access","refresh_token":"refresh"}`,
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(configDir, name), []byte(content), 0o640))
	}

	hub := startHub(t, configDir, httpAddress)
	hub.stop()

	output := hub.output.String()
	for name, content := range files {
		kept, err := os.ReadFile(filepath.Join(configDir, name))
		require.NoError(t, err)
		assert.Equal(t, content, string(kept))
		assert.NotContains(t, output, name)
	}
	readOnly := openSQLite(t, dbPath)
	settings, err := database.NewEmailSettingsRepository(&database.Handles{Reader: readOnly, Writer: readOnly}).Get(context.Background())
	require.NoError(t, err)
	assert.Empty(t, settings.Username)
	assert.Empty(t, settings.FromAddress)
}
