package database

import (
	"testing"

	"example/sensorHub/drivers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ingest drops a reading whose type has no row, and a command is acknowledged
// only by a stored reading, so a driver type with no row leaves every command
// on that property to time out.
func TestMigrations_StoreEveryMeasurementTypeADriverReports(t *testing.T) {
	db := newMigratedTempFileDB(t)
	rows, err := db.Query("SELECT name, category FROM measurement_types")
	require.NoError(t, err)
	defer rows.Close()
	stored := map[string]string{}
	for rows.Next() {
		var name, category string
		require.NoError(t, rows.Scan(&name, &category))
		stored[name] = category
	}
	require.NoError(t, rows.Err())

	all := drivers.All()
	require.NotEmpty(t, all)
	for _, driver := range all {
		for _, measurementType := range driver.SupportedMeasurementTypes() {
			category, ok := stored[measurementType.Name]
			if assert.True(t, ok, "%s reports %q, which no migration adds", driver.Type(), measurementType.Name) {
				assert.Equal(t, string(measurementType.Category), category, "%s reports %q", driver.Type(), measurementType.Name)
			}
		}
	}
}
