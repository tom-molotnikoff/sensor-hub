//go:build seeded

package service

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"sort"
	"testing"
	"time"

	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	"example/sensorHub/testharness/seed"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seededHandles(t *testing.T) *database.Handles {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dbPath := filepath.Join(t.TempDir(), "first-load.db")
	require.NoError(t, seed.Generate(context.Background(), dbPath, seed.Default, logger))

	handles, err := database.Open(&appProps.ApplicationConfiguration{
		DatabasePath:              dbPath,
		DatabaseReaderConnections: 4,
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { handles.Close() })
	return handles
}

func typeNamesInJoinTable(t *testing.T, handles *database.Handles) []string {
	t.Helper()
	rows, err := handles.Reader.Query(`SELECT DISTINCT mt.name FROM sensor_measurement_types smt
		JOIN measurement_types mt ON mt.id = smt.measurement_type_id`)
	require.NoError(t, err)
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		names = append(names, name)
	}
	require.NoError(t, rows.Err())
	sort.Strings(names)
	return names
}

func TestMeasurementTypesWithReadings_AnswersUnderTenMillisecondsAtP99(t *testing.T) {
	handles := seededHandles(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo := database.NewMeasurementTypeRepository(handles, logger)
	ctx := context.Background()

	const calls = 20
	elapsed := make([]time.Duration, 0, calls)
	var last []string
	for range calls {
		began := time.Now()
		types, err := repo.GetAllWithReadings(ctx)
		require.NoError(t, err)
		elapsed = append(elapsed, time.Since(began))

		last = last[:0]
		for _, mt := range types {
			last = append(last, mt.Name)
		}
		sort.Strings(last)
	}

	slices.Sort(elapsed)
	p99 := elapsed[len(elapsed)-1]
	assert.Less(t, p99, 10*time.Millisecond, "p99 over %d calls was %s", calls, p99)
	assert.Equal(t, typeNamesInJoinTable(t, handles), last, "exactly the types with a row in the join table")
}

func TestLatestPerSeries_SeedsTheStoreWithinFiveHundredMilliseconds(t *testing.T) {
	handles := seededHandles(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sensorRepo := database.NewSensorRepository(handles, logger)
	mtRepo := database.NewMeasurementTypeRepository(handles, logger)

	began := time.Now()
	repo := database.NewReadingsRepository(handles, sensorRepo, mtRepo, logger)
	latest, err := repo.GetLatest(context.Background())
	seeding := time.Since(began)

	require.NoError(t, err)
	assert.Equal(t, seed.Default.Sensors*seed.Default.MeasurementTypes, len(latest), "one reading per series")
	assert.Less(t, seeding, 500*time.Millisecond, "cold seed took %s", seeding)
}
