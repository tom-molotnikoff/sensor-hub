package database

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func indexNames(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='index' AND tbl_name = ?", table)
	require.NoError(t, err)
	defer rows.Close()

	var names []string
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		names = append(names, n)
	}
	require.NoError(t, rows.Err())
	return names
}

func TestMigration20_DropsRedundantSensorIndexKeepsRest(t *testing.T) {
	db := newInMemoryDB(t)
	require.NoError(t, newTestMigrator(t, db).Migrate(20))

	indexes := indexNames(t, db, "readings")
	assert.NotContains(t, indexes, "idx_readings_sensor_id", "redundant prefix index is dropped")
	assert.Contains(t, indexes, "idx_readings_sensor_type_time", "composite index retained")
	assert.Contains(t, indexes, "idx_readings_time_asc", "ASC time index retained (retention delete uses it)")
	assert.Contains(t, indexes, "idx_readings_time", "DESC time index retained")
}

func TestMigration20_DownRestoresSensorIndex(t *testing.T) {
	db := newInMemoryDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(20))
	require.NoError(t, m.Migrate(19))

	assert.Contains(t, indexNames(t, db, "readings"), "idx_readings_sensor_id", "down migration restores the index")
}
