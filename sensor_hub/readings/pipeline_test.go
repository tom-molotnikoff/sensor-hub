package readings_test

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/readings"
	"example/sensorHub/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type consumerCall struct {
	name     string
	sensor   gen.Sensor
	readings []readings.Reading
}

type recordingConsumer struct {
	name  string
	calls *[]consumerCall
}

func (c recordingConsumer) Consume(_ context.Context, sensor gen.Sensor, batch []readings.Reading) {
	*c.calls = append(*c.calls, consumerCall{name: c.name, sensor: sensor, readings: batch})
}

type storage struct {
	handles  *database.Handles
	sensors  database.SensorRepositoryInterface[gen.Sensor]
	readings database.ReadingsRepository
	liveView *service.LiveView
	sensor   gen.Sensor
	logger   *slog.Logger
}

func migratedStorage(t *testing.T) storage {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	handles, err := database.Open(&appProps.ApplicationConfiguration{
		DatabasePath:              filepath.Join(t.TempDir(), "pipeline.db"),
		DatabaseReaderConnections: 4,
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { handles.Close() })

	ctx := context.Background()
	sensors := database.NewSensorRepository(handles, logger)
	require.NoError(t, sensors.AddSensor(ctx, gen.Sensor{Name: "office-sensor", SensorDriver: "sensor-hub-http-temperature"}))
	sensor, err := sensors.GetSensorByName(ctx, "office-sensor")
	require.NoError(t, err)

	return storage{
		handles:  handles,
		sensors:  sensors,
		readings: database.NewReadingsRepository(handles, sensors, database.NewMeasurementTypeRepository(handles, logger), logger),
		liveView: service.NewLiveView(sensors, logger),
		sensor:   *sensor,
		logger:   logger,
	}
}

func (s storage) storedSensor(t *testing.T) gen.Sensor {
	t.Helper()
	sensor, err := s.sensors.GetSensorByName(context.Background(), s.sensor.Name)
	require.NoError(t, err)
	return *sensor
}

func numeric(measurementType string, value float64) gen.Reading {
	return gen.Reading{MeasurementType: measurementType, NumericValue: &value, Time: "2026-10-06 12:00:00"}
}

func TestProcess_PassesOnlyStoredReadingsToEachConsumerInOrder(t *testing.T) {
	store := migratedStorage(t)
	var calls []consumerCall
	pipeline := readings.NewPipeline(store.readings, store.liveView, store.logger,
		recordingConsumer{name: "commands", calls: &calls},
		recordingConsumer{name: "alerts", calls: &calls},
		recordingConsumer{name: "live view", calls: &calls},
	)

	err := pipeline.Process(context.Background(), store.sensor, []gen.Reading{numeric("temperature", 21.5), numeric("not_a_type", 1)})

	require.NoError(t, err)
	require.Len(t, calls, 3)
	for i, name := range []string{"commands", "alerts", "live view"} {
		assert.Equal(t, name, calls[i].name)
		assert.Equal(t, store.sensor, calls[i].sensor)
		require.Len(t, calls[i].readings, 1)
		assert.Equal(t, "temperature", calls[i].readings[0].MeasurementType)
		assert.Equal(t, "office-sensor", calls[i].readings[0].SensorName)
	}
	stored := store.storedSensor(t)
	assert.Equal(t, gen.Good, stored.HealthStatus)
	assert.Equal(t, "successful reading", stored.HealthReason)
}

func TestProcess_MarksHealthBadAndCallsNoConsumerWhenStorageFails(t *testing.T) {
	store := migratedStorage(t)
	_, err := store.handles.Writer.Exec(`CREATE TRIGGER reject_high AFTER INSERT ON readings
		WHEN NEW.numeric_value > 100 BEGIN SELECT RAISE(ABORT, 'rejected'); END`)
	require.NoError(t, err)
	var calls []consumerCall
	pipeline := readings.NewPipeline(store.readings, store.liveView, store.logger, recordingConsumer{name: "alerts", calls: &calls})

	err = pipeline.Process(context.Background(), store.sensor, []gen.Reading{numeric("temperature", 21.5), numeric("temperature", 999)})

	assert.ErrorContains(t, err, "rejected")
	stored := store.storedSensor(t)
	assert.Equal(t, gen.Bad, stored.HealthStatus)
	assert.Regexp(t, `^error storing readings: issue persisting reading to database: .*rejected`, stored.HealthReason)
	assert.Empty(t, calls)
}
