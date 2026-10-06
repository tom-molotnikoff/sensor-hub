package readings

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	batch    database.ReadingBatch
	keepOnly string
	err      error
}

func (f *fakeStore) Ingest(_ context.Context, batch database.ReadingBatch) ([]gen.Reading, error) {
	f.batch = batch
	if f.err != nil {
		return nil, f.err
	}
	var stored []gen.Reading
	for _, reading := range batch.Readings {
		if reading.MeasurementType == f.keepOnly {
			stored = append(stored, reading)
		}
	}
	return stored, nil
}

type healthCall struct {
	sensorID int
	status   gen.SensorHealthStatus
	reason   string
}

type fakeHealth struct {
	calls []healthCall
}

func (f *fakeHealth) RecordHealth(_ context.Context, sensorID int, status gen.SensorHealthStatus, reason string) {
	f.calls = append(f.calls, healthCall{sensorID: sensorID, status: status, reason: reason})
}

type consumerCall struct {
	name     string
	sensor   gen.Sensor
	readings []gen.Reading
}

type recordingConsumer struct {
	name  string
	calls *[]consumerCall
}

func (c recordingConsumer) Consume(_ context.Context, sensor gen.Sensor, readings []gen.Reading) {
	*c.calls = append(*c.calls, consumerCall{name: c.name, sensor: sensor, readings: readings})
}

func numeric(measurementType string, value float64) gen.Reading {
	return gen.Reading{MeasurementType: measurementType, NumericValue: &value, Time: "2026-10-06 12:00:00"}
}

func TestProcess_PassesOnlyStoredReadingsToEachConsumerInOrder(t *testing.T) {
	store := &fakeStore{keepOnly: "temperature"}
	health := &fakeHealth{}
	var calls []consumerCall
	pipeline := NewPipeline(store, health, slog.Default(),
		recordingConsumer{name: "commands", calls: &calls},
		recordingConsumer{name: "alerts", calls: &calls},
		recordingConsumer{name: "live view", calls: &calls},
	)
	sensor := gen.Sensor{Id: 7, Name: "office-sensor"}

	err := pipeline.Process(context.Background(), sensor, []gen.Reading{numeric("temperature", 21.5), numeric("not_a_type", 1)})

	require.NoError(t, err)
	assert.Equal(t, "office-sensor", store.batch.SensorName)
	for _, reading := range store.batch.Readings {
		assert.Equal(t, "office-sensor", reading.SensorName)
	}
	require.Len(t, calls, 3)
	for i, name := range []string{"commands", "alerts", "live view"} {
		assert.Equal(t, name, calls[i].name)
		assert.Equal(t, sensor, calls[i].sensor)
		require.Len(t, calls[i].readings, 1)
		assert.Equal(t, "temperature", calls[i].readings[0].MeasurementType)
	}
	assert.Empty(t, health.calls)
}

func TestProcess_MarksHealthBadAndCallsNoConsumerWhenStorageFails(t *testing.T) {
	store := &fakeStore{err: errors.New("disk full")}
	health := &fakeHealth{}
	var calls []consumerCall
	pipeline := NewPipeline(store, health, slog.Default(), recordingConsumer{name: "alerts", calls: &calls})

	err := pipeline.Process(context.Background(), gen.Sensor{Id: 7, Name: "office-sensor"}, []gen.Reading{numeric("temperature", 21.5)})

	assert.ErrorContains(t, err, "disk full")
	assert.Equal(t, []healthCall{{sensorID: 7, status: gen.Bad, reason: "error storing readings: disk full"}}, health.calls)
	assert.Empty(t, calls)
}
