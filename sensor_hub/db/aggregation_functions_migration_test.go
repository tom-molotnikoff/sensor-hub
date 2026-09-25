package database

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func aggregationRows(t *testing.T, db *sql.DB) map[string]map[string]int {
	t.Helper()
	rows, err := db.Query(`
		SELECT mt.name, mta.function, mta.is_default
		FROM measurement_type_aggregations mta
		JOIN measurement_types mt ON mt.id = mta.measurement_type_id`)
	require.NoError(t, err)
	defer rows.Close()

	found := make(map[string]map[string]int)
	for rows.Next() {
		var typeName, function string
		var isDefault int
		require.NoError(t, rows.Scan(&typeName, &function, &isDefault))
		if found[typeName] == nil {
			found[typeName] = make(map[string]int)
		}
		found[typeName][function] = isDefault
	}
	require.NoError(t, rows.Err())
	return found
}

var numericMeasurementTypes = []string{
	"temperature", "humidity", "pressure", "power", "battery", "voltage",
	"luminance", "link_quality", "illuminance", "energy", "current", "co2",
	"voc", "formaldehyde", "pm25", "soil_moisture", "energy_today",
	"energy_month", "energy_yesterday",
}

func TestMigration23_OffersMinAndMaxForEveryNumericType(t *testing.T) {
	db := newInMemoryDB(t)
	require.NoError(t, newTestMigrator(t, db).Migrate(23))

	rows := aggregationRows(t, db)
	for _, name := range numericMeasurementTypes {
		assert.Equal(t, map[string]int{"avg": 1, "min": 0, "max": 0}, rows[name], name)
	}
	assert.Equal(t, map[string]int{"count": 1, "last": 0}, rows["motion"])
}

func TestMigration23_DownRemovesOnlyMinAndMax(t *testing.T) {
	db := newInMemoryDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(22))
	before := aggregationRows(t, db)

	require.NoError(t, m.Migrate(23))
	require.NoError(t, m.Migrate(22))

	assert.Equal(t, before, aggregationRows(t, db))
}

func TestMigration24_OffersIncreaseForEnergyCountersOnly(t *testing.T) {
	db := newInMemoryDB(t)
	require.NoError(t, newTestMigrator(t, db).Migrate(24))

	rows := aggregationRows(t, db)
	for _, name := range []string{"energy", "energy_today", "energy_month"} {
		assert.Equal(t, map[string]int{"avg": 1, "min": 0, "max": 0, "increase": 0}, rows[name], name)
	}
	assert.Equal(t, map[string]int{"avg": 1, "min": 0, "max": 0}, rows["energy_yesterday"])
	assert.Equal(t, map[string]int{"avg": 1, "min": 0, "max": 0}, rows["temperature"])
	assert.Equal(t, map[string]int{"count": 1, "last": 0}, rows["motion"])
}

func TestMigration24_RebuildKeepsEveryRowAndAllowsIncrease(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(23))

	_, err := db.Exec("INSERT INTO measurement_types (name, display_name, category, default_unit) VALUES ('flow', 'Flow', 'numeric', 'L')")
	require.NoError(t, err)
	flow := measurementTypeID(t, db, "flow")
	_, err = db.Exec("INSERT INTO measurement_type_aggregations (measurement_type_id, function, is_default) VALUES (?, 'last', 1)", flow)
	require.NoError(t, err)
	before := aggregationRows(t, db)

	require.NoError(t, m.Migrate(24))
	after := aggregationRows(t, db)
	for name, functions := range before {
		for function, isDefault := range functions {
			assert.Equal(t, isDefault, after[name][function], "%s %s", name, function)
		}
	}
	assert.Equal(t, map[string]int{"last": 1}, after["flow"])

	_, err = db.Exec("INSERT INTO measurement_type_aggregations (measurement_type_id, function, is_default) VALUES (?, 'increase', 0)", flow)
	assert.NoError(t, err)
	_, err = db.Exec("INSERT INTO measurement_type_aggregations (measurement_type_id, function, is_default) VALUES (?, 'sum', 0)", flow)
	assert.Error(t, err, "sum is no longer an allowed function")
}

func TestMigration24_DownRestoresTheEarlierConstraint(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(23))
	before := aggregationRows(t, db)

	require.NoError(t, m.Migrate(24))
	require.NoError(t, m.Migrate(23))
	assert.Equal(t, before, aggregationRows(t, db))

	temperature := measurementTypeID(t, db, "temperature")
	_, err := db.Exec("INSERT INTO measurement_type_aggregations (measurement_type_id, function, is_default) VALUES (?, 'increase', 0)", temperature)
	assert.Error(t, err, "increase is not allowed before migration 24")
	_, err = db.Exec("INSERT INTO measurement_type_aggregations (measurement_type_id, function, is_default) VALUES (?, 'sum', 0)", temperature)
	assert.NoError(t, err)
}
