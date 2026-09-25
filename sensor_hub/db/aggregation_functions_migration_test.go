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
