package appProps

import (
	"context"
	"log/slog"
	"os"
	"time"
)

// watchInterval is a var so tests can shorten it.
var watchInterval = 2 * time.Second

// cooldownAfterWrite is extra time to ignore file changes after the application
// finishes writing config files. This prevents the watcher from reloading
// changes that the application itself just wrote.
const cooldownAfterWrite = 3 * time.Second

// WatchConfigFiles polls the three property files for modification-time changes
// and reloads configuration when an external change is detected. It is aware of
// the application's own writes (via SaveConfigurationToFiles) and ignores them.
// onReload, if non-nil, is called after each successful reload.
//
// The goroutine exits when ctx is cancelled.
func WatchConfigFiles(ctx context.Context, onReload func()) {
	modTimes := snapshotModTimes()

	go func() {
		ticker := time.NewTicker(watchInterval)
		defer ticker.Stop()

		slog.Info("config file watcher started", "interval", watchInterval.String())

		for {
			select {
			case <-ctx.Done():
				slog.Info("config file watcher stopping", "reason", ctx.Err())
				return
			case <-ticker.C:
				// If the application is currently writing, snapshot the times
				// so we don't treat the in-progress write as an external change.
				if IsWriteInProgress() {
					modTimes = snapshotModTimes()
					continue
				}

				// Cooldown: ignore changes shortly after a write finishes.
				// This covers both the case where the watcher saw the flag AND
				// the case where the write completed between ticks.
				lastWrite := LastWriteCompletedAt()
				if !lastWrite.IsZero() && time.Since(lastWrite) < cooldownAfterWrite {
					modTimes = snapshotModTimes()
					continue
				}

				current := snapshotModTimes()
				changed := false
				for path, t := range current {
					if prev, ok := modTimes[path]; !ok || !t.Equal(prev) {
						changed = true
						break
					}
				}

				if !changed {
					continue
				}

				slog.Info("config file change detected, reloading")

				appProps, err := ReadApplicationPropertiesFile()
				if err != nil {
					slog.Error("failed to re-read application properties", "error", err)
					modTimes = current
					continue
				}

				smtpProps, err := ReadSMTPPropertiesFile()
				if err != nil {
					slog.Error("failed to re-read SMTP properties", "error", err)
					modTimes = current
					continue
				}

				dbProps, err := ReadDatabasePropertiesFile()
				if err != nil {
					slog.Error("failed to re-read database properties", "error", err)
					modTimes = current
					continue
				}

				reloadErr := ReloadConfig(appProps, smtpProps, dbProps)
				modTimes = current

				if reloadErr == nil && onReload != nil {
					onReload()
				}
			}
		}
	}()
}

// snapshotModTimes returns the current modification times of all config files.
func snapshotModTimes() map[string]time.Time {
	times := make(map[string]time.Time, 3)
	for _, path := range ConfigFilePaths() {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		times[path] = info.ModTime()
	}
	return times
}
