package database

import (
	"context"
	"database/sql"
	"errors"
	"example/sensorHub/alerting"
	gen "example/sensorHub/gen"
	"log/slog"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockAlertRepository is kept for use by other packages
type MockAlertRepository struct {
	mock.Mock
}

func (m *MockAlertRepository) GetAlertRuleBySensorID(ctx context.Context, sensorID int) (*alerting.AlertRule, error) {
	args := m.Called(ctx, sensorID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*alerting.AlertRule), args.Error(1)
}

func (m *MockAlertRepository) GetAlertRuleByID(ctx context.Context, ruleID int) (*alerting.AlertRule, error) {
	args := m.Called(ctx, ruleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*alerting.AlertRule), args.Error(1)
}

func (m *MockAlertRepository) GetAlertRulesBySensorID(ctx context.Context, sensorID int) ([]alerting.AlertRule, error) {
	args := m.Called(ctx, sensorID)
	return args.Get(0).([]alerting.AlertRule), args.Error(1)
}

func (m *MockAlertRepository) GetAlertRuleForReading(ctx context.Context, sensorID int, measurementTypeName string) (*alerting.AlertRule, error) {
	args := m.Called(ctx, sensorID, measurementTypeName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*alerting.AlertRule), args.Error(1)
}

func (m *MockAlertRepository) UpdateLastAlertSent(ctx context.Context, ruleID int) error {
	args := m.Called(ctx, ruleID)
	return args.Error(0)
}

func (m *MockAlertRepository) RecordAlertSent(ctx context.Context, ruleID, sensorID, measurementTypeId int, reason string, numericValue float64, statusValue string) error {
	args := m.Called(ctx, ruleID, sensorID, measurementTypeId, reason, numericValue, statusValue)
	return args.Error(0)
}

func (m *MockAlertRepository) GetAllAlertRules(ctx context.Context) ([]alerting.AlertRule, error) {
	args := m.Called(ctx)
	return args.Get(0).([]alerting.AlertRule), args.Error(1)
}

func (m *MockAlertRepository) GetAlertRuleBySensorName(ctx context.Context, sensorName string) (*alerting.AlertRule, error) {
	args := m.Called(ctx, sensorName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*alerting.AlertRule), args.Error(1)
}

func (m *MockAlertRepository) CreateAlertRule(ctx context.Context, rule *alerting.AlertRule) error {
	args := m.Called(ctx, rule)
	return args.Error(0)
}

func (m *MockAlertRepository) UpdateAlertRule(ctx context.Context, rule *alerting.AlertRule) error {
	args := m.Called(ctx, rule)
	return args.Error(0)
}

func (m *MockAlertRepository) DeleteAlertRule(ctx context.Context, ruleID int) error {
	args := m.Called(ctx, ruleID)
	return args.Error(0)
}

func (m *MockAlertRepository) GetAlertHistory(ctx context.Context, sensorID int, limit int) ([]gen.AlertHistoryEntry, error) {
	args := m.Called(ctx, sensorID, limit)
	return args.Get(0).([]gen.AlertHistoryEntry), args.Error(1)
}

func (m *MockAlertRepository) DeleteAlertHistoryOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	args := m.Called(ctx, cutoff)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockAlertRepository) GetAlertRule(ctx context.Context, sensorID, measurementTypeId int) (*alerting.AlertRule, error) {
	args := m.Called(ctx, sensorID, measurementTypeId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*alerting.AlertRule), args.Error(1)
}

// ============================================================================
// GetAlertRuleBySensorID tests (implementation)
// ============================================================================

func TestAlertRepository_GetAlertRuleBySensorID_Success(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	now := time.Now()
	dbMock.ExpectQuery("SELECT").
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows(alertRuleColumns).
			AddRow(1, 5, "sensor-1", 1, "temperature", "numeric_range", 30.0, 10.0, nil, true, 1, now))

	rule, err := repo.GetAlertRuleBySensorID(context.Background(), 5)

	assert.NoError(t, err)
	require.NotNil(t, rule)
	assert.Equal(t, 1, rule.ID)
	assert.Equal(t, 5, rule.SensorID)
	assert.Equal(t, alerting.AlertTypeNumericRange, rule.AlertType)
	assert.Equal(t, 30.0, rule.HighThreshold)
	assert.Equal(t, 10.0, rule.LowThreshold)
	assert.True(t, rule.Enabled)
	require.NotNil(t, rule.LastAlertSentAt)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAlertRepository_GetAlertRuleBySensorID_NotFound(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	dbMock.ExpectQuery("SELECT").
		WithArgs(999).
		WillReturnError(sql.ErrNoRows)

	rule, err := repo.GetAlertRuleBySensorID(context.Background(), 999)

	assert.NoError(t, err)
	assert.Nil(t, rule)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAlertRepository_GetAlertRuleBySensorID_NullLastAlertSent(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	dbMock.ExpectQuery("SELECT").
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows(alertRuleColumns).
			AddRow(1, 5, "sensor-1", 1, "temperature", "numeric_range", 30.0, 10.0, nil, true, 1, nil))

	rule, err := repo.GetAlertRuleBySensorID(context.Background(), 5)

	assert.NoError(t, err)
	require.NotNil(t, rule)
	assert.Nil(t, rule.LastAlertSentAt)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAlertRepository_GetAlertRuleBySensorID_WithTriggerStatus(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	dbMock.ExpectQuery("SELECT").
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows(alertRuleColumns).
			AddRow(1, 5, "sensor-1", 1, "contact", "status_based", 0.0, 0.0, "bad", true, 1, nil))

	rule, err := repo.GetAlertRuleBySensorID(context.Background(), 5)

	assert.NoError(t, err)
	require.NotNil(t, rule)
	assert.Equal(t, alerting.AlertTypeStatusBased, rule.AlertType)
	assert.Equal(t, "bad", rule.TriggerStatus)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAlertRepository_GetAlertRuleBySensorID_DBError(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	dbMock.ExpectQuery("SELECT").
		WithArgs(5).
		WillReturnError(errors.New("database error"))

	rule, err := repo.GetAlertRuleBySensorID(context.Background(), 5)

	assert.Error(t, err)
	assert.Nil(t, rule)
	assert.Contains(t, err.Error(), "failed to get alert rule")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

// ============================================================================
// RecordAlertSent tests
// ============================================================================

func TestAlertRepository_RecordAlertSent_Success(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	dbMock.ExpectExec("INSERT INTO alert_sent_history").
		WithArgs(1, 5, 1, "temperature too high", 35.5, "").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.RecordAlertSent(context.Background(), 1, 5, 1, "temperature too high", 35.5, "")

	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAlertRepository_RecordAlertSent_WithStatusValue(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	dbMock.ExpectExec("INSERT INTO alert_sent_history").
		WithArgs(1, 5, 1, "sensor status changed", 0.0, "bad").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.RecordAlertSent(context.Background(), 1, 5, 1, "sensor status changed", 0.0, "bad")

	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAlertRepository_RecordAlertSent_DBError(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	dbMock.ExpectExec("INSERT INTO alert_sent_history").
		WithArgs(1, 5, 1, "test", 0.0, "").
		WillReturnError(errors.New("database error"))

	err := repo.RecordAlertSent(context.Background(), 1, 5, 1, "test", 0.0, "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to record alert sent")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

// ============================================================================
// GetAllAlertRules tests
// ============================================================================

func TestAlertRepository_GetAllAlertRules_Success(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	now := time.Now()
	dbMock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(alertRuleColumns).
			AddRow(1, 1, "sensor-1", 1, "temperature", "numeric_range", 30.0, 10.0, nil, true, 1, now).
			AddRow(2, 2, "sensor-2", 1, "temperature", "status_based", 0.0, 0.0, "bad", true, 2, nil))

	rules, err := repo.GetAllAlertRules(context.Background())

	assert.NoError(t, err)
	assert.Len(t, rules, 2)
	assert.Equal(t, "sensor-1", rules[0].SensorName)
	assert.Equal(t, "sensor-2", rules[1].SensorName)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAlertRepository_GetAllAlertRules_Empty(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	dbMock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(alertRuleColumns))

	rules, err := repo.GetAllAlertRules(context.Background())

	assert.NoError(t, err)
	assert.Empty(t, rules)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAlertRepository_GetAllAlertRules_DBError(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	dbMock.ExpectQuery("SELECT").
		WillReturnError(errors.New("database error"))

	rules, err := repo.GetAllAlertRules(context.Background())

	assert.Error(t, err)
	assert.Nil(t, rules)
	assert.Contains(t, err.Error(), "failed to get all alert rules")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

// ============================================================================
// GetAlertRuleBySensorName tests
// ============================================================================

func TestAlertRepository_GetAlertRuleBySensorName_Success(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	dbMock.ExpectQuery("SELECT").
		WithArgs("sensor-1").
		WillReturnRows(sqlmock.NewRows(alertRuleColumns).
			AddRow(1, 1, "sensor-1", 1, "temperature", "numeric_range", 30.0, 10.0, nil, true, 1, nil))

	rule, err := repo.GetAlertRuleBySensorName(context.Background(), "sensor-1")

	assert.NoError(t, err)
	require.NotNil(t, rule)
	assert.Equal(t, "sensor-1", rule.SensorName)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAlertRepository_GetAlertRuleBySensorName_NotFound(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	dbMock.ExpectQuery("SELECT").
		WithArgs("nonexistent").
		WillReturnError(sql.ErrNoRows)

	rule, err := repo.GetAlertRuleBySensorName(context.Background(), "nonexistent")

	assert.Error(t, err)
	assert.Nil(t, rule)
	assert.Contains(t, err.Error(), "alert rule not found")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAlertRepository_GetAlertRuleBySensorName_DBError(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	dbMock.ExpectQuery("SELECT").
		WithArgs("sensor-1").
		WillReturnError(errors.New("database error"))

	rule, err := repo.GetAlertRuleBySensorName(context.Background(), "sensor-1")

	assert.Error(t, err)
	assert.Nil(t, rule)
	assert.Contains(t, err.Error(), "failed to get alert rule")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

// ============================================================================
// CreateAlertRule tests
// ============================================================================

func TestAlertRepository_CreateAlertRule_Success(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	rule := &alerting.AlertRule{
		SensorID:          1,
		MeasurementTypeId: 1,
		AlertType:         alerting.AlertTypeNumericRange,
		HighThreshold:     30.0,
		LowThreshold:      10.0,
		RateLimitSeconds:  1,
		Enabled:           true,
	}

	dbMock.ExpectExec("INSERT INTO sensor_alert_rules").
		WithArgs(1, 1, alerting.AlertTypeNumericRange, 30.0, 10.0, "", 1, true).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.CreateAlertRule(context.Background(), rule)

	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAlertRepository_CreateAlertRule_StatusBased(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	rule := &alerting.AlertRule{
		SensorID:          1,
		MeasurementTypeId: 1,
		AlertType:         alerting.AlertTypeStatusBased,
		TriggerStatus:     "bad",
		RateLimitSeconds:  2,
		Enabled:           true,
	}

	dbMock.ExpectExec("INSERT INTO sensor_alert_rules").
		WithArgs(1, 1, alerting.AlertTypeStatusBased, 0.0, 0.0, "bad", 2, true).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.CreateAlertRule(context.Background(), rule)

	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAlertRepository_CreateAlertRule_DBError(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	rule := &alerting.AlertRule{
		SensorID:          1,
		MeasurementTypeId: 1,
		AlertType:         alerting.AlertTypeNumericRange,
	}

	dbMock.ExpectExec("INSERT INTO sensor_alert_rules").
		WithArgs(1, 1, alerting.AlertTypeNumericRange, 0.0, 0.0, "", 0, false).
		WillReturnError(errors.New("duplicate entry"))

	err := repo.CreateAlertRule(context.Background(), rule)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create alert rule")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

// ============================================================================
// UpdateAlertRule tests
// ============================================================================

func TestAlertRepository_UpdateAlertRule_Success(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	rule := &alerting.AlertRule{
		ID:               1,
		SensorID:         1,
		AlertType:        alerting.AlertTypeNumericRange,
		HighThreshold:    35.0,
		LowThreshold:     12.0,
		RateLimitSeconds: 2,
		Enabled:          false,
	}

	dbMock.ExpectExec("UPDATE sensor_alert_rules").
		WithArgs(alerting.AlertTypeNumericRange, 35.0, 12.0, "", 2, false, 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateAlertRule(context.Background(), rule)

	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAlertRepository_UpdateAlertRule_DBError(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	rule := &alerting.AlertRule{
		ID: 1,
	}

	dbMock.ExpectExec("UPDATE sensor_alert_rules").
		WithArgs(alerting.AlertType(""), 0.0, 0.0, "", 0, false, 1).
		WillReturnError(errors.New("database error"))

	err := repo.UpdateAlertRule(context.Background(), rule)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update alert rule")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

// ============================================================================
// DeleteAlertRule tests
// ============================================================================

func TestAlertRepository_DeleteAlertRule_Success(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	dbMock.ExpectExec("DELETE FROM sensor_alert_rules WHERE id = \\?").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.DeleteAlertRule(context.Background(), 1)

	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAlertRepository_DeleteAlertRule_NotFound(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	dbMock.ExpectExec("DELETE FROM sensor_alert_rules WHERE id = \\?").
		WithArgs(999).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.DeleteAlertRule(context.Background(), 999)

	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAlertRepository_DeleteAlertRule_DBError(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	dbMock.ExpectExec("DELETE FROM sensor_alert_rules WHERE id = \\?").
		WithArgs(1).
		WillReturnError(errors.New("database error"))

	err := repo.DeleteAlertRule(context.Background(), 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete alert rule")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

// ============================================================================
// GetAlertHistory tests
// ============================================================================

func TestAlertRepository_GetAlertHistory_Success(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	now := time.Now()
	dbMock.ExpectQuery("SELECT").
		WithArgs(1, 10).
		WillReturnRows(sqlmock.NewRows(alertHistoryColumns).
			AddRow(1, 1, "numeric_range", 35.5, now).
			AddRow(2, 1, "numeric_range", 36.0, now.Add(-time.Hour)))

	history, err := repo.GetAlertHistory(context.Background(), 1, 10)

	assert.NoError(t, err)
	assert.Len(t, history, 2)
	assert.Equal(t, "35.50", history[0].ReadingValue)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAlertRepository_GetAlertHistory_Empty(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	dbMock.ExpectQuery("SELECT").
		WithArgs(1, 10).
		WillReturnRows(sqlmock.NewRows(alertHistoryColumns))

	history, err := repo.GetAlertHistory(context.Background(), 1, 10)

	assert.NoError(t, err)
	assert.Empty(t, history)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAlertRepository_GetAlertHistory_NullReadingValue(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	now := time.Now()
	dbMock.ExpectQuery("SELECT").
		WithArgs(1, 10).
		WillReturnRows(sqlmock.NewRows(alertHistoryColumns).
			AddRow(1, 1, "status_based", nil, now))

	history, err := repo.GetAlertHistory(context.Background(), 1, 10)

	assert.NoError(t, err)
	assert.Len(t, history, 1)
	assert.Empty(t, history[0].ReadingValue)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAlertRepository_GetAlertHistory_DBError(t *testing.T) {
	db, dbMock := newMockDB(t)
	repo := NewAlertRepository(handles(db), slog.Default())

	dbMock.ExpectQuery("SELECT").
		WithArgs(1, 10).
		WillReturnError(errors.New("database error"))

	history, err := repo.GetAlertHistory(context.Background(), 1, 10)

	assert.Error(t, err)
	assert.Nil(t, history)
	assert.Contains(t, err.Error(), "failed to get alert history")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}
