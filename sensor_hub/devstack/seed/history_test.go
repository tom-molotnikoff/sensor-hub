package main

import (
	"context"
	"math"
	"testing"
	"time"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type seededSeries struct {
	device      device
	sensorID    int
	typeID      int
	measurement string
}

func (s seededSeries) key() [2]int {
	return [2]int{s.sensorID, s.typeID}
}

func allSeededSeries(t *testing.T, db *database.Handles) []seededSeries {
	t.Helper()
	ids, err := seededSensorIDs(context.Background(), db.Reader)
	require.NoError(t, err)
	var series []seededSeries
	for _, device := range (&seeder{httpMocks: composeHTTPMocks}).devices() {
		sensorID, ok := ids[device.Name]
		if !ok {
			continue
		}
		for _, measurement := range measurementsOf(device) {
			var typeID int
			require.NoError(t, db.Reader.QueryRow("SELECT id FROM measurement_types WHERE name = ?", measurement).Scan(&typeID))
			series = append(series, seededSeries{device: device, sensorID: sensorID, typeID: typeID, measurement: measurement})
		}
	}
	require.NotEmpty(t, series)
	return series
}

func measurementsOf(device device) []string {
	var names []string
	for _, signal := range device.signals {
		for _, track := range signal.generate(nil, 0, nil) {
			names = append(names, track.measurement)
		}
	}
	return names
}

func readingsOf(t *testing.T, db *database.Handles, series seededSeries) []gen.Reading {
	t.Helper()
	rows, err := db.Reader.Query(`SELECT numeric_value, text_state, time FROM readings
		WHERE sensor_id = ? AND measurement_type_id = ? ORDER BY time`, series.sensorID, series.typeID)
	require.NoError(t, err)
	defer rows.Close()
	var readings []gen.Reading
	for rows.Next() {
		var reading gen.Reading
		require.NoError(t, rows.Scan(&reading.NumericValue, &reading.TextState, &reading.Time))
		readings = append(readings, reading)
	}
	require.NoError(t, rows.Err())
	return readings
}

func storedTime(t *testing.T, reading gen.Reading) time.Time {
	t.Helper()
	at, err := time.Parse(time.DateTime, reading.Time)
	require.NoError(t, err)
	return at
}

func queryInt(t *testing.T, db *database.Handles, query string, args ...any) int {
	t.Helper()
	var count int
	require.NoError(t, db.Reader.QueryRow(query, args...).Scan(&count))
	return count
}

func gapsOtherThanOneMinute(t *testing.T, db *database.Handles, after time.Time) int {
	t.Helper()
	return queryInt(t, db, `SELECT COUNT(*) FROM (
		SELECT unixepoch(time) - unixepoch(LAG(time) OVER (PARTITION BY sensor_id, measurement_type_id ORDER BY time)) AS gap
		FROM readings WHERE time >= ?) WHERE gap IS NOT NULL AND gap != 60`, after.UTC().Format(time.DateTime))
}

func duplicateReadings(t *testing.T, db *database.Handles) int {
	t.Helper()
	return queryInt(t, db, `SELECT COUNT(*) FROM (
		SELECT 1 FROM readings GROUP BY sensor_id, measurement_type_id, time HAVING COUNT(*) > 1)`)
}

func forgetHistoryAfter(t *testing.T, db *database.Handles, cutoff time.Time) {
	t.Helper()
	at := cutoff.UTC().Format(time.DateTime)
	for _, query := range []string{
		"DELETE FROM readings WHERE time > ?",
		"DELETE FROM sensor_health_history WHERE recorded_at > ?",
		"DELETE FROM alert_sent_history WHERE sent_at > ?",
	} {
		_, err := db.Writer.Exec(query, at)
		require.NoError(t, err)
	}
}

func TestHistory_FirstRunFillsTheWindow(t *testing.T) {
	t.Parallel()
	db := openTempDatabase(t)
	started := time.Now().UTC()

	runSeedOver(t, db, historyWindow)

	series := allSeededSeries(t, db)
	stored := make(map[[2]int][]gen.Reading, len(series))
	for _, one := range series {
		stored[one.key()] = readingsOf(t, db, one)
	}
	t.Run("readings at one-minute spacing across the last 30 days", func(t *testing.T) {
		assert.Zero(t, gapsOtherThanOneMinute(t, db, started.Add(-historyWindow-time.Hour)))
		for _, one := range series {
			readings := stored[one.key()]
			require.Len(t, readings, int(historyWindow/readingInterval), "%s %s", one.device.Name, one.measurement)
			assert.WithinDuration(t, started.Add(-historyWindow+readingInterval), storedTime(t, readings[0]), 2*time.Second, one.measurement)
			assert.WithinDuration(t, started, storedTime(t, readings[len(readings)-1]), 2*time.Second, one.measurement)
		}
	})
	t.Run("the series are recorded for their sensors", func(t *testing.T) {
		for _, one := range series {
			assert.Equal(t, 1, queryInt(t, db, "SELECT COUNT(*) FROM sensor_measurement_types WHERE sensor_id = ? AND measurement_type_id = ? AND unit = ''",
				one.sensorID, one.typeID), "%s %s", one.device.Name, one.measurement)
		}
	})
	t.Run("values stay in the mock ranges and end on the mock's starting state", func(t *testing.T) {
		for _, device := range (&seeder{httpMocks: composeHTTPMocks}).devices() {
			byMeasurement := make(map[string][]gen.Reading)
			for _, one := range series {
				if one.device.Name == device.Name {
					byMeasurement[one.measurement] = stored[one.key()]
				}
			}
			for _, signal := range device.signals {
				assertFollowsTheMock(t, device.Name, signal, byMeasurement)
			}
		}
	})
	t.Run("every sensor has an unhealthy spell and ends healthy", func(t *testing.T) {
		ids, err := seededSensorIDs(context.Background(), db.Reader)
		require.NoError(t, err)
		logger := discardLogger()
		sensors := database.NewSensorRepository(db, logger)
		live := database.NewReadingsRepository(db, sensors, database.NewMeasurementTypeRepository(db, logger), logger)
		for name := range ids {
			reading := 1.0
			require.NoError(t, live.Ingest(context.Background(), database.ReadingBatch{
				SensorName: name,
				Readings:   []gen.Reading{{MeasurementType: "battery", NumericValue: &reading, Time: time.Now().UTC().Format(time.RFC3339)}},
			}))
		}
		for name, id := range ids {
			assert.Positive(t, queryInt(t, db, "SELECT COUNT(*) FROM sensor_health_history WHERE sensor_id = ? AND health_status = 'bad'", id), name)
			var last string
			require.NoError(t, db.Reader.QueryRow(`SELECT health_status FROM sensor_health_history WHERE sensor_id = ?
				ORDER BY recorded_at DESC, id DESC LIMIT 1`, id).Scan(&last))
			assert.Equal(t, "good", last, name)
			assert.Zero(t, queryInt(t, db, `SELECT COUNT(*) FROM (
				SELECT health_status = LAG(health_status) OVER (ORDER BY recorded_at, id) AS repeated
				FROM sensor_health_history WHERE sensor_id = ?) WHERE repeated`, id), "%s repeats a health status once live readings arrive", name)
		}
	})
	t.Run("every seeded rule has fired inside the window", func(t *testing.T) {
		for _, rule := range seededRules {
			fired := queryInt(t, db, `SELECT COUNT(*) FROM alert_sent_history h
				JOIN devseed_sensors d ON d.sensor_id = h.sensor_id
				JOIN measurement_types mt ON mt.id = h.measurement_type_id
				WHERE d.device = ? AND mt.name = ? AND h.sent_at > ?`,
				rule.device, rule.measurement, started.Add(-historyWindow).Format(time.DateTime))
			assert.Positive(t, fired, "%s %s", rule.device, rule.measurement)
		}
	})
}

func assertFollowsTheMock(t *testing.T, device string, signal signal, readings map[string][]gen.Reading) {
	t.Helper()
	numbers := func(measurement string) []float64 {
		values := make([]float64, 0, len(readings[measurement]))
		for _, reading := range readings[measurement] {
			require.NotNil(t, reading.NumericValue, "%s %s", device, measurement)
			values = append(values, *reading.NumericValue)
		}
		require.NotEmpty(t, values, "%s %s", device, measurement)
		return values
	}
	states := func(measurement string) []string {
		values := make([]string, 0, len(readings[measurement]))
		for _, reading := range readings[measurement] {
			require.NotNil(t, reading.TextState, "%s %s", device, measurement)
			values = append(values, *reading.TextState)
		}
		require.NotEmpty(t, values, "%s %s", device, measurement)
		return values
	}
	switch signal := signal.(type) {
	case walk:
		values := numbers(signal.measurement)
		for i, number := range values {
			require.GreaterOrEqual(t, number, signal.low, "%s %s", device, signal.measurement)
			require.LessOrEqual(t, number, signal.high, "%s %s", device, signal.measurement)
			if i > 0 {
				require.LessOrEqual(t, math.Abs(number-values[i-1]), signal.step+0.5*math.Pow10(-signal.decimals), "%s %s moves more than one step", device, signal.measurement)
			}
		}
		assert.Equal(t, signal.start, values[len(values)-1], "%s %s ends where the mock starts", device, signal.measurement)
	case uniform:
		for _, number := range numbers(signal.measurement) {
			require.GreaterOrEqual(t, number, signal.low, "%s %s", device, signal.measurement)
			require.LessOrEqual(t, number, signal.high, "%s %s", device, signal.measurement)
		}
	case steady:
		for _, number := range numbers(signal.measurement) {
			require.Equal(t, signal.value.number, number, "%s %s", device, signal.measurement)
		}
	case toggle:
		for _, state := range states(signal.measurement) {
			require.Contains(t, []string{stateOn, stateOff}, state, "%s %s", device, signal.measurement)
		}
	case plugDraw:
		power, energy, current := numbers("power"), numbers("energy"), numbers("current")
		for i := range power {
			require.GreaterOrEqual(t, power[i], signal.minPower, device)
			require.LessOrEqual(t, power[i], signal.maxPower, device)
			require.InDelta(t, power[i]/mainsVoltage, current[i], 0.0005, device)
			require.GreaterOrEqual(t, energy[i], 0.0, device)
			if i > 0 {
				require.GreaterOrEqual(t, energy[i], energy[i-1], "%s energy never goes down", device)
			}
		}
		assert.Equal(t, signal.startEnergy, energy[len(energy)-1], "%s energy ends where the mock starts", device)
		for _, state := range states("state") {
			require.Equal(t, stateOn, state, device)
		}
	default:
		t.Fatalf("%s has a signal the test does not know: %T", device, signal)
	}
}

func TestHistory_RerunStraightAwayAddsAtMostOneReadingPerSeries(t *testing.T) {
	t.Parallel()
	db := openTempDatabase(t)
	runSeed(t, db)
	series := allSeededSeries(t, db)
	before := make([]int, len(series))
	for i, one := range series {
		before[i] = len(readingsOf(t, db, one))
	}
	alertsBefore := queryInt(t, db, "SELECT COUNT(*) FROM alert_sent_history")
	healthBefore := queryInt(t, db, "SELECT COUNT(*) FROM sensor_health_history")

	runSeed(t, db)

	for i, one := range series {
		assert.LessOrEqual(t, len(readingsOf(t, db, one))-before[i], 1, "%s %s", one.device.Name, one.measurement)
	}
	assert.Zero(t, duplicateReadings(t, db))
	assert.LessOrEqual(t, queryInt(t, db, "SELECT COUNT(*) FROM alert_sent_history")-alertsBefore, len(seededRules))
	assert.Equal(t, healthBefore, queryInt(t, db, "SELECT COUNT(*) FROM sensor_health_history"))
}

func TestHistory_TopUpRunsFromTheNewestReadingToNow(t *testing.T) {
	t.Parallel()
	db := openTempDatabase(t)
	runSeedOver(t, db, 6*24*time.Hour)
	downtimeStarted := time.Now().Add(-5 * 24 * time.Hour)
	forgetHistoryAfter(t, db, downtimeStarted)
	series := allSeededSeries(t, db)
	newest := make([]time.Time, len(series))
	kept := make([]int, len(series))
	for i, one := range series {
		readings := readingsOf(t, db, one)
		newest[i] = storedTime(t, readings[len(readings)-1])
		kept[i] = len(readings)
	}
	cutoff := downtimeStarted.UTC().Format(time.DateTime)
	healthKept := queryInt(t, db, "SELECT COUNT(*) FROM sensor_health_history")
	alertsKept := queryInt(t, db, "SELECT COUNT(*) FROM alert_sent_history")
	started := time.Now().UTC()

	runSeedOver(t, db, historyWindow)

	for i, one := range series {
		readings := readingsOf(t, db, one)
		require.Greater(t, len(readings), kept[i], "%s %s", one.device.Name, one.measurement)
		assert.Equal(t, newest[i].Add(readingInterval), storedTime(t, readings[kept[i]]), "%s %s picks up a minute after its newest reading", one.device.Name, one.measurement)
		assert.WithinDuration(t, started, storedTime(t, readings[len(readings)-1]), readingInterval+2*time.Second, one.measurement)
	}
	assert.Zero(t, gapsOtherThanOneMinute(t, db, time.Time{}))
	assert.Zero(t, duplicateReadings(t, db))
	assert.Equal(t, healthKept, queryInt(t, db, "SELECT COUNT(*) FROM sensor_health_history WHERE recorded_at <= ?", cutoff))
	assert.Equal(t, alertsKept, queryInt(t, db, "SELECT COUNT(*) FROM alert_sent_history WHERE sent_at <= ?", cutoff))
	assert.Positive(t, queryInt(t, db, "SELECT COUNT(*) FROM sensor_health_history WHERE recorded_at > ? AND health_status = 'bad'", cutoff))
	assert.Positive(t, queryInt(t, db, "SELECT COUNT(*) FROM alert_sent_history WHERE sent_at > ?", cutoff))
}

func TestHistory_FortyDayGapFillsOnlyTheLastThirtyDays(t *testing.T) {
	t.Parallel()
	db := openTempDatabase(t)
	runSeed(t, db)
	series := allSeededSeries(t, db)
	fortyDaysAgo := time.Now().UTC().Add(-40 * 24 * time.Hour).Truncate(time.Second)
	_, err := db.Writer.Exec("DELETE FROM readings")
	require.NoError(t, err)
	for _, one := range series {
		_, err := db.Writer.Exec("INSERT INTO readings (sensor_id, measurement_type_id, numeric_value, text_state, time) VALUES (?, ?, 1, NULL, ?)",
			one.sensorID, one.typeID, fortyDaysAgo.Format(time.DateTime))
		require.NoError(t, err)
	}
	started := time.Now().UTC()

	runSeedOver(t, db, historyWindow)

	windowStart := started.Add(-historyWindow)
	for _, one := range series {
		readings := readingsOf(t, db, one)
		require.Len(t, readings, 1+int(historyWindow/readingInterval), "%s %s", one.device.Name, one.measurement)
		assert.Equal(t, fortyDaysAgo, storedTime(t, readings[0]))
		assert.WithinDuration(t, windowStart.Add(readingInterval), storedTime(t, readings[1]), 2*time.Second, "%s %s", one.device.Name, one.measurement)
	}
	assert.Zero(t, queryInt(t, db, "SELECT COUNT(*) FROM readings WHERE time > ? AND time < ?",
		fortyDaysAgo.Format(time.DateTime), windowStart.Format(time.DateTime)), "the ten days before the window stay empty")
}

func TestHistory_LeavesHandAddedAndRediscoveredSensorsAlone(t *testing.T) {
	db := openTempDatabase(t)
	runSeed(t, db)
	ctx := context.Background()
	sensors := newSensorService(db)
	require.NoError(t, sensors.ServiceDeleteSensorByName(ctx, "office-plug"))
	for _, sensor := range []gen.Sensor{
		{Name: "office-plug", ExternalId: ptr("office-plug"), SensorDriver: zigbee2mqttDriver, Status: gen.SensorStatusPending},
		{Name: "garage-sensor", ExternalId: ptr("garage-sensor"), SensorDriver: zigbee2mqttDriver, Status: gen.SensorStatusActive},
	} {
		require.NoError(t, sensors.ServiceAddSensor(ctx, sensor))
	}
	forgetHistoryAfter(t, db, time.Now().Add(-time.Hour))

	runSeed(t, db)

	for _, name := range []string{"office-plug", "garage-sensor"} {
		id, err := sensors.ServiceGetSensorIdByName(ctx, name)
		require.NoError(t, err)
		for _, table := range []string{"readings", "sensor_health_history", "alert_sent_history", "sensor_measurement_types"} {
			assert.Zero(t, queryInt(t, db, "SELECT COUNT(*) FROM "+table+" WHERE sensor_id = ?", id), "%s has rows in %s", name, table)
		}
	}
	livingRoom, err := sensors.ServiceGetSensorIdByName(ctx, "living-room-sensor")
	require.NoError(t, err)
	assert.Positive(t, queryInt(t, db, "SELECT COUNT(*) FROM readings WHERE sensor_id = ? AND time > ?",
		livingRoom, time.Now().Add(-time.Hour).UTC().Format(time.DateTime)), "the seeded sensors were still topped up")
}

func TestHistory_LeavesADisabledSeededSensorAlone(t *testing.T) {
	db := openTempDatabase(t)
	runSeed(t, db)
	ctx := context.Background()
	sensors := newSensorService(db)
	require.NoError(t, sensors.ServiceSetEnabledSensorByName(ctx, "bedroom-sensor", false))
	forgetHistoryAfter(t, db, time.Now().Add(-time.Hour))
	id, err := sensors.ServiceGetSensorIdByName(ctx, "bedroom-sensor")
	require.NoError(t, err)
	since := time.Now().Add(-time.Hour).UTC().Format(time.DateTime)

	runSeed(t, db)

	assert.Zero(t, queryInt(t, db, "SELECT COUNT(*) FROM readings WHERE sensor_id = ? AND time > ?", id, since))
	assert.Zero(t, queryInt(t, db, "SELECT COUNT(*) FROM sensor_health_history WHERE sensor_id = ? AND recorded_at > ?", id, since))
	disabled, err := sensors.ServiceGetSensorByName(ctx, "bedroom-sensor")
	require.NoError(t, err)
	assert.Equal(t, gen.Unknown, disabled.HealthStatus)
}

func ptr(value string) *string {
	return &value
}
