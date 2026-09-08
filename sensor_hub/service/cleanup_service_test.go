package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ============================================================================
// Test helpers
// ============================================================================

func setupCleanupService() (*cleanupService, *MockSensorRepository, *MockReadingsRepository, *MockFailedLoginRepository, *MockAlertRepository, *MockMaintenanceRepository) {
	sensorRepo := new(MockSensorRepository)
	readingsRepo := new(MockReadingsRepository)
	failedRepo := new(MockFailedLoginRepository)
	alertRepo := new(MockAlertRepository)
	maintenanceRepo := new(MockMaintenanceRepository)
	sampler := new(MockReadingsSampler)
	sampler.On("Sample", mock.Anything).Return(nil).Maybe()

	service := &cleanupService{
		sensorRepo:      sensorRepo,
		readingsRepo:    readingsRepo,
		failedRepo:      failedRepo,
		alertRepo:       alertRepo,
		maintenanceRepo: maintenanceRepo,
		readingsSampler: sampler,
		logger:          slog.Default().With("component", "cleanup_service"),
		metrics:         newSQLiteInstruments(),
	}
	return service, sensorRepo, readingsRepo, failedRepo, alertRepo, maintenanceRepo
}

func defaultMaintenanceExpectations(maintenanceRepo *MockMaintenanceRepository) {
	stats := &database.DatabaseStatsResult{PageCount: 100, FreelistCount: 10, PageSize: 4096}
	statsAfter := &database.DatabaseStatsResult{PageCount: 90, FreelistCount: 0, PageSize: 4096}
	maintenanceRepo.On("DatabaseStats", mock.Anything).Return(stats, nil).Once()
	maintenanceRepo.On("ReclaimFreePages", mock.Anything, reclaimChunkPages).Return(int64(10), nil).Once()
	maintenanceRepo.On("ReclaimFreePages", mock.Anything, reclaimChunkPages).Return(int64(0), nil)
	maintenanceRepo.On("Optimise", mock.Anything).Return(nil)
	maintenanceRepo.On("Checkpoint", mock.Anything).Return(&database.CheckpointResult{}, nil)
	maintenanceRepo.On("DatabaseStats", mock.Anything).Return(statsAfter, nil).Once()
}

// ============================================================================
// performCleanup tests
// ============================================================================

func TestCleanupService_PerformCleanup_AllEnabled(t *testing.T) {
	service, sensorRepo, readingsRepo, failedRepo, alertRepo, maintenanceRepo := setupCleanupService()

	sensorRepo.On("GetSensorsWithRetention", mock.Anything).Return([]gen.Sensor{}, nil)
	readingsRepo.On("DeleteReadingsOlderThanExcludingSensors", mock.Anything, mock.AnythingOfType("time.Time"), []int{}).Return(nil)
	sensorRepo.On("DeleteHealthHistoryOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).Return(nil)
	failedRepo.On("DeleteAttemptsOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).Return(nil)
	alertRepo.On("DeleteAlertHistoryOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).Return(int64(5), nil)
	defaultMaintenanceExpectations(maintenanceRepo)

	err := service.performCleanup(context.Background(), 30, 90, 7, 90)

	assert.NoError(t, err)
	readingsRepo.AssertExpectations(t)
	sensorRepo.AssertExpectations(t)
	failedRepo.AssertExpectations(t)
	alertRepo.AssertExpectations(t)
	maintenanceRepo.AssertExpectations(t)
}

func TestCleanupService_PerformCleanup_AllDisabled(t *testing.T) {
	service, _, _, _, _, maintenanceRepo := setupCleanupService()

	defaultMaintenanceExpectations(maintenanceRepo)

	// Zero values mean no cleanup should happen
	err := service.performCleanup(context.Background(), 0, 0, 0, 0)

	assert.NoError(t, err)
}

func TestCleanupService_PerformCleanup_OnlyTemperature(t *testing.T) {
	service, sensorRepo, readingsRepo, _, _, maintenanceRepo := setupCleanupService()

	sensorRepo.On("GetSensorsWithRetention", mock.Anything).Return([]gen.Sensor{}, nil)
	readingsRepo.On("DeleteReadingsOlderThanExcludingSensors", mock.Anything, mock.AnythingOfType("time.Time"), []int{}).Return(nil)
	defaultMaintenanceExpectations(maintenanceRepo)

	err := service.performCleanup(context.Background(), 0, 30, 0, 0)

	assert.NoError(t, err)
	readingsRepo.AssertExpectations(t)
	sensorRepo.AssertNotCalled(t, "DeleteHealthHistoryOlderThan")
}

func TestCleanupService_PerformCleanup_OnlyHealthHistory(t *testing.T) {
	service, sensorRepo, _, _, _, maintenanceRepo := setupCleanupService()

	sensorRepo.On("DeleteHealthHistoryOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).Return(nil)
	defaultMaintenanceExpectations(maintenanceRepo)

	err := service.performCleanup(context.Background(), 30, 0, 0, 0)

	assert.NoError(t, err)
	sensorRepo.AssertExpectations(t)
}

func TestCleanupService_PerformCleanup_OnlyFailedLogins(t *testing.T) {
	service, _, _, failedRepo, _, maintenanceRepo := setupCleanupService()

	failedRepo.On("DeleteAttemptsOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).Return(nil)
	defaultMaintenanceExpectations(maintenanceRepo)

	err := service.performCleanup(context.Background(), 0, 0, 7, 0)

	assert.NoError(t, err)
	failedRepo.AssertExpectations(t)
}

func TestCleanupService_PerformCleanup_TemperatureError_GetSensors(t *testing.T) {
	service, sensorRepo, _, _, _, _ := setupCleanupService()

	sensorRepo.On("GetSensorsWithRetention", mock.Anything).Return(nil, errors.New("database error"))

	err := service.performCleanup(context.Background(), 30, 90, 7, 90)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
}

func TestCleanupService_PerformCleanup_TemperatureError_GlobalDelete(t *testing.T) {
	service, sensorRepo, readingsRepo, _, _, _ := setupCleanupService()

	sensorRepo.On("GetSensorsWithRetention", mock.Anything).Return([]gen.Sensor{}, nil)
	readingsRepo.On("DeleteReadingsOlderThanExcludingSensors", mock.Anything, mock.AnythingOfType("time.Time"), []int{}).Return(errors.New("database error"))

	err := service.performCleanup(context.Background(), 30, 90, 7, 90)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
}

func TestCleanupService_PerformCleanup_HealthHistoryError(t *testing.T) {
	service, sensorRepo, readingsRepo, _, _, _ := setupCleanupService()

	sensorRepo.On("GetSensorsWithRetention", mock.Anything).Return([]gen.Sensor{}, nil)
	readingsRepo.On("DeleteReadingsOlderThanExcludingSensors", mock.Anything, mock.AnythingOfType("time.Time"), []int{}).Return(nil)
	sensorRepo.On("DeleteHealthHistoryOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).Return(errors.New("health history error"))

	err := service.performCleanup(context.Background(), 30, 90, 7, 90)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health history error")
}

func TestCleanupService_PerformCleanup_FailedLoginError(t *testing.T) {
	service, sensorRepo, readingsRepo, failedRepo, _, _ := setupCleanupService()

	sensorRepo.On("GetSensorsWithRetention", mock.Anything).Return([]gen.Sensor{}, nil)
	readingsRepo.On("DeleteReadingsOlderThanExcludingSensors", mock.Anything, mock.AnythingOfType("time.Time"), []int{}).Return(nil)
	sensorRepo.On("DeleteHealthHistoryOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).Return(nil)
	failedRepo.On("DeleteAttemptsOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).Return(errors.New("failed login error"))

	err := service.performCleanup(context.Background(), 30, 90, 7, 90)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed login error")
}

func TestCleanupService_PerformCleanup_RetentionDaysCalculation(t *testing.T) {
	service, sensorRepo, readingsRepo, failedRepo, alertRepo, maintenanceRepo := setupCleanupService()

	now := time.Now()
	expectedTempThreshold := now.AddDate(0, 0, -90)
	expectedHealthThreshold := now.AddDate(0, 0, -30)
	expectedFailedThreshold := now.AddDate(0, 0, -7)
	expectedAlertThreshold := now.AddDate(0, 0, -90)

	sensorRepo.On("GetSensorsWithRetention", mock.Anything).Return([]gen.Sensor{}, nil)

	readingsRepo.On("DeleteReadingsOlderThanExcludingSensors", mock.Anything, mock.MatchedBy(func(t time.Time) bool {
		return t.Sub(expectedTempThreshold).Abs() < time.Second
	}), []int{}).Return(nil)

	sensorRepo.On("DeleteHealthHistoryOlderThan", mock.Anything, mock.MatchedBy(func(t time.Time) bool {
		return t.Sub(expectedHealthThreshold).Abs() < time.Second
	})).Return(nil)

	failedRepo.On("DeleteAttemptsOlderThan", mock.Anything, mock.MatchedBy(func(t time.Time) bool {
		return t.Sub(expectedFailedThreshold).Abs() < time.Second
	})).Return(nil)

	alertRepo.On("DeleteAlertHistoryOlderThan", mock.Anything, mock.MatchedBy(func(t time.Time) bool {
		return t.Sub(expectedAlertThreshold).Abs() < time.Second
	})).Return(int64(0), nil)

	defaultMaintenanceExpectations(maintenanceRepo)

	err := service.performCleanup(context.Background(), 30, 90, 7, 90)

	assert.NoError(t, err)
}

func TestCleanupService_PerformCleanup_PerSensorRetention(t *testing.T) {
	service, sensorRepo, readingsRepo, failedRepo, alertRepo, maintenanceRepo := setupCleanupService()

	retentionHours := 48
	customSensor := gen.Sensor{Id: 42, Name: "sensor-co2", RetentionHours: &retentionHours}

	sensorRepo.On("GetSensorsWithRetention", mock.Anything).Return([]gen.Sensor{customSensor}, nil)
	readingsRepo.On("DeleteReadingsOlderThanForSensor", mock.Anything, mock.MatchedBy(func(t time.Time) bool {
		expected := time.Now().Add(-time.Duration(retentionHours) * time.Hour)
		return t.Sub(expected).Abs() < time.Second
	}), 42).Return(nil)
	readingsRepo.On("DeleteReadingsOlderThanExcludingSensors", mock.Anything, mock.AnythingOfType("time.Time"), []int{42}).Return(nil)
	sensorRepo.On("DeleteHealthHistoryOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).Return(nil)
	failedRepo.On("DeleteAttemptsOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).Return(nil)
	alertRepo.On("DeleteAlertHistoryOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).Return(int64(0), nil)
	defaultMaintenanceExpectations(maintenanceRepo)

	err := service.performCleanup(context.Background(), 30, 90, 7, 90)

	assert.NoError(t, err)
	readingsRepo.AssertCalled(t, "DeleteReadingsOlderThanForSensor", mock.Anything, mock.AnythingOfType("time.Time"), 42)
	readingsRepo.AssertCalled(t, "DeleteReadingsOlderThanExcludingSensors", mock.Anything, mock.AnythingOfType("time.Time"), []int{42})
}

func TestCleanupService_PerformCleanup_PerSensorRetentionError(t *testing.T) {
	service, sensorRepo, readingsRepo, _, _, _ := setupCleanupService()

	retentionHours := 24
	customSensor := gen.Sensor{Id: 7, Name: "sensor-co", RetentionHours: &retentionHours}

	sensorRepo.On("GetSensorsWithRetention", mock.Anything).Return([]gen.Sensor{customSensor}, nil)
	readingsRepo.On("DeleteReadingsOlderThanForSensor", mock.Anything, mock.AnythingOfType("time.Time"), 7).Return(errors.New("per-sensor delete error"))

	err := service.performCleanup(context.Background(), 30, 90, 7, 90)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "per-sensor delete error")
}

func TestCleanupService_PerformCleanup_MultipleCustomSensors(t *testing.T) {
	service, sensorRepo, readingsRepo, failedRepo, alertRepo, maintenanceRepo := setupCleanupService()

	h1, h2 := 24, 720
	sensors := []gen.Sensor{
		{Id: 1, Name: "sensor-a", RetentionHours: &h1},
		{Id: 2, Name: "sensor-b", RetentionHours: &h2},
	}

	sensorRepo.On("GetSensorsWithRetention", mock.Anything).Return(sensors, nil)
	readingsRepo.On("DeleteReadingsOlderThanForSensor", mock.Anything, mock.AnythingOfType("time.Time"), 1).Return(nil)
	readingsRepo.On("DeleteReadingsOlderThanForSensor", mock.Anything, mock.AnythingOfType("time.Time"), 2).Return(nil)
	readingsRepo.On("DeleteReadingsOlderThanExcludingSensors", mock.Anything, mock.AnythingOfType("time.Time"), []int{1, 2}).Return(nil)
	sensorRepo.On("DeleteHealthHistoryOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).Return(nil)
	failedRepo.On("DeleteAttemptsOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).Return(nil)
	alertRepo.On("DeleteAlertHistoryOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).Return(int64(0), nil)
	defaultMaintenanceExpectations(maintenanceRepo)

	err := service.performCleanup(context.Background(), 30, 90, 7, 90)

	assert.NoError(t, err)
	readingsRepo.AssertExpectations(t)
}

// ============================================================================
// Database maintenance tests
// ============================================================================

func TestCleanupService_PerformCleanup_ReclaimOptimiseAndCheckpointAreCalled(t *testing.T) {
	service, _, _, _, _, maintenanceRepo := setupCleanupService()

	stats := &database.DatabaseStatsResult{PageCount: 200, FreelistCount: 50, PageSize: 4096}
	statsAfter := &database.DatabaseStatsResult{PageCount: 150, FreelistCount: 0, PageSize: 4096}
	maintenanceRepo.On("DatabaseStats", mock.Anything).Return(stats, nil).Once()
	maintenanceRepo.On("ReclaimFreePages", mock.Anything, reclaimChunkPages).Return(int64(50), nil).Once()
	maintenanceRepo.On("ReclaimFreePages", mock.Anything, reclaimChunkPages).Return(int64(0), nil).Once()
	maintenanceRepo.On("Optimise", mock.Anything).Return(nil)
	maintenanceRepo.On("Checkpoint", mock.Anything).Return(&database.CheckpointResult{LogPages: 4, CheckpointedPages: 4}, nil)
	maintenanceRepo.On("DatabaseStats", mock.Anything).Return(statsAfter, nil).Once()

	err := service.performCleanup(context.Background(), 0, 0, 0, 0)

	assert.NoError(t, err)
	maintenanceRepo.AssertExpectations(t)
}

func TestCleanupService_PerformCleanup_ReclaimLoopsUntilFreelistIsEmpty(t *testing.T) {
	service, _, _, _, _, maintenanceRepo := setupCleanupService()

	stats := &database.DatabaseStatsResult{PageCount: 200, FreelistCount: 1200, PageSize: 4096}
	statsAfter := &database.DatabaseStatsResult{PageCount: 150, FreelistCount: 0, PageSize: 4096}
	maintenanceRepo.On("DatabaseStats", mock.Anything).Return(stats, nil).Once()
	maintenanceRepo.On("ReclaimFreePages", mock.Anything, reclaimChunkPages).Return(int64(512), nil).Twice()
	maintenanceRepo.On("ReclaimFreePages", mock.Anything, reclaimChunkPages).Return(int64(176), nil).Once()
	maintenanceRepo.On("ReclaimFreePages", mock.Anything, reclaimChunkPages).Return(int64(0), nil).Once()
	maintenanceRepo.On("Optimise", mock.Anything).Return(nil)
	maintenanceRepo.On("Checkpoint", mock.Anything).Return(&database.CheckpointResult{}, nil)
	maintenanceRepo.On("DatabaseStats", mock.Anything).Return(statsAfter, nil).Once()

	err := service.performCleanup(context.Background(), 0, 0, 0, 0)

	assert.NoError(t, err)
	maintenanceRepo.AssertNumberOfCalls(t, "ReclaimFreePages", 4)
}

func TestCleanupService_PerformCleanup_ReclaimError_DoesNotFail(t *testing.T) {
	service, _, _, _, _, maintenanceRepo := setupCleanupService()

	stats := &database.DatabaseStatsResult{PageCount: 100, FreelistCount: 10, PageSize: 4096}
	maintenanceRepo.On("DatabaseStats", mock.Anything).Return(stats, nil).Once()
	maintenanceRepo.On("ReclaimFreePages", mock.Anything, reclaimChunkPages).Return(int64(0), errors.New("reclaim error"))

	err := service.performCleanup(context.Background(), 0, 0, 0, 0)

	// Maintenance errors are warned, not returned
	assert.NoError(t, err)
}

func TestCleanupService_PerformCleanup_CheckpointError_DoesNotFail(t *testing.T) {
	service, _, _, _, _, maintenanceRepo := setupCleanupService()

	stats := &database.DatabaseStatsResult{PageCount: 100, FreelistCount: 0, PageSize: 4096}
	maintenanceRepo.On("DatabaseStats", mock.Anything).Return(stats, nil)
	maintenanceRepo.On("ReclaimFreePages", mock.Anything, reclaimChunkPages).Return(int64(0), nil)
	maintenanceRepo.On("Optimise", mock.Anything).Return(nil)
	maintenanceRepo.On("Checkpoint", mock.Anything).Return(nil, errors.New("checkpoint error"))

	err := service.performCleanup(context.Background(), 0, 0, 0, 0)

	assert.NoError(t, err)
}

func TestCleanupService_PerformCleanup_DatabaseStatsError_DoesNotFail(t *testing.T) {
	service, _, _, _, _, maintenanceRepo := setupCleanupService()

	maintenanceRepo.On("DatabaseStats", mock.Anything).Return(nil, errors.New("stats error"))

	err := service.performCleanup(context.Background(), 0, 0, 0, 0)

	// Maintenance errors are warned, not returned
	assert.NoError(t, err)
}

func TestCleanupService_PerformCleanup_OptimiseError_DoesNotFail(t *testing.T) {
	service, _, _, _, _, maintenanceRepo := setupCleanupService()

	stats := &database.DatabaseStatsResult{PageCount: 100, FreelistCount: 0, PageSize: 4096}
	maintenanceRepo.On("DatabaseStats", mock.Anything).Return(stats, nil)
	maintenanceRepo.On("ReclaimFreePages", mock.Anything, reclaimChunkPages).Return(int64(0), nil)
	maintenanceRepo.On("Optimise", mock.Anything).Return(errors.New("optimise error"))
	maintenanceRepo.On("Checkpoint", mock.Anything).Return(&database.CheckpointResult{}, nil)

	err := service.performCleanup(context.Background(), 0, 0, 0, 0)

	assert.NoError(t, err)
}

// ============================================================================
// Alert history cleanup tests
// ============================================================================

func TestCleanupService_PerformCleanup_AlertHistoryCleanup(t *testing.T) {
	service, _, _, _, alertRepo, maintenanceRepo := setupCleanupService()

	now := time.Now()
	expectedThreshold := now.AddDate(0, 0, -45)

	alertRepo.On("DeleteAlertHistoryOlderThan", mock.Anything, mock.MatchedBy(func(t time.Time) bool {
		return t.Sub(expectedThreshold).Abs() < time.Second
	})).Return(int64(12), nil)
	defaultMaintenanceExpectations(maintenanceRepo)

	err := service.performCleanup(context.Background(), 0, 0, 0, 45)

	assert.NoError(t, err)
	alertRepo.AssertExpectations(t)
}

func TestCleanupService_PerformCleanup_AlertHistoryCleanupDisabled(t *testing.T) {
	service, _, _, _, alertRepo, maintenanceRepo := setupCleanupService()

	defaultMaintenanceExpectations(maintenanceRepo)

	err := service.performCleanup(context.Background(), 0, 0, 0, 0)

	assert.NoError(t, err)
	alertRepo.AssertNotCalled(t, "DeleteAlertHistoryOlderThan")
}

func TestCleanupService_PerformCleanup_AlertHistoryError_DoesNotFail(t *testing.T) {
	service, _, _, _, alertRepo, maintenanceRepo := setupCleanupService()

	alertRepo.On("DeleteAlertHistoryOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).Return(int64(0), errors.New("alert history error"))
	defaultMaintenanceExpectations(maintenanceRepo)

	err := service.performCleanup(context.Background(), 0, 0, 0, 90)

	// Alert history errors are warned, not returned
	assert.NoError(t, err)
}

// ============================================================================
// NewCleanupService tests
// ============================================================================

func TestNewCleanupService_ReturnsService(t *testing.T) {
	sensorRepo := new(MockSensorRepository)
	readingsRepo := new(MockReadingsRepository)
	failedRepo := new(MockFailedLoginRepository)
	alertRepo := new(MockAlertRepository)
	maintenanceRepo := new(MockMaintenanceRepository)

	service := NewCleanupService(sensorRepo, readingsRepo, failedRepo, nil, alertRepo, maintenanceRepo, new(MockReadingsSampler), slog.Default())

	assert.NotNil(t, service)
}

// ============================================================================
// DatabaseStatsResult tests
// ============================================================================

func TestDatabaseStatsResult_SizeBytes(t *testing.T) {
	stats := &database.DatabaseStatsResult{PageCount: 100, FreelistCount: 10, PageSize: 4096}
	assert.Equal(t, int64(409600), stats.SizeBytes())
}

func TestDatabaseStatsResult_FreelistBytes(t *testing.T) {
	stats := &database.DatabaseStatsResult{PageCount: 100, FreelistCount: 10, PageSize: 4096}
	assert.Equal(t, int64(40960), stats.FreelistBytes())
}

func TestDatabaseStatsResult_FreelistRatio(t *testing.T) {
	stats := &database.DatabaseStatsResult{PageCount: 100, FreelistCount: 10, PageSize: 4096}
	assert.InDelta(t, 0.1, stats.FreelistRatio(), 0.001)
}

func TestDatabaseStatsResult_FreelistRatio_ZeroPages(t *testing.T) {
	stats := &database.DatabaseStatsResult{PageCount: 0, FreelistCount: 0, PageSize: 4096}
	assert.Equal(t, 0.0, stats.FreelistRatio())
}
