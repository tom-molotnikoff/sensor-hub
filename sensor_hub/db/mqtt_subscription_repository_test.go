package database

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"

	gen "example/sensorHub/gen"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var subscriptionColumns = []string{
	"id", "broker_id", "topic_pattern", "driver_type", "enabled", "created_at", "updated_at",
}

// ============================================================================
// Add tests
// ============================================================================

func TestMQTTSubscriptionRepository_Add_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTSubscriptionRepository(handles(db), slog.Default())

	sub := gen.MQTTSubscription{
		BrokerId: 1, TopicPattern: "zigbee2mqtt/+", DriverType: "mqtt-zigbee2mqtt", Enabled: true,
	}

	mock.ExpectExec("INSERT INTO mqtt_subscriptions").
		WithArgs(1, "zigbee2mqtt/+", "mqtt-zigbee2mqtt", true).
		WillReturnResult(sqlmock.NewResult(1, 1))

	id, err := repo.Add(context.Background(), sub)
	assert.NoError(t, err)
	assert.Equal(t, 1, id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMQTTSubscriptionRepository_Add_EmptyTopic(t *testing.T) {
	db, _ := newMockDB(t)
	repo := NewMQTTSubscriptionRepository(handles(db), slog.Default())

	_, err := repo.Add(context.Background(), gen.MQTTSubscription{BrokerId: 1, DriverType: "mqtt-zigbee2mqtt"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "topic pattern cannot be empty")
}

func TestMQTTSubscriptionRepository_Add_EmptyDriverType(t *testing.T) {
	db, _ := newMockDB(t)
	repo := NewMQTTSubscriptionRepository(handles(db), slog.Default())

	_, err := repo.Add(context.Background(), gen.MQTTSubscription{BrokerId: 1, TopicPattern: "test/+"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "driver type cannot be empty")
}

func TestMQTTSubscriptionRepository_Add_InvalidBrokerID(t *testing.T) {
	db, _ := newMockDB(t)
	repo := NewMQTTSubscriptionRepository(handles(db), slog.Default())

	_, err := repo.Add(context.Background(), gen.MQTTSubscription{TopicPattern: "test/+", DriverType: "mqtt-zigbee2mqtt"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "broker id must be positive")
}

// ============================================================================
// GetByID tests
// ============================================================================

func TestMQTTSubscriptionRepository_GetByID_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTSubscriptionRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT .+ FROM mqtt_subscriptions WHERE id = \\?").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows(subscriptionColumns).
			AddRow(1, 1, "zigbee2mqtt/+", "mqtt-zigbee2mqtt", true, "2025-01-01 00:00:00", "2025-01-01 00:00:00"))

	sub, err := repo.GetByID(context.Background(), 1)
	assert.NoError(t, err)
	require.NotNil(t, sub)
	assert.Equal(t, "zigbee2mqtt/+", sub.TopicPattern)
	assert.Equal(t, "mqtt-zigbee2mqtt", sub.DriverType)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMQTTSubscriptionRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTSubscriptionRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT .+ FROM mqtt_subscriptions WHERE id = \\?").
		WithArgs(99).
		WillReturnError(sql.ErrNoRows)

	sub, err := repo.GetByID(context.Background(), 99)
	assert.NoError(t, err)
	assert.Nil(t, sub)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GetAll tests
// ============================================================================

func TestMQTTSubscriptionRepository_GetAll_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTSubscriptionRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT .+ FROM mqtt_subscriptions ORDER BY").
		WillReturnRows(sqlmock.NewRows(subscriptionColumns).
			AddRow(1, 1, "zigbee2mqtt/+", "mqtt-zigbee2mqtt", true, "2025-01-01 00:00:00", "2025-01-01 00:00:00").
			AddRow(2, 1, "rtl_433/+/+", "mqtt-rtl433", true, "2025-01-01 00:00:00", "2025-01-01 00:00:00"))

	subs, err := repo.GetAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, subs, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GetByBrokerID tests
// ============================================================================

func TestMQTTSubscriptionRepository_GetByBrokerID_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTSubscriptionRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT .+ FROM mqtt_subscriptions WHERE broker_id = \\?").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows(subscriptionColumns).
			AddRow(1, 1, "zigbee2mqtt/+", "mqtt-zigbee2mqtt", true, "2025-01-01 00:00:00", "2025-01-01 00:00:00"))

	subs, err := repo.GetByBrokerID(context.Background(), 1)
	assert.NoError(t, err)
	assert.Len(t, subs, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GetEnabledByBrokerID tests
// ============================================================================

func TestMQTTSubscriptionRepository_GetEnabledByBrokerID_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTSubscriptionRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT .+ FROM mqtt_subscriptions WHERE broker_id = \\? AND enabled = 1").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows(subscriptionColumns).
			AddRow(1, 1, "zigbee2mqtt/+", "mqtt-zigbee2mqtt", true, "2025-01-01 00:00:00", "2025-01-01 00:00:00"))

	subs, err := repo.GetEnabledByBrokerID(context.Background(), 1)
	assert.NoError(t, err)
	assert.Len(t, subs, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMQTTSubscriptionRepository_GetEnabledByDriverType_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTSubscriptionRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT .+ FROM mqtt_subscriptions WHERE driver_type = \\? AND enabled = 1 ORDER BY id LIMIT 1").
		WithArgs("mqtt-zigbee2mqtt").
		WillReturnRows(sqlmock.NewRows(subscriptionColumns).
			AddRow(3, 12, "zigbee2mqtt/+", "mqtt-zigbee2mqtt", true, "2025-01-01 00:00:00", "2025-01-01 00:00:00"))

	sub, err := repo.GetEnabledByDriverType(context.Background(), "mqtt-zigbee2mqtt")
	assert.NoError(t, err)
	require.NotNil(t, sub)
	assert.Equal(t, 12, sub.BrokerId)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMQTTSubscriptionRepository_GetEnabledByDriverType_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTSubscriptionRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT .+ FROM mqtt_subscriptions WHERE driver_type = \\? AND enabled = 1 ORDER BY id LIMIT 1").
		WithArgs("mqtt-zigbee2mqtt").
		WillReturnError(sql.ErrNoRows)

	sub, err := repo.GetEnabledByDriverType(context.Background(), "mqtt-zigbee2mqtt")
	assert.NoError(t, err)
	assert.Nil(t, sub)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// Update tests
// ============================================================================

func TestMQTTSubscriptionRepository_Update_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTSubscriptionRepository(handles(db), slog.Default())

	mock.ExpectExec("UPDATE mqtt_subscriptions SET").
		WithArgs(1, "zigbee2mqtt/#", "mqtt-zigbee2mqtt", true, 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	id1 := 1
	err := repo.Update(context.Background(), gen.MQTTSubscription{
		Id: &id1, BrokerId: 1, TopicPattern: "zigbee2mqtt/#", DriverType: "mqtt-zigbee2mqtt", Enabled: true,
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMQTTSubscriptionRepository_Update_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTSubscriptionRepository(handles(db), slog.Default())

	mock.ExpectExec("UPDATE mqtt_subscriptions SET").
		WithArgs(1, "test/+", "mqtt-zigbee2mqtt", true, 99).
		WillReturnResult(sqlmock.NewResult(0, 0))

	id99 := 99
	err := repo.Update(context.Background(), gen.MQTTSubscription{
		Id: &id99, BrokerId: 1, TopicPattern: "test/+", DriverType: "mqtt-zigbee2mqtt", Enabled: true,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no MQTT subscription found with id 99")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// Delete tests
// ============================================================================

func TestMQTTSubscriptionRepository_Delete_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTSubscriptionRepository(handles(db), slog.Default())

	mock.ExpectExec("DELETE FROM mqtt_subscriptions WHERE id = \\?").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Delete(context.Background(), 1)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMQTTSubscriptionRepository_Delete_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTSubscriptionRepository(handles(db), slog.Default())

	mock.ExpectExec("DELETE FROM mqtt_subscriptions WHERE id = \\?").
		WithArgs(99).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.Delete(context.Background(), 99)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no MQTT subscription found with id 99")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// DB Error tests
// ============================================================================

func TestMQTTSubscriptionRepository_Add_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTSubscriptionRepository(handles(db), slog.Default())

	mock.ExpectExec("INSERT INTO mqtt_subscriptions").
		WithArgs(1, "zigbee2mqtt/+", "mqtt-zigbee2mqtt", true).
		WillReturnError(errors.New("foreign key constraint"))

	_, err := repo.Add(context.Background(), gen.MQTTSubscription{
		BrokerId: 1, TopicPattern: "zigbee2mqtt/+", DriverType: "mqtt-zigbee2mqtt", Enabled: true,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error adding MQTT subscription")
	assert.NoError(t, mock.ExpectationsWereMet())
}
