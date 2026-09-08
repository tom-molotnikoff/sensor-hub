package database

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seriesRows(t *testing.T, db *sql.DB) map[[2]int]string {
	t.Helper()
	rows, err := db.Query("SELECT sensor_id, measurement_type_id, unit FROM sensor_measurement_types")
	require.NoError(t, err)
	defer rows.Close()

	found := make(map[[2]int]string)
	for rows.Next() {
		var sensorID, typeID int
		var unit string
		require.NoError(t, rows.Scan(&sensorID, &typeID, &unit))
		found[[2]int{sensorID, typeID}] = unit
	}
	require.NoError(t, rows.Err())
	return found
}

func measurementTypeID(t *testing.T, db *sql.DB, name string) int {
	t.Helper()
	var id int
	require.NoError(t, db.QueryRow("SELECT id FROM measurement_types WHERE name = ?", name).Scan(&id))
	return id
}

func TestMigration22_RebuildsTheJoinTableFromReadings(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(21))

	_, err := db.Exec("INSERT INTO sensors (name, sensor_driver, config) VALUES ('Office', 'sensor-hub-http-temperature', '{}')")
	require.NoError(t, err)
	var sensorID int
	require.NoError(t, db.QueryRow("SELECT id FROM sensors WHERE name = 'Office'").Scan(&sensorID))

	reported := measurementTypeID(t, db, "temperature")
	declaredOnly := measurementTypeID(t, db, "humidity")
	reportedButUndeclared := measurementTypeID(t, db, "pressure")

	_, err = db.Exec("DELETE FROM sensor_measurement_types")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO sensor_measurement_types (sensor_id, measurement_type_id, unit) VALUES (?, ?, 'K'), (?, ?, '%')",
		sensorID, reported, sensorID, declaredOnly)
	require.NoError(t, err)

	_, err = db.Exec("INSERT INTO readings (sensor_id, measurement_type_id, numeric_value, time) VALUES (?, ?, 21.0, '2026-01-01 00:00:00'), (?, ?, 1000.0, '2026-01-01 00:00:00')",
		sensorID, reported, sensorID, reportedButUndeclared)
	require.NoError(t, err)

	require.NoError(t, m.Migrate(22))

	rows := seriesRows(t, db)
	assert.Contains(t, rows, [2]int{sensorID, reported}, "a pair with readings keeps its row")
	assert.Equal(t, "K", rows[[2]int{sensorID, reported}], "an existing row keeps its configured unit")
	assert.Contains(t, rows, [2]int{sensorID, reportedButUndeclared}, "a pair with readings and no row gains one")
	assert.NotContains(t, rows, [2]int{sensorID, declaredOnly}, "a pair with no readings loses its row")
	assert.Len(t, rows, 2)
}

func TestMigration22_DownLeavesTheJoinTableAlone(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(22))

	_, err := db.Exec("INSERT INTO sensors (name, sensor_driver, config) VALUES ('Office', 'sensor-hub-http-temperature', '{}')")
	require.NoError(t, err)
	var sensorID int
	require.NoError(t, db.QueryRow("SELECT id FROM sensors WHERE name = 'Office'").Scan(&sensorID))
	typeID := measurementTypeID(t, db, "temperature")
	_, err = db.Exec("INSERT INTO sensor_measurement_types (sensor_id, measurement_type_id) VALUES (?, ?)", sensorID, typeID)
	require.NoError(t, err)

	require.NoError(t, m.Migrate(21))

	assert.Equal(t, map[[2]int]string{{sensorID, typeID}: ""}, seriesRows(t, db))
}
