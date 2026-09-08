package seed

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	before := time.Now().UTC()
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

	var latest string
	require.NoError(t, db.QueryRow("SELECT MAX(time) FROM readings").Scan(&latest))
	newest, err := time.Parse("2006-01-02 15:04:05", latest)
	require.NoError(t, err)
	assert.False(t, newest.Before(before.Truncate(time.Second)),
		"the window ends at generation time, so the seed always looks like a live database")
}

func TestGenerate_StampsTheVersionAndTheShape(t *testing.T) {
	shape := Shape{Sensors: 2, MeasurementTypes: 2, Days: 4, Readings: 40}
	path := generate(t, shape)

	version, stored, err := StoredStamp(path)
	require.NoError(t, err)
	assert.Equal(t, Version, version)
	assert.Equal(t, shape, stored)
	assert.True(t, IsCurrent(path, shape))
}

func TestIsCurrent_IsFalseForAMissingFileOrADifferentShape(t *testing.T) {
	shape := Shape{Sensors: 1, MeasurementTypes: 1, Days: 2, Readings: 10}

	assert.False(t, IsCurrent(filepath.Join(t.TempDir(), "absent.db"), shape))

	path := generate(t, shape)
	assert.True(t, IsCurrent(path, shape))

	smaller := shape
	smaller.Readings = 5
	assert.False(t, IsCurrent(path, smaller), "a seed of a different shape is not current")
}

func TestGenerate_RejectsAShapeTheSchemaCannotSupply(t *testing.T) {
	err := Generate(context.Background(), filepath.Join(t.TempDir(), "seed.db"),
		Shape{Sensors: 1, MeasurementTypes: len(measurementTypeNames) + 1, Days: 1, Readings: 1}, discardLogger())
	assert.ErrorIs(t, err, errMeasurementTypesOutOfRange)
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
