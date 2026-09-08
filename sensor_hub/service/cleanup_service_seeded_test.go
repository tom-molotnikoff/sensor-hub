//go:build seeded

package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	database "example/sensorHub/db"
	"example/sensorHub/testharness/seed"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const readingsQueryWindowDays = 2

func timeReadingsQuery(t *testing.T, repo database.ReadingsRepository) time.Duration {
	t.Helper()
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -readingsQueryWindowDays)

	began := time.Now()
	_, err := repo.GetBetweenDates(context.Background(),
		start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"),
		"seed-sensor-01", "temperature",
		database.AggregationPT1H, database.AggregationFunctionAvg)
	require.NoError(t, err)
	return time.Since(began)
}

func TestCleanupService_PerformCleanup_DoesNotStallConcurrentReads(t *testing.T) {
	service, spy, _ := seededCleanupService(t, seed.Default)
	readingsRepo := service.readingsRepo

	var idle time.Duration
	for range 10 {
		if elapsed := timeReadingsQuery(t, readingsRepo); elapsed > idle {
			idle = elapsed
		}
	}

	done := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	var worst time.Duration
	var samples int

	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				elapsed := timeReadingsQuery(t, readingsRepo)
				mu.Lock()
				if elapsed > worst {
					worst = elapsed
				}
				samples++
				mu.Unlock()
			}
		}()
	}

	require.NoError(t, service.performCleanup(context.Background(), 0, seed.Default.Days/2, 0, 0))
	close(done)
	wg.Wait()

	require.Greater(t, spy.total, int64(0), "the retention delete leaves pages to reclaim")
	require.Greater(t, samples, 0, "reads ran during the pass")
	assert.Less(t, worst, idle+200*time.Millisecond,
		fmt.Sprintf("worst read during the pass %s, idle %s", worst, idle))
}
