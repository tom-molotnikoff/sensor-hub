package service

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"example/sensorHub/alerting"
	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/readings"
	"example/sensorHub/testharness/sqlcount"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const readingsPerMessage = 9

var messageTypes = []string{"temperature", "humidity", "pressure", "power", "battery", "voltage", "luminance", "link_quality", "energy"}

func countingSensorService(t *testing.T) (*SensorService, database.ReadingsRepository, gen.Sensor, *sqlcount.Recorder) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	driverName, recorder, err := sqlcount.Register()
	require.NoError(t, err)

	handles, err := database.OpenWithDriver(driverName, &appProps.ApplicationConfiguration{
		DatabasePath:              filepath.Join(t.TempDir(), "ingest.db"),
		DatabaseReaderConnections: 4,
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { handles.Close() })

	ctx := context.Background()
	sensorRepo := database.NewSensorRepository(handles, logger)
	require.NoError(t, sensorRepo.AddSensor(ctx, gen.Sensor{Name: "office-plug", SensorDriver: "sensor-hub-http-temperature"}))
	sensor, err := sensorRepo.GetSensorByName(ctx, "office-plug")
	require.NoError(t, err)

	mtRepo := database.NewMeasurementTypeRepository(handles, logger)
	readingsRepo := database.NewReadingsRepository(handles, sensorRepo, mtRepo, logger)
	processor := alerting.NewThresholdAlertProcessor(database.NewAlertRepository(handles, logger), nil, nil, nil, logger)

	liveView := NewLiveView(sensorRepo, logger)
	pipeline := readings.NewPipeline(readingsRepo, liveView, logger, processor, liveView)

	return NewSensorService(sensorRepo, mtRepo, pipeline, liveView, nil, nil, logger), readingsRepo, *sensor, recorder
}

func message() []gen.Reading {
	readings := make([]gen.Reading, 0, readingsPerMessage)
	for i, measurementType := range messageTypes {
		value := float64(i)
		readings = append(readings, gen.Reading{
			MeasurementType: measurementType,
			NumericValue:    &value,
			Time:            "2026-01-16 12:00:00",
		})
	}
	return readings
}

func TestServiceProcessPushReadings_CostsAtMostTwelveStatementsInOneTransaction(t *testing.T) {
	service, _, sensor, recorder := countingSensorService(t)
	ctx := context.Background()
	require.NoError(t, service.ServiceProcessPushReadings(ctx, sensor, message()))

	recorder.Reset()
	require.NoError(t, service.ServiceProcessPushReadings(ctx, sensor, message()))

	transactions := recorder.Transactions()
	require.Len(t, transactions, 1, "one transaction for the message")
	assert.True(t, transactions[0].Committed, "the transaction commits")

	statements := transactions[0].Statements
	assert.LessOrEqual(t, len(statements), 12, "statements executed: %v", statements)

	inserted := 0
	for _, statement := range statements {
		assert.NotContains(t, statement, "LOWER(name)", "no name is looked up on the ingest path")
		if strings.HasPrefix(statement, "INSERT INTO readings") {
			inserted++
		}
	}
	assert.Equal(t, readingsPerMessage, inserted, "every reading of the message is stored")
}

func TestIngest_LooksUpNoNamesAndWritesNoSeriesRowForAKnownPair(t *testing.T) {
	_, readingsRepo, sensor, recorder := countingSensorService(t)
	ctx := context.Background()
	batch := readings.ReadingBatch{SensorName: sensor.Name, HealthReason: "successful reading", Readings: message()}
	_, err := readingsRepo.Ingest(ctx, batch)
	require.NoError(t, err)

	recorder.Reset()
	_, err = readingsRepo.Ingest(ctx, batch)
	require.NoError(t, err)

	statements := recorder.All()
	assert.Len(t, statements, 11, "nine inserts and the two-statement health update: %v", statements)
	for _, statement := range statements {
		assert.NotContains(t, statement, "LOWER(name)", "names resolve from cache, not from the database")
		assert.NotContains(t, statement, "INSERT OR IGNORE INTO sensor_measurement_types",
			"a pair already seen is not written again")
	}
}
