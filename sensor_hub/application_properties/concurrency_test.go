package appProps

import (
	"testing"
)

// The config global is replaced by ReloadConfig on watcher and HTTP handler
// goroutines while periodic tasks and request handlers read it. Run with
// -race to make this meaningful.
func TestReloadConfig_ConcurrentWithReads_IsRaceFree(t *testing.T) {
	origConfig := AppConfig()
	defer func() { SetAppConfig(origConfig) }()

	appMap := validAppPropsMap()
	smtpMap := validSmtpPropsMap()
	dbMap := validDbPropsMap()

	ReloadConfig(appMap, smtpMap, dbMap)

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			ReloadConfig(appMap, smtpMap, dbMap)
		}
		close(done)
	}()

	total := 0
	for {
		select {
		case <-done:
			if total == 0 {
				t.Fatal("reader never observed the config")
			}
			return
		default:
			total += AppConfig().SensorCollectionInterval
		}
	}
}
