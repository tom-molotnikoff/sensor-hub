package database

import (
	"context"
	"log/slog"
	"testing"

	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seriesCount(t *testing.T, repo *ReadingsRepositoryImpl, sensorName, measurementType string) int {
	t.Helper()
	var count int
	query := `SELECT COUNT(*) FROM sensor_measurement_types smt
		JOIN sensors s ON s.id = smt.sensor_id
		JOIN measurement_types mt ON mt.id = smt.measurement_type_id
		WHERE LOWER(s.name) = LOWER(?) AND mt.name = ?`
	require.NoError(t, repo.db.Reader.QueryRow(query, sensorName, measurementType).Scan(&count))
	return count
}

func readingCount(t *testing.T, repo *ReadingsRepositoryImpl) int {
	t.Helper()
	var count int
	require.NoError(t, repo.db.Reader.QueryRow("SELECT COUNT(*) FROM readings").Scan(&count))
	return count
}

func reading(measurementType string, value float64) gen.Reading {
	return gen.Reading{MeasurementType: measurementType, NumericValue: &value, Time: "2026-01-16 12:00:00"}
}

func TestIngest_RecordsASeriesOnFirstSightOnly(t *testing.T) {
	repo, db := migratedReadingsRepo(t)
	ctx := context.Background()
	require.Equal(t, 0, seriesCount(t, repo, "Office", "temperature"), "the pair starts unknown")

	require.NoError(t, repo.Ingest(ctx, ReadingBatch{
		SensorName: "Office",
		Readings:   []gen.Reading{reading("temperature", 21.0), reading("temperature", 21.5)},
	}))
	assert.Equal(t, 1, seriesCount(t, repo, "Office", "temperature"), "one row however many readings")

	_, err := db.Exec("DELETE FROM sensor_measurement_types")
	require.NoError(t, err)

	require.NoError(t, repo.Ingest(ctx, ReadingBatch{
		SensorName: "Office",
		Readings:   []gen.Reading{reading("temperature", 22.0)},
	}))
	assert.Equal(t, 0, seriesCount(t, repo, "Office", "temperature"), "a known pair is not written again")
}

func TestIngest_RollsBackTheWholeBatchWhenAReadingFails(t *testing.T) {
	repo, db := migratedReadingsRepo(t)
	ctx := context.Background()
	_, err := db.Exec(`CREATE TRIGGER reject_high AFTER INSERT ON readings
		WHEN NEW.numeric_value > 100 BEGIN SELECT RAISE(ABORT, 'rejected'); END`)
	require.NoError(t, err)

	err = repo.Ingest(ctx, ReadingBatch{
		SensorName: "Office",
		Readings:   []gen.Reading{reading("temperature", 21.0), reading("temperature", 999.0)},
	})

	require.Error(t, err)
	assert.Equal(t, 0, readingCount(t, repo), "no reading of the batch survives")
	assert.Equal(t, 0, seriesCount(t, repo, "Office", "temperature"), "the series row rolls back with the readings")
}

func TestIngest_ResolvesARenamedSensorAndForgetsTheOldName(t *testing.T) {
	repo, db := migratedReadingsRepo(t)
	ctx := context.Background()
	sensorRepo := NewSensorRepository(handles(db), slog.Default())
	renamedRepo := NewReadingsRepository(handles(db), sensorRepo, NewMeasurementTypeRepository(handles(db), slog.Default()), slog.Default())

	require.NoError(t, renamedRepo.Ingest(ctx, ReadingBatch{
		SensorName: "Office",
		Readings:   []gen.Reading{reading("temperature", 21.0)},
	}))
	id, err := sensorRepo.GetSensorIdByName(ctx, "Office")
	require.NoError(t, err)

	require.NoError(t, sensorRepo.UpdateSensorById(ctx, gen.Sensor{
		Id:           id,
		Name:         "Study",
		SensorDriver: "sensor-hub-http-temperature",
		Config:       map[string]string{},
	}, false))

	require.NoError(t, renamedRepo.Ingest(ctx, ReadingBatch{
		SensorName: "Study",
		Readings:   []gen.Reading{reading("temperature", 22.0)},
	}))
	assert.Equal(t, 2, readingCount(t, repo), "the new name resolves to the same sensor")

	err = renamedRepo.Ingest(ctx, ReadingBatch{
		SensorName: "Office",
		Readings:   []gen.Reading{reading("temperature", 23.0)},
	})
	assert.ErrorContains(t, err, "issue finding sensor id", "the old name is unknown")
}
