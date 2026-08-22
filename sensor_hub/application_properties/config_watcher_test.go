package appProps

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validAppPropsContent = "sensor.collection.interval=300\nsensor.discovery.skip=true\n"

// setupWatcherConfigDir points the config engine at a temp directory holding
// valid property files, and restores the previous paths and config afterwards.
func setupWatcherConfigDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "application.properties"), []byte(validAppPropsContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "smtp.properties"), []byte("smtp.user=\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "database.properties"), []byte("database.path=data/test.db\n"), 0644))

	oldDir := GetConfigDir()
	oldCfg := AppConfig()
	t.Cleanup(func() {
		setConfigPaths(oldDir)
		SetAppConfig(oldCfg)
	})

	// A write from an earlier test must not put this test inside the cooldown.
	lastWriteCompletedAt.Store(0)

	require.NoError(t, InitialiseConfig(dir))
	return dir
}

// startFastWatcher runs the watcher on a short interval so tests observe a
// tick without real-time waits, and stops it when the test ends.
func startFastWatcher(t *testing.T, onReload func()) {
	t.Helper()

	oldInterval := watchInterval
	watchInterval = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		watchInterval = oldInterval
	})

	WatchConfigFiles(ctx, onReload)
}

// notify signals without blocking, so an unexpected extra reload can never
// wedge the watcher goroutine past test cleanup.
func notify(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func TestWatchConfigFiles_ExternalEditReloadsAndNotifies(t *testing.T) {
	dir := setupWatcherConfigDir(t)

	notified := make(chan struct{}, 1)
	startFastWatcher(t, func() { notify(notified) })

	edited := "sensor.collection.interval=123\nsensor.discovery.skip=true\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "application.properties"), []byte(edited), 0644))

	select {
	case <-notified:
	case <-time.After(2 * time.Second):
		t.Fatal("no reload notification after an external edit")
	}

	assert.Equal(t, 123, AppConfig().SensorCollectionInterval)
}

func TestWatchConfigFiles_FailedReloadSendsNoNotification(t *testing.T) {
	dir := setupWatcherConfigDir(t)

	notified := make(chan struct{}, 1)
	startFastWatcher(t, func() { notify(notified) })

	broken := "sensor.collection.interval=not-a-number\nsensor.discovery.skip=true\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "application.properties"), []byte(broken), 0644))

	select {
	case <-notified:
		t.Fatal("reload notification sent for a file that fails to parse")
	case <-time.After(500 * time.Millisecond):
	}

	assert.Equal(t, 300, AppConfig().SensorCollectionInterval)
}

func TestWatchConfigFiles_OwnWriteSendsNoNotification(t *testing.T) {
	setupWatcherConfigDir(t)

	notified := make(chan struct{}, 1)
	startFastWatcher(t, func() { notify(notified) })

	require.NoError(t, SaveConfigurationToFiles())

	select {
	case <-notified:
		t.Fatal("reload notification sent for the application's own write")
	case <-time.After(500 * time.Millisecond):
	}
}
