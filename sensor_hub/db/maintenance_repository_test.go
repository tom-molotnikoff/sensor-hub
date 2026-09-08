package database

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func newInMemoryDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func newMigratedTempFileDB(t *testing.T) *sql.DB {
	t.Helper()
	db := newTempFileDB(t)
	require.NoError(t, RunMigrations(db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	return db
}

func newTempFileDB(t *testing.T) *sql.DB {
	t.Helper()
	db, _ := newTempFileDBAt(t)
	return db
}

func newTempFileDBAt(t *testing.T) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", "file:"+path+"?"+writerDSNParams)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db, path
}

func fillAndEmptyTable(t *testing.T, db *sql.DB, rows int) {
	t.Helper()
	_, err := db.Exec("CREATE TABLE test_data (id INTEGER PRIMARY KEY, payload TEXT)")
	require.NoError(t, err)
	_, err = db.Exec(`WITH RECURSIVE seq(n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < ?)
		INSERT INTO test_data (payload) SELECT hex(randomblob(100)) FROM seq`, rows)
	require.NoError(t, err)
	_, err = db.Exec("DELETE FROM test_data")
	require.NoError(t, err)
}

func TestMaintenanceRepository_DatabaseStats(t *testing.T) {
	db := newInMemoryDB(t)
	repo := NewMaintenanceRepository(handles(db))

	_, err := db.Exec("CREATE TABLE dummy (id INTEGER PRIMARY KEY)")
	require.NoError(t, err)

	stats, err := repo.DatabaseStats(context.Background())
	require.NoError(t, err)

	assert.Greater(t, stats.PageCount, int64(0))
	assert.GreaterOrEqual(t, stats.FreelistCount, int64(0))
	assert.Greater(t, stats.PageSize, int64(0))
}

func TestMaintenanceRepository_Optimise(t *testing.T) {
	db := newInMemoryDB(t)
	repo := NewMaintenanceRepository(handles(db))

	err := repo.Optimise(context.Background())
	assert.NoError(t, err)
}

func TestMaintenanceRepository_ReclaimFreePages_RejectsNonPositiveChunk(t *testing.T) {
	db := newTempFileDB(t)
	repo := NewMaintenanceRepository(handles(db))

	_, err := repo.ReclaimFreePages(context.Background(), 0)
	assert.Error(t, err)
}

func TestMaintenanceRepository_ReclaimFreePages_ReturnsPagesFreedInChunks(t *testing.T) {
	db := newMigratedTempFileDB(t)
	repo := NewMaintenanceRepository(handles(db))

	fillAndEmptyTable(t, db, 10000)

	statsAfterDelete, err := repo.DatabaseStats(context.Background())
	require.NoError(t, err)
	require.Greater(t, statsAfterDelete.FreelistCount, int64(64), "the delete leaves more than one chunk to reclaim")

	freed, err := repo.ReclaimFreePages(context.Background(), 64)
	require.NoError(t, err)

	assert.Equal(t, int64(64), freed, "one chunk is reclaimed per call")

	statsAfterChunk, err := repo.DatabaseStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, statsAfterDelete.FreelistCount-64, statsAfterChunk.FreelistCount)
	assert.Equal(t, statsAfterDelete.PageCount-64, statsAfterChunk.PageCount)
}

func TestMaintenanceRepository_ReclaimFreePages_ReturnsZeroWhenFreelistIsEmpty(t *testing.T) {
	db := newMigratedTempFileDB(t)
	repo := NewMaintenanceRepository(handles(db))

	freed, err := repo.ReclaimFreePages(context.Background(), 512)
	require.NoError(t, err)
	assert.Equal(t, int64(0), freed)
}

func walSize(t *testing.T, dbPath string) int64 {
	t.Helper()
	info, err := os.Stat(dbPath + "-wal")
	if os.IsNotExist(err) {
		return 0
	}
	require.NoError(t, err)
	return info.Size()
}

func TestMaintenanceRepository_Checkpoint_TruncatesTheWAL(t *testing.T) {
	db, path := newTempFileDBAt(t)
	repo := NewMaintenanceRepository(handles(db))

	_, err := db.Exec("CREATE TABLE dummy (id INTEGER PRIMARY KEY, payload TEXT)")
	require.NoError(t, err)
	_, err = db.Exec(`WITH RECURSIVE seq(n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 5000)
		INSERT INTO dummy (payload) SELECT hex(randomblob(100)) FROM seq`)
	require.NoError(t, err)
	require.Greater(t, walSize(t, path), int64(0), "the writes leave a write-ahead log to truncate")

	result, err := repo.Checkpoint(context.Background())
	require.NoError(t, err)

	assert.Equal(t, int64(0), result.Busy)
	assert.Equal(t, int64(0), walSize(t, path), "a truncating checkpoint empties the write-ahead log")
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
