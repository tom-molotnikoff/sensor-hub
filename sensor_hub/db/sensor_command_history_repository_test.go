package database

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	gen "example/sensorHub/gen"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestSensorCommandHistoryRepository_AddSentCommand_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSensorCommandHistoryRepository(handles(db), slog.Default())
	userID := 99
	sentAt := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)

	mock.ExpectExec("INSERT INTO sensor_command_history").
		WithArgs(7, &userID, "state", "ON", "zigbee2mqtt/office-plug/set", `{"state":"ON"}`, 10, sentAt).
		WillReturnResult(sqlmock.NewResult(42, 1))

	id, err := repo.AddSentCommand(context.Background(), 7, &userID, "state", "ON", "zigbee2mqtt/office-plug/set", `{"state":"ON"}`, 10, sentAt)
	assert.NoError(t, err)
	assert.Equal(t, 42, id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSensorCommandHistoryRepository_HasPendingCommand_ReturnsTrue(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSensorCommandHistoryRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT COUNT\\(1\\) FROM sensor_command_history WHERE sensor_id = \\? AND property = \\? AND status = 'sent'").
		WithArgs(7, "state").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	hasPending, err := repo.HasPendingCommand(context.Background(), 7, "state")
	assert.NoError(t, err)
	assert.True(t, hasPending)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSensorCommandHistoryRepository_AddSentCommand_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSensorCommandHistoryRepository(handles(db), slog.Default())

	mock.ExpectExec("INSERT INTO sensor_command_history").
		WithArgs(7, nil, "state", "ON", "zigbee2mqtt/office-plug/set", `{"state":"ON"}`, 10, sqlmock.AnyArg()).
		WillReturnError(errors.New("write failed"))

	_, err := repo.AddSentCommand(context.Background(), 7, nil, "state", "ON", "zigbee2mqtt/office-plug/set", `{"state":"ON"}`, 10, time.Now().UTC())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error inserting sensor command history")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSensorCommandHistoryRepository_MarkAcknowledged_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSensorCommandHistoryRepository(handles(db), slog.Default())
	acknowledgedAt := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)

	mock.ExpectExec("UPDATE sensor_command_history").
		WithArgs(acknowledgedAt, "OFF", 42).
		WillReturnResult(sqlmock.NewResult(0, 1))

	updated, err := repo.MarkAcknowledged(context.Background(), 42, "OFF", acknowledgedAt)
	assert.NoError(t, err)
	assert.True(t, updated)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSensorCommandHistoryRepository_ListPendingCommands_ReturnsRows(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSensorCommandHistoryRepository(handles(db), slog.Default())
	sentAt := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT id, sensor_id, property, value, status, timeout_seconds, sent_at, acknowledged_at, acknowledged_value").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "sensor_id", "property", "value", "status", "timeout_seconds", "sent_at", "acknowledged_at", "acknowledged_value",
		}).AddRow(42, 7, "state", "ON", "sent", 10, sentAt, nil, nil))

	commands, err := repo.ListPendingCommands(context.Background())
	assert.NoError(t, err)
	assert.Len(t, commands, 1)
	assert.Equal(t, 42, commands[0].ID)
	assert.Equal(t, 7, commands[0].SensorID)
	assert.Equal(t, "state", commands[0].Property)
	assert.Equal(t, "ON", commands[0].Value)
	assert.Equal(t, "sent", commands[0].Status)
	assert.Equal(t, 10, commands[0].TimeoutSeconds)
	assert.Equal(t, sentAt, commands[0].SentAt)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSensorCommandHistoryRepository_ListBySensorID_ReturnsNewestEntriesWithUserMetadata(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSensorCommandHistoryRepository(handles(db), slog.Default())

	sentAt := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	acknowledgedAt := sentAt.Add(2 * time.Second)
	ackValue := "true"

	mock.ExpectQuery("SELECT h.id, h.property, h.value, h.status, h.sent_at, h.acknowledged_at, h.acknowledged_value, h.timeout_seconds, h.mqtt_topic, h.mqtt_payload, u.id, u.username").
		WithArgs(7, 50).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "property", "value", "status", "sent_at", "acknowledged_at", "acknowledged_value", "timeout_seconds", "mqtt_topic", "mqtt_payload", "user_id", "username",
		}).
			AddRow(42, "state", "ON", "acknowledged", sentAt, acknowledgedAt, ackValue, 10, "zigbee2mqtt/office-plug/set", `{"state":"ON"}`, 99, "admin").
			AddRow(41, "state", "OFF", "timed_out", sentAt.Add(-time.Minute), nil, nil, 10, "zigbee2mqtt/office-plug/set", `{"state":"OFF"}`, nil, nil))

	history, err := repo.ListBySensorID(context.Background(), 7, 50)
	assert.NoError(t, err)
	assert.Equal(t, []gen.CommandHistoryEntry{
		{
			Id:                42,
			Property:          "state",
			Value:             "ON",
			Status:            gen.CommandHistoryEntryStatusAcknowledged,
			SentAt:            sentAt,
			AcknowledgedAt:    &acknowledgedAt,
			AcknowledgedValue: &ackValue,
			TimeoutSeconds:    10,
			MqttTopic:         "zigbee2mqtt/office-plug/set",
			MqttPayload:       `{"state":"ON"}`,
			User: &gen.CommandHistoryUser{
				Id:       99,
				Username: "admin",
			},
		},
		{
			Id:             41,
			Property:       "state",
			Value:          "OFF",
			Status:         gen.CommandHistoryEntryStatusTimedOut,
			SentAt:         sentAt.Add(-time.Minute),
			TimeoutSeconds: 10,
			MqttTopic:      "zigbee2mqtt/office-plug/set",
			MqttPayload:    `{"state":"OFF"}`,
		},
	}, history)
	assert.NoError(t, mock.ExpectationsWereMet())
}
