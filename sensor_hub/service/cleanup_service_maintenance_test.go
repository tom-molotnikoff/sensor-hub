package service

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	"example/sensorHub/testharness/seed"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pagesFreedSpy struct {
	database.MaintenanceRepository
	total int64
}

func (s *pagesFreedSpy) ReclaimFreePages(ctx context.Context, chunkPages int) (int64, error) {
	freed, err := s.MaintenanceRepository.ReclaimFreePages(ctx, chunkPages)
	s.total += freed
	return freed, err
}

func seededCleanupService(t *testing.T, shape seed.Shape) (*cleanupService, *pagesFreedSpy, string) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dbPath := filepath.Join(t.TempDir(), "cleanup.db")
	require.NoError(t, seed.Generate(context.Background(), dbPath, shape, logger))

	handles, err := database.Open(&appProps.ApplicationConfiguration{
		DatabasePath:              dbPath,
		DatabaseReaderConnections: 4,
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { handles.Close() })

	sensorRepo := database.NewSensorRepository(handles, logger)
	mtRepo := database.NewMeasurementTypeRepository(handles, logger)
	readingsRepo := database.NewReadingsRepository(handles, sensorRepo, mtRepo, logger)
	spy := &pagesFreedSpy{MaintenanceRepository: database.NewMaintenanceRepository(handles)}

	service := NewCleanupService(
		sensorRepo,
		readingsRepo,
		database.NewFailedLoginRepository(handles, logger),
		nil,
		nil,
		spy,
		NewReadingsSampler(readingsRepo, logger),
		logger,
	).(*cleanupService)

	return service, spy, dbPath
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Size()
}

func TestCleanupService_PerformCleanup_ReturnsFreedPagesToTheFile(t *testing.T) {
	service, spy, dbPath := seededCleanupService(t, seed.Shape{
		Sensors:          4,
		MeasurementTypes: 4,
		Days:             60,
		Readings:         200_000,
	})

	sizeBefore := fileSize(t, dbPath)

	require.NoError(t, service.performCleanup(context.Background(), 0, 30, 0, 0))

	stats, err := service.maintenanceRepo.DatabaseStats(context.Background())
	require.NoError(t, err)

	require.Greater(t, spy.total, int64(0), "the retention delete leaves pages to reclaim")
	assert.Equal(t, int64(0), stats.FreelistCount, "the pass empties the freelist")
	assert.GreaterOrEqual(t, sizeBefore-fileSize(t, dbPath), spy.total*stats.PageSize,
		"the file shrinks by at least the reclaimed pages")
}
