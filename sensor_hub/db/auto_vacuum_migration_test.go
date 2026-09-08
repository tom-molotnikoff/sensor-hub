package database

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func autoVacuumMode(t *testing.T, db *sql.DB) int {
	t.Helper()
	var mode int
	require.NoError(t, db.QueryRow("PRAGMA auto_vacuum").Scan(&mode))
	return mode
}

func TestMigration21_SwitchesToIncrementalAutoVacuum(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(20))
	require.Equal(t, 0, autoVacuumMode(t, db), "databases start with auto_vacuum off")

	require.NoError(t, m.Migrate(21))

	assert.Equal(t, 2, autoVacuumMode(t, db), "auto_vacuum reports incremental")
}

func TestMigration21_DownRestoresNoAutoVacuum(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(21))
	require.NoError(t, m.Migrate(20))

	assert.Equal(t, 0, autoVacuumMode(t, db))
}
