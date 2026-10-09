package database

import (
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLatestMigrationVersion_IsTheVersionRunMigrationsLeaves(t *testing.T) {
	db := newTempFileDB(t)
	require.NoError(t, RunMigrations(db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	var migrated uint
	require.NoError(t, db.QueryRow("SELECT version FROM schema_migrations").Scan(&migrated))

	latest, err := LatestMigrationVersion()

	require.NoError(t, err)
	assert.Equal(t, migrated, latest)
}
