package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Test helpers
// ============================================================================

func setupReadingsService() (*ReadingsService, *MockReadingsRepository, *MockMeasurementTypeRepository) {
	repo := new(MockReadingsRepository)
	mtRepo := new(MockMeasurementTypeRepository)
	svc := NewReadingsService(repo, mtRepo, DefaultAggregationTiers, true, slog.Default())
	return svc, repo, mtRepo
}

// ============================================================================
// ServiceGetBetweenDates tests
// ============================================================================

func TestReadingsService_ServiceGetBetweenDates_RawForShortRange(t *testing.T) {
	svc, repo, _ := setupReadingsService()

	val1 := 22.5
	readings := []gen.Reading{
		{Id: 1, SensorName: "LivingRoom", MeasurementType: "temperature", Unit: "°C", NumericValue: &val1, Time: "2025-01-15 10:00:00"},
	}
	// 10-minute span → raw (no aggregation)
	repo.On("GetBetweenDates", mock.Anything, "2025-01-15 10:00:00", "2025-01-15 10:10:00", "", "", database.AggregationRaw, database.AggregationFunctionNone).Return(readings, nil)

	result, err := svc.ServiceGetBetweenDates(context.Background(), "2025-01-15 10:00:00", "2025-01-15 10:10:00", "", "", "", "")

	assert.NoError(t, err)
	assert.Equal(t, gen.AggregatedReadingsResponseAggregationIntervalRaw, result.AggregationInterval)
	assert.Equal(t, gen.AggregatedReadingsResponseAggregationFunctionNone, result.AggregationFunction)
	assert.Len(t, result.Readings, 1)
}

func TestReadingsService_ServiceGetBetweenDates_AggregatedFor3DayRange(t *testing.T) {
	svc, repo, mtRepo := setupReadingsService()

	val1 := 22.5
	readings := []gen.Reading{
		{Id: 0, SensorName: "LivingRoom", MeasurementType: "temperature", Unit: "°C", NumericValue: &val1, Time: "2025-01-15 10:00:00"},
	}
	mtRepo.On("GetAggregationsForMeasurementType", mock.Anything, "temperature").Return(&database.MeasurementTypeAggregation{
		MeasurementType:    "temperature",
		DefaultFunction:    "avg",
		SupportedFunctions: []string{"avg"},
	}, nil)
	// 3-day span → PT15M interval
	repo.On("GetBetweenDates", mock.Anything, "2025-01-15 00:00:00", "2025-01-18 00:00:00", "", "temperature", database.AggregationPT15M, database.AggregationFunctionAvg).Return(readings, nil)

	result, err := svc.ServiceGetBetweenDates(context.Background(), "2025-01-15 00:00:00", "2025-01-18 00:00:00", "", "temperature", "", "")

	assert.NoError(t, err)
	assert.Equal(t, gen.AggregatedReadingsResponseAggregationIntervalPT15M, result.AggregationInterval)
	assert.Equal(t, gen.AggregatedReadingsResponseAggregationFunctionAvg, result.AggregationFunction)
	assert.Len(t, result.Readings, 1)
}

func TestReadingsService_ServiceGetBetweenDates_OverrideInterval(t *testing.T) {
	svc, repo, mtRepo := setupReadingsService()

	readings := []gen.Reading{}
	mtRepo.On("GetAggregationsForMeasurementType", mock.Anything, "temperature").Return(&database.MeasurementTypeAggregation{
		MeasurementType:    "temperature",
		DefaultFunction:    "avg",
		SupportedFunctions: []string{"avg"},
	}, nil)
	repo.On("GetBetweenDates", mock.Anything, "2025-01-15 00:00:00", "2025-01-15 01:00:00", "", "temperature", database.AggregationInterval("PT1H"), database.AggregationFunctionAvg).Return(readings, nil)

	result, err := svc.ServiceGetBetweenDates(context.Background(), "2025-01-15 00:00:00", "2025-01-15 01:00:00", "", "temperature", "PT1H", "")

	assert.NoError(t, err)
	assert.Equal(t, gen.AggregatedReadingsResponseAggregationInterval("PT1H"), result.AggregationInterval)
}

func TestReadingsService_ServiceGetBetweenDates_OverrideFunction(t *testing.T) {
	svc, repo, mtRepo := setupReadingsService()

	readings := []gen.Reading{}
	mtRepo.On("GetAggregationsForMeasurementType", mock.Anything, "temperature").Return(&database.MeasurementTypeAggregation{
		MeasurementType:    "temperature",
		DefaultFunction:    "avg",
		SupportedFunctions: []string{"avg", "count", "last"},
	}, nil)
	repo.On("GetBetweenDates", mock.Anything, "2025-01-15 00:00:00", "2025-01-18 00:00:00", "", "temperature", database.AggregationPT15M, database.AggregationFunctionCount).Return(readings, nil)

	result, err := svc.ServiceGetBetweenDates(context.Background(), "2025-01-15 00:00:00", "2025-01-18 00:00:00", "", "temperature", "", "count")

	assert.NoError(t, err)
	assert.Equal(t, gen.AggregatedReadingsResponseAggregationFunctionCount, result.AggregationFunction)
}

func TestReadingsService_ServiceGetBetweenDates_UnsupportedFunction(t *testing.T) {
	svc, _, mtRepo := setupReadingsService()

	mtRepo.On("GetAggregationsForMeasurementType", mock.Anything, "motion").Return(&database.MeasurementTypeAggregation{
		MeasurementType:    "motion",
		DefaultFunction:    "count",
		SupportedFunctions: []string{"count", "last"},
	}, nil)

	result, err := svc.ServiceGetBetweenDates(context.Background(), "2025-01-15 00:00:00", "2025-01-18 00:00:00", "", "motion", "", "avg")

	assert.Error(t, err)
	assert.Nil(t, result)
	var unsupported *ErrUnsupportedAggregationFunction
	assert.True(t, errors.As(err, &unsupported))
	assert.Equal(t, "avg", unsupported.Function)
	assert.Equal(t, "motion", unsupported.MeasurementType)
	assert.Equal(t, []string{"count", "last"}, unsupported.Supported)
}

func TestReadingsService_ServiceGetBetweenDates_OverrideFunctionWithoutMeasurementType(t *testing.T) {
	svc, _, mtRepo := setupReadingsService()

	result, err := svc.ServiceGetBetweenDates(context.Background(), "2025-01-15 00:00:00", "2025-01-18 00:00:00", "", "", "", "count")

	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrMeasurementTypeRequiredForFunction)
	mtRepo.AssertNotCalled(t, "GetAggregationsForMeasurementType", mock.Anything, mock.Anything)
}

func TestReadingsService_ServiceGetBetweenDates_NoReadingsIsEmptySlice(t *testing.T) {
	svc, repo, _ := setupReadingsService()

	repo.On("GetBetweenDates", mock.Anything, "2025-01-15 10:00:00", "2025-01-15 10:05:00", "", "", database.AggregationRaw, database.AggregationFunctionNone).Return([]gen.Reading(nil), nil)

	result, err := svc.ServiceGetBetweenDates(context.Background(), "2025-01-15 10:00:00", "2025-01-15 10:05:00", "", "", "", "")

	require.NoError(t, err)
	assert.NotNil(t, result.Readings)
	assert.Empty(t, result.Readings)
}

func TestReadingsService_ServiceGetBetweenDates_DisabledAggregation(t *testing.T) {
	repo := new(MockReadingsRepository)
	mtRepo := new(MockMeasurementTypeRepository)
	svc := NewReadingsService(repo, mtRepo, DefaultAggregationTiers, false, slog.Default())

	val := 22.5
	readings := []gen.Reading{
		{Id: 1, SensorName: "LivingRoom", MeasurementType: "temperature", Unit: "°C", NumericValue: &val, Time: "2025-01-15 10:00:00"},
	}
	// Even for a 3-day range, disabled → raw
	repo.On("GetBetweenDates", mock.Anything, "2025-01-15 00:00:00", "2025-01-18 00:00:00", "", "", database.AggregationRaw, database.AggregationFunctionNone).Return(readings, nil)

	result, err := svc.ServiceGetBetweenDates(context.Background(), "2025-01-15 00:00:00", "2025-01-18 00:00:00", "", "", "", "")

	assert.NoError(t, err)
	assert.Equal(t, gen.AggregatedReadingsResponseAggregationIntervalRaw, result.AggregationInterval)
	assert.Equal(t, gen.AggregatedReadingsResponseAggregationFunctionNone, result.AggregationFunction)
}

func TestReadingsService_ServiceGetBetweenDates_Error(t *testing.T) {
	svc, repo, _ := setupReadingsService()

	repo.On("GetBetweenDates", mock.Anything, "2025-01-15 10:00:00", "2025-01-15 10:10:00", "", "", database.AggregationRaw, database.AggregationFunctionNone).Return([]gen.Reading{}, errors.New("database error"))

	result, err := svc.ServiceGetBetweenDates(context.Background(), "2025-01-15 10:00:00", "2025-01-15 10:10:00", "", "", "", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
	assert.Nil(t, result)
}

func TestReadingsService_ServiceGetBetweenDates_InvalidDateFormat(t *testing.T) {
	svc, _, _ := setupReadingsService()

	result, err := svc.ServiceGetBetweenDates(context.Background(), "not-a-date", "2025-01-15 10:00:00", "", "", "", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse start date")
	assert.Nil(t, result)
}

// ============================================================================
// ServiceGetLatest tests
// ============================================================================

func TestReadingsService_ServiceGetLatest_Success(t *testing.T) {
	svc, repo, _ := setupReadingsService()

	val1 := 22.5
	val2 := 20.0
	val3 := 23.5
	readings := []gen.Reading{
		{Id: 10, SensorName: "LivingRoom", MeasurementType: "temperature", Unit: "°C", NumericValue: &val1, Time: "2025-01-15 14:00:00"},
		{Id: 20, SensorName: "Bedroom", MeasurementType: "temperature", Unit: "°C", NumericValue: &val2, Time: "2025-01-15 14:00:00"},
		{Id: 30, SensorName: "Kitchen", MeasurementType: "temperature", Unit: "°C", NumericValue: &val3, Time: "2025-01-15 14:00:00"},
	}
	repo.On("GetLatest", mock.Anything).Return(readings, nil)

	result, err := svc.ServiceGetLatest(context.Background())

	assert.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Equal(t, "LivingRoom", result[0].SensorName)
}

func TestReadingsService_ServiceGetLatest_Empty(t *testing.T) {
	svc, repo, _ := setupReadingsService()

	repo.On("GetLatest", mock.Anything).Return([]gen.Reading{}, nil)

	result, err := svc.ServiceGetLatest(context.Background())

	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestReadingsService_ServiceGetLatest_Error(t *testing.T) {
	svc, repo, _ := setupReadingsService()

	repo.On("GetLatest", mock.Anything).Return([]gen.Reading{}, errors.New("database error"))

	result, err := svc.ServiceGetLatest(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
	assert.Nil(t, result)
}

// ============================================================================
// NewReadingsService tests
// ============================================================================

func TestNewReadingsService_ReturnsService(t *testing.T) {
	repo := new(MockReadingsRepository)
	mtRepo := new(MockMeasurementTypeRepository)
	svc := NewReadingsService(repo, mtRepo, DefaultAggregationTiers, true, slog.Default())

	assert.NotNil(t, svc)
}
