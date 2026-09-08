package seed

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func generate(t *testing.T, shape Shape) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "seed.db")
	require.NoError(t, Generate(context.Background(), path, shape, discardLogger()))
	return path
}

func openSeed(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=query_only(1)")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestGenerate_WritesTheRequestedShape(t *testing.T) {
	shape := Shape{Sensors: 8, MeasurementTypes: 9, Days: 90, Readings: 3600}
	db := openSeed(t, generate(t, shape))

	var sensors, types, readings, series int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM sensors").Scan(&sensors))
	require.NoError(t, db.QueryRow("SELECT COUNT(DISTINCT measurement_type_id) FROM readings").Scan(&types))
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM readings").Scan(&readings))
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM (SELECT DISTINCT sensor_id, measurement_type_id FROM readings)").Scan(&series))

	assert.Equal(t, shape.Sensors, sensors)
	assert.Equal(t, shape.MeasurementTypes, types)
	assert.Equal(t, shape.Readings, readings)
	assert.Equal(t, shape.Sensors*shape.MeasurementTypes, series)

	var span float64
	require.NoError(t, db.QueryRow("SELECT julianday(MAX(time)) - julianday(MIN(time)) FROM readings").Scan(&span))
	assert.InDelta(t, float64(shape.Days), span, 0.01, "readings span the requested window")
}

func TestGenerate_StampsTheSeedVersion(t *testing.T) {
	path := generate(t, Shape{Sensors: 2, MeasurementTypes: 2, Days: 4, Readings: 40})

	version, err := StoredVersion(path)
	require.NoError(t, err)
	assert.Equal(t, Version, version)
	assert.True(t, IsCurrent(path))
}

func TestIsCurrent_IsFalseForAMissingOrStaleFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.db")
	assert.False(t, IsCurrent(missing))

	path := generate(t, Shape{Sensors: 1, MeasurementTypes: 1, Days: 2, Readings: 10})
	db, err := sql.Open("sqlite", "file:"+path)
	require.NoError(t, err)
	_, err = db.Exec("PRAGMA user_version = 0")
	require.NoError(t, err)
	require.NoError(t, db.Close())

	assert.False(t, IsCurrent(path))
}

func TestGenerate_RejectsAShapeTheSchemaCannotSupply(t *testing.T) {
	err := Generate(context.Background(), filepath.Join(t.TempDir(), "seed.db"),
		Shape{Sensors: 1, MeasurementTypes: len(measurementTypeNames) + 1, Days: 1, Readings: 1}, discardLogger())
	assert.ErrorIs(t, err, errNoMeasurementTypes)
}

func TestGenerate_FullSeed(t *testing.T) {
	if os.Getenv("SENSOR_HUB_FULL_SEED") == "" {
		t.Skip("set SENSOR_HUB_FULL_SEED=1 to generate the five-million-row seed")
	}

	db := openSeed(t, generate(t, Default))

	var readings int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM readings").Scan(&readings))
	assert.Equal(t, Default.Readings, readings)

	var latest string
	require.NoError(t, db.QueryRow(
		"SELECT time FROM readings WHERE sensor_id = 1 AND measurement_type_id = 1 ORDER BY time DESC LIMIT 1").Scan(&latest))
	assert.NotEmpty(t, latest)
}
