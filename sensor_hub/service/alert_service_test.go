package service

import (
	"context"
	"example/sensorHub/alerting"
	gen "example/sensorHub/gen"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockAlertRepositoryForService struct {
	mock.Mock
}

func (m *mockAlertRepositoryForService) GetAlertRuleBySensorID(ctx context.Context, sensorID int) (*alerting.AlertRule, error) {
	args := m.Called(ctx, sensorID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*alerting.AlertRule), args.Error(1)
}

func (m *mockAlertRepositoryForService) GetAlertRuleByID(ctx context.Context, ruleID int) (*alerting.AlertRule, error) {
	args := m.Called(ctx, ruleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*alerting.AlertRule), args.Error(1)
}

func (m *mockAlertRepositoryForService) GetAlertRulesBySensorID(ctx context.Context, sensorID int) ([]alerting.AlertRule, error) {
	args := m.Called(ctx, sensorID)
	return args.Get(0).([]alerting.AlertRule), args.Error(1)
}

func (m *mockAlertRepositoryForService) GetAlertRuleForReading(ctx context.Context, sensorID int, measurementTypeName string) (*alerting.AlertRule, error) {
	args := m.Called(ctx, sensorID, measurementTypeName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*alerting.AlertRule), args.Error(1)
}

func (m *mockAlertRepositoryForService) UpdateLastAlertSent(ctx context.Context, ruleID int) error {
	args := m.Called(ctx, ruleID)
	return args.Error(0)
}

func (m *mockAlertRepositoryForService) RecordAlertSent(ctx context.Context, ruleID, sensorID, measurementTypeId int, reason string, numericValue float64, statusValue string) error {
	args := m.Called(ctx, ruleID, sensorID, measurementTypeId, reason, numericValue, statusValue)
	return args.Error(0)
}

func (m *mockAlertRepositoryForService) GetAllAlertRules(ctx context.Context) ([]alerting.AlertRule, error) {
	args := m.Called(ctx)
	return args.Get(0).([]alerting.AlertRule), args.Error(1)
}

func (m *mockAlertRepositoryForService) GetAlertRuleBySensorName(ctx context.Context, sensorName string) (*alerting.AlertRule, error) {
	args := m.Called(ctx, sensorName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*alerting.AlertRule), args.Error(1)
}

func (m *mockAlertRepositoryForService) CreateAlertRule(ctx context.Context, rule *alerting.AlertRule) error {
	args := m.Called(ctx, rule)
	return args.Error(0)
}

func (m *mockAlertRepositoryForService) UpdateAlertRule(ctx context.Context, rule *alerting.AlertRule) error {
	args := m.Called(ctx, rule)
	return args.Error(0)
}

func (m *mockAlertRepositoryForService) DeleteAlertRule(ctx context.Context, ruleID int) error {
	args := m.Called(ctx, ruleID)
	return args.Error(0)
}

func (m *mockAlertRepositoryForService) GetAlertHistory(ctx context.Context, sensorID int, limit int) ([]gen.AlertHistoryEntry, error) {
	args := m.Called(ctx, sensorID, limit)
	return args.Get(0).([]gen.AlertHistoryEntry), args.Error(1)
}

func (m *mockAlertRepositoryForService) DeleteAlertHistoryOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	args := m.Called(ctx, cutoff)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockAlertRepositoryForService) GetAlertRule(ctx context.Context, sensorID, measurementTypeId int) (*alerting.AlertRule, error) {
	args := m.Called(ctx, sensorID, measurementTypeId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*alerting.AlertRule), args.Error(1)
}

func TestServiceGetAllAlertRules(t *testing.T) {
	mockRepo := new(mockAlertRepositoryForService)
	service := NewAlertManagementService(mockRepo, nil, slog.Default())

	expectedRules := []alerting.AlertRule{
		{SensorID: 1, SensorName: "Sensor1", AlertType: alerting.AlertTypeNumericRange},
		{SensorID: 2, SensorName: "Sensor2", AlertType: alerting.AlertTypeStatusBased},
	}

	mockRepo.On("GetAllAlertRules", mock.Anything).Return(expectedRules, nil)

	rules, err := service.ServiceGetAllAlertRules(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, 2, len(rules))
	assert.Equal(t, "Sensor1", rules[0].SensorName)
	mockRepo.AssertExpectations(t)
}

func TestServiceGetAlertRuleBySensorID(t *testing.T) {
	mockRepo := new(mockAlertRepositoryForService)
	service := NewAlertManagementService(mockRepo, nil, slog.Default())

	expectedRule := &alerting.AlertRule{
		SensorID:   1,
		SensorName: "TestSensor",
		AlertType:  alerting.AlertTypeNumericRange,
	}

	mockRepo.On("GetAlertRuleBySensorID", mock.Anything, 1).Return(expectedRule, nil)

	rule, err := service.ServiceGetAlertRuleBySensorID(context.Background(), 1)

	assert.NoError(t, err)
	assert.Equal(t, "TestSensor", rule.SensorName)
	mockRepo.AssertExpectations(t)
}

func TestServiceCreateAlertRule(t *testing.T) {
	mockRepo := new(mockAlertRepositoryForService)
	service := NewAlertManagementService(mockRepo, nil, slog.Default())

	newRule := &alerting.AlertRule{
		SensorID:         1,
		AlertType:        alerting.AlertTypeNumericRange,
		HighThreshold:    30.0,
		LowThreshold:     10.0,
		RateLimitSeconds: 1,
		Enabled:          true,
	}

	mockRepo.On("CreateAlertRule", mock.Anything, newRule).Return(nil)

	err := service.ServiceCreateAlertRule(context.Background(), newRule)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestServiceUpdateAlertRule(t *testing.T) {
	mockRepo := new(mockAlertRepositoryForService)
	service := NewAlertManagementService(mockRepo, nil, slog.Default())

	updatedRule := &alerting.AlertRule{
		SensorID:         1,
		AlertType:        alerting.AlertTypeNumericRange,
		HighThreshold:    35.0,
		LowThreshold:     12.0,
		RateLimitSeconds: 2,
		Enabled:          false,
	}

	mockRepo.On("UpdateAlertRule", mock.Anything, updatedRule).Return(nil)

	err := service.ServiceUpdateAlertRule(context.Background(), updatedRule)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestServiceDeleteAlertRule(t *testing.T) {
	mockRepo := new(mockAlertRepositoryForService)
	service := NewAlertManagementService(mockRepo, nil, slog.Default())

	mockRepo.On("DeleteAlertRule", mock.Anything, 1).Return(nil)

	err := service.ServiceDeleteAlertRule(context.Background(), 1)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestServiceGetAlertHistory(t *testing.T) {
	mockRepo := new(mockAlertRepositoryForService)
	service := NewAlertManagementService(mockRepo, nil, slog.Default())

	expectedHistory := []gen.AlertHistoryEntry{
		{SensorId: 1, AlertType: "numeric_range", ReadingValue: "35.5", SentAt: time.Now()},
		{SensorId: 1, AlertType: "numeric_range", ReadingValue: "40.0", SentAt: time.Now().Add(-2 * time.Hour)},
	}

	mockRepo.On("GetAlertHistory", mock.Anything, 1, 10).Return(expectedHistory, nil)

	history, err := service.ServiceGetAlertHistory(context.Background(), 1, 10)

	assert.NoError(t, err)
	assert.Equal(t, 2, len(history))
	assert.Equal(t, 1, history[0].SensorId)
	mockRepo.AssertExpectations(t)
}

type spyRuleCache struct {
	invalidations int
}

func (s *spyRuleCache) InvalidateRules() { s.invalidations++ }

func TestAlertManagementService_RuleWritesInvalidateTheRuleCache(t *testing.T) {
	rule := &alerting.AlertRule{SensorID: 1, MeasurementTypeId: 1, AlertType: alerting.AlertTypeNumericRange}

	for _, tc := range []struct {
		name  string
		setup func(*mockAlertRepositoryForService)
		write func(AlertManagementServiceInterface) error
	}{
		{
			name:  "create",
			setup: func(m *mockAlertRepositoryForService) { m.On("CreateAlertRule", mock.Anything, rule).Return(nil) },
			write: func(s AlertManagementServiceInterface) error {
				return s.ServiceCreateAlertRule(context.Background(), rule)
			},
		},
		{
			name:  "update",
			setup: func(m *mockAlertRepositoryForService) { m.On("UpdateAlertRule", mock.Anything, rule).Return(nil) },
			write: func(s AlertManagementServiceInterface) error {
				return s.ServiceUpdateAlertRule(context.Background(), rule)
			},
		},
		{
			name:  "delete",
			setup: func(m *mockAlertRepositoryForService) { m.On("DeleteAlertRule", mock.Anything, 7).Return(nil) },
			write: func(s AlertManagementServiceInterface) error {
				return s.ServiceDeleteAlertRule(context.Background(), 7)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := new(mockAlertRepositoryForService)
			cache := &spyRuleCache{}
			tc.setup(mockRepo)

			require.NoError(t, tc.write(NewAlertManagementService(mockRepo, cache, slog.Default())))

			assert.Equal(t, 1, cache.invalidations, "the next reading reads the rules again")
		})
	}
}
