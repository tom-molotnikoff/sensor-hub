package database

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func newInMemoryDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMaintenanceRepository_DatabaseStats(t *testing.T) {
	db := newInMemoryDB(t)
	repo := NewMaintenanceRepository(handles(db))

	// Create a table to ensure the database has some pages
	_, err := db.Exec("CREATE TABLE dummy (id INTEGER PRIMARY KEY)")
	require.NoError(t, err)

	stats, err := repo.DatabaseStats(context.Background())
	require.NoError(t, err)

	assert.Greater(t, stats.PageCount, int64(0))
	assert.GreaterOrEqual(t, stats.FreelistCount, int64(0))
	assert.Greater(t, stats.PageSize, int64(0))
}

func TestMaintenanceRepository_Vacuum(t *testing.T) {
	db := newInMemoryDB(t)
	repo := NewMaintenanceRepository(handles(db))

	err := repo.Vacuum(context.Background())
	assert.NoError(t, err)
}

func TestMaintenanceRepository_Optimise(t *testing.T) {
	db := newInMemoryDB(t)
	repo := NewMaintenanceRepository(handles(db))

	err := repo.Optimise(context.Background())
	assert.NoError(t, err)
}

func TestMaintenanceRepository_StatsAfterInsertAndDelete(t *testing.T) {
	db := newInMemoryDB(t)
	repo := NewMaintenanceRepository(handles(db))

	// Create a table and insert data
	_, err := db.Exec("CREATE TABLE test_data (id INTEGER PRIMARY KEY, payload TEXT)")
	require.NoError(t, err)

	for i := 0; i < 1000; i++ {
		_, err = db.Exec("INSERT INTO test_data (payload) VALUES (?)", "some data payload that takes space")
		require.NoError(t, err)
	}

	statsBefore, err := repo.DatabaseStats(context.Background())
	require.NoError(t, err)
	assert.Greater(t, statsBefore.PageCount, int64(1))

	// Delete all data — freelist should grow
	_, err = db.Exec("DELETE FROM test_data")
	require.NoError(t, err)

	statsAfterDelete, err := repo.DatabaseStats(context.Background())
	require.NoError(t, err)
	assert.Greater(t, statsAfterDelete.FreelistCount, int64(0), "freelist should have pages after delete")

	// Vacuum — freelist should return to 0
	err = repo.Vacuum(context.Background())
	require.NoError(t, err)

	statsAfterVacuum, err := repo.DatabaseStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(0), statsAfterVacuum.FreelistCount, "freelist should be 0 after vacuum")
	assert.Less(t, statsAfterVacuum.PageCount, statsBefore.PageCount, "page count should decrease after vacuum")
}

func TestDatabaseStatsResult_Computed(t *testing.T) {
	stats := &DatabaseStatsResult{PageCount: 200, FreelistCount: 50, PageSize: 4096}

	assert.Equal(t, int64(819200), stats.SizeBytes())
	assert.Equal(t, int64(204800), stats.FreelistBytes())
	assert.InDelta(t, 0.25, stats.FreelistRatio(), 0.001)
}

func TestDatabaseStatsResult_ZeroPageCount(t *testing.T) {
	stats := &DatabaseStatsResult{PageCount: 0, FreelistCount: 0, PageSize: 4096}
	assert.Equal(t, 0.0, stats.FreelistRatio())
}
