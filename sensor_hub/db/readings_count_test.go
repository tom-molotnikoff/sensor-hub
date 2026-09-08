package database

import (
	"context"
	"log/slog"
	"testing"

	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCountReadingsPerActiveSensor_CountsEverySensorIncludingEmptyOnes(t *testing.T) {
	repo, db := migratedReadingsRepo(t)
	ctx := context.Background()

	sensorRepo := NewSensorRepository(handles(db), slog.Default())
	require.NoError(t, sensorRepo.AddSensor(ctx, gen.Sensor{Name: "Loft", SensorDriver: "sensor-hub-http-temperature"}))

	value := 21.5
	readings := []gen.Reading{
		{SensorName: "Office", MeasurementType: "temperature", Unit: "°C", NumericValue: &value, Time: "2026-01-16 12:00:00"},
		{SensorName: "Office", MeasurementType: "temperature", Unit: "°C", NumericValue: &value, Time: "2026-01-16 12:01:00"},
	}
	require.NoError(t, repo.Ingest(ctx, ReadingBatch{SensorName: "Office", Readings: readings}))

	counts, err := repo.CountReadingsPerActiveSensor(ctx)
	require.NoError(t, err)

	assert.Equal(t, map[string]int{"Office": 2, "Loft": 0}, counts)
}

func TestCountReadingsPerActiveSensor_ExcludesInactiveSensors(t *testing.T) {
	repo, db := migratedReadingsRepo(t)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, "UPDATE sensors SET status = 'dismissed' WHERE name = 'Office'")
	require.NoError(t, err)

	counts, err := repo.CountReadingsPerActiveSensor(ctx)
	require.NoError(t, err)

	assert.Empty(t, counts)
}
