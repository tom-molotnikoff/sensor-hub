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

var brokerColumns = []string{
	"id", "name", "type", "host", "port", "username", "password", "client_id",
	"ca_cert_path", "client_cert_path", "client_key_path", "enabled", "created_at", "updated_at",
}

// ============================================================================
// Add tests
// ============================================================================

func TestMQTTBrokerRepository_Add_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTBrokerRepository(handles(db), slog.Default())

	broker := gen.MQTTBroker{
		Name: "test-broker", Type: "external", Host: "mqtt.example.com", Port: 1883, Enabled: true,
	}

	mock.ExpectExec("INSERT INTO mqtt_brokers").
		WithArgs("test-broker", "external", "mqtt.example.com", 1883,
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), true).
		WillReturnResult(sqlmock.NewResult(1, 1))

	id, err := repo.Add(context.Background(), broker)
	assert.NoError(t, err)
	assert.Equal(t, 1, id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMQTTBrokerRepository_Add_EmptyName(t *testing.T) {
	db, _ := newMockDB(t)
	repo := NewMQTTBrokerRepository(handles(db), slog.Default())

	_, err := repo.Add(context.Background(), gen.MQTTBroker{Host: "localhost", Port: 1883})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "broker name cannot be empty")
}

func TestMQTTBrokerRepository_Add_EmptyHost(t *testing.T) {
	db, _ := newMockDB(t)
	repo := NewMQTTBrokerRepository(handles(db), slog.Default())

	_, err := repo.Add(context.Background(), gen.MQTTBroker{Name: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "broker host cannot be empty")
}

// ============================================================================
// GetByID tests
// ============================================================================

func TestMQTTBrokerRepository_GetByID_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTBrokerRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT .+ FROM mqtt_brokers WHERE id = \\?").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows(brokerColumns).
			AddRow(1, "test-broker", "external", "mqtt.example.com", 1883,
				nil, nil, nil, nil, nil, nil, true, "2025-01-01 00:00:00", "2025-01-01 00:00:00"))

	broker, err := repo.GetByID(context.Background(), 1)
	assert.NoError(t, err)
	require.NotNil(t, broker)
	assert.Equal(t, "test-broker", broker.Name)
	assert.Equal(t, "mqtt.example.com", broker.Host)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMQTTBrokerRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTBrokerRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT .+ FROM mqtt_brokers WHERE id = \\?").
		WithArgs(99).
		WillReturnError(sql.ErrNoRows)

	broker, err := repo.GetByID(context.Background(), 99)
	assert.NoError(t, err)
	assert.Nil(t, broker)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GetByName tests
// ============================================================================

func TestMQTTBrokerRepository_GetByName_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTBrokerRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT .+ FROM mqtt_brokers WHERE LOWER\\(name\\) = LOWER\\(\\?\\)").
		WithArgs("test-broker").
		WillReturnRows(sqlmock.NewRows(brokerColumns).
			AddRow(1, "test-broker", "external", "mqtt.example.com", 1883,
				"user", "pass", "sensor-hub-1", nil, nil, nil, true, "2025-01-01 00:00:00", "2025-01-01 00:00:00"))

	broker, err := repo.GetByName(context.Background(), "test-broker")
	assert.NoError(t, err)
	require.NotNil(t, broker)
	assert.Equal(t, "test-broker", broker.Name)
	assert.Equal(t, "user", *broker.Username)
	assert.Equal(t, "pass", *broker.Password)
	assert.Equal(t, "sensor-hub-1", *broker.ClientId)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GetAll tests
// ============================================================================

func TestMQTTBrokerRepository_GetAll_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTBrokerRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT .+ FROM mqtt_brokers ORDER BY name").
		WillReturnRows(sqlmock.NewRows(brokerColumns).
			AddRow(1, "broker-a", "embedded", "localhost", 1883,
				nil, nil, nil, nil, nil, nil, true, "2025-01-01 00:00:00", "2025-01-01 00:00:00").
			AddRow(2, "broker-b", "external", "mqtt.example.com", 8883,
				"user", "pass", nil, "/ca.crt", "/client.crt", "/client.key", true, "2025-01-01 00:00:00", "2025-01-01 00:00:00"))

	brokers, err := repo.GetAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, brokers, 2)
	assert.Equal(t, "broker-a", brokers[0].Name)
	assert.Equal(t, "broker-b", brokers[1].Name)
	assert.Equal(t, "/ca.crt", *brokers[1].CaCertPath)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMQTTBrokerRepository_GetAll_Empty(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTBrokerRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT .+ FROM mqtt_brokers ORDER BY name").
		WillReturnRows(sqlmock.NewRows(brokerColumns))

	brokers, err := repo.GetAll(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, brokers)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// Update tests
// ============================================================================

func TestMQTTBrokerRepository_Update_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTBrokerRepository(handles(db), slog.Default())

	mock.ExpectExec("UPDATE mqtt_brokers SET").
		WithArgs("updated-broker", "external", "new-host.com", 8883,
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), true, 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	id1 := 1
	err := repo.Update(context.Background(), gen.MQTTBroker{
		Id: &id1, Name: "updated-broker", Type: "external", Host: "new-host.com", Port: 8883, Enabled: true,
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMQTTBrokerRepository_Update_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTBrokerRepository(handles(db), slog.Default())

	mock.ExpectExec("UPDATE mqtt_brokers SET").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 99).
		WillReturnResult(sqlmock.NewResult(0, 0))

	id99 := 99
	err := repo.Update(context.Background(), gen.MQTTBroker{Id: &id99, Name: "x", Type: "external", Host: "h", Port: 1883})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no MQTT broker found with id 99")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// Delete tests
// ============================================================================

func TestMQTTBrokerRepository_Delete_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTBrokerRepository(handles(db), slog.Default())

	mock.ExpectExec("DELETE FROM mqtt_brokers WHERE id = \\?").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Delete(context.Background(), 1)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMQTTBrokerRepository_Delete_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTBrokerRepository(handles(db), slog.Default())

	mock.ExpectExec("DELETE FROM mqtt_brokers WHERE id = \\?").
		WithArgs(99).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.Delete(context.Background(), 99)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no MQTT broker found with id 99")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GetEnabled tests
// ============================================================================

func TestMQTTBrokerRepository_GetEnabled_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTBrokerRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT .+ FROM mqtt_brokers WHERE enabled = 1").
		WillReturnRows(sqlmock.NewRows(brokerColumns).
			AddRow(1, "enabled-broker", "external", "mqtt.example.com", 1883,
				nil, nil, nil, nil, nil, nil, true, "2025-01-01 00:00:00", "2025-01-01 00:00:00"))

	brokers, err := repo.GetEnabled(context.Background())
	assert.NoError(t, err)
	assert.Len(t, brokers, 1)
	assert.Equal(t, "enabled-broker", brokers[0].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// DBError tests
// ============================================================================

func TestMQTTBrokerRepository_Add_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMQTTBrokerRepository(handles(db), slog.Default())

	mock.ExpectExec("INSERT INTO mqtt_brokers").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(errors.New("unique constraint"))

	_, err := repo.Add(context.Background(), gen.MQTTBroker{Name: "dup", Type: "external", Host: "h", Port: 1883, Enabled: true})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error adding MQTT broker")
	assert.NoError(t, mock.ExpectationsWereMet())
}
