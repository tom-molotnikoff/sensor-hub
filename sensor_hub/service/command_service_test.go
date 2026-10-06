package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"example/sensorHub/actuation"
	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockCommandSensorRepository struct{ mock.Mock }

func (m *mockCommandSensorRepository) GetSensorById(ctx context.Context, id int) (*gen.Sensor, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.Sensor), args.Error(1)
}

type mockCommandSubscriptionRepository struct{ mock.Mock }

func (m *mockCommandSubscriptionRepository) ListEnabledByDriverType(ctx context.Context, driverType string) ([]gen.MQTTSubscription, error) {
	args := m.Called(ctx, driverType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]gen.MQTTSubscription), args.Error(1)
}

type mockCommandHistoryRepository struct{ mock.Mock }

func (m *mockCommandHistoryRepository) HasPendingCommand(ctx context.Context, sensorID int, property string) (bool, error) {
	args := m.Called(ctx, sensorID, property)
	return args.Bool(0), args.Error(1)
}

func (m *mockCommandHistoryRepository) CommandStatus(ctx context.Context, id int) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}

func (m *mockCommandHistoryRepository) AddSentCommand(ctx context.Context, command database.NewCommand) (int, error) {
	args := m.Called(ctx, command)
	return args.Int(0), args.Error(1)
}

func officePlugCommand(userID *int, automationRunID *int) interface{} {
	want := database.NewCommand{
		SensorID:        7,
		UserID:          userID,
		AutomationRunID: automationRunID,
		Property:        "state",
		Value:           "ON",
		MQTTTopic:       "zigbee2mqtt/office-plug/set",
		MQTTPayload:     `{"state":"ON"}`,
		TimeoutSeconds:  10,
	}
	return mock.MatchedBy(func(got database.NewCommand) bool {
		got.SentAt = time.Time{}
		return assert.ObjectsAreEqual(want, got)
	})
}

func (m *mockCommandHistoryRepository) MarkAcknowledged(ctx context.Context, id int, acknowledgedValue string, acknowledgedAt time.Time) (bool, error) {
	if !m.hasExpectation("MarkAcknowledged") {
		return false, nil
	}
	args := m.Called(ctx, id, acknowledgedValue, acknowledgedAt)
	return args.Bool(0), args.Error(1)
}

func (m *mockCommandHistoryRepository) MarkTimedOut(ctx context.Context, id int) (bool, error) {
	if !m.hasExpectation("MarkTimedOut") {
		return false, nil
	}
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *mockCommandHistoryRepository) MarkFailed(ctx context.Context, id int) (bool, error) {
	if !m.hasExpectation("MarkFailed") {
		return false, nil
	}
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *mockCommandHistoryRepository) ListPendingCommands(ctx context.Context) ([]database.PendingCommandRecord, error) {
	if !m.hasExpectation("ListPendingCommands") {
		return nil, nil
	}
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]database.PendingCommandRecord), args.Error(1)
}

func (m *mockCommandHistoryRepository) ListBySensorID(ctx context.Context, sensorID int, limit int) ([]gen.CommandHistoryEntry, error) {
	if !m.hasExpectation("ListBySensorID") {
		return nil, nil
	}
	args := m.Called(ctx, sensorID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]gen.CommandHistoryEntry), args.Error(1)
}

func (m *mockCommandHistoryRepository) hasExpectation(method string) bool {
	for _, call := range m.ExpectedCalls {
		if call.Method == method {
			return true
		}
	}
	return false
}

type mockCommandPublisher struct{ mock.Mock }

func (m *mockCommandPublisher) Publish(brokerID int, topic string, payload []byte, qos byte) error {
	return m.Called(brokerID, topic, payload, qos).Error(0)
}

type fakeCommandLifecycle struct {
	tracked []database.PendingCommandRecord
	failed  []database.PendingCommandRecord
}

func (f *fakeCommandLifecycle) Track(_ context.Context, command database.PendingCommandRecord) <-chan string {
	f.tracked = append(f.tracked, command)
	outcome := make(chan string, 1)
	outcome <- actuation.CommandStatusAcknowledged
	return outcome
}

func (f *fakeCommandLifecycle) MarkFailed(_ context.Context, command database.PendingCommandRecord) {
	f.failed = append(f.failed, command)
}

func (f *fakeCommandLifecycle) RecoverPending(context.Context) error {
	return nil
}

func (f *fakeCommandLifecycle) Await(int) (<-chan string, bool) {
	return nil, false
}

func controllableOfficePlug() *gen.Sensor {
	return &gen.Sensor{
		Id:           7,
		Name:         "office-plug",
		SensorDriver: "mqtt-zigbee2mqtt",
		Status:       gen.SensorStatusActive,
		Enabled:      true,
		Metadata: &map[string]interface{}{
			"exposes": []interface{}{
				map[string]interface{}{
					"type":      "binary",
					"property":  "state",
					"access":    float64(7),
					"value_on":  "ON",
					"value_off": "OFF",
				},
			},
		},
	}
}

func newCommandServiceForTest(sensorRepo CommandSensorRepository, subRepo CommandSubscriptionRepository, historyRepo CommandHistoryRepository, publisher CommandPublisher, lifecycle *fakeCommandLifecycle) (*CommandService, *fakeCommandLifecycle) {
	if lifecycle == nil {
		lifecycle = &fakeCommandLifecycle{}
	}
	return NewCommandService(sensorRepo, subRepo, historyRepo, publisher, lifecycle, nil), lifecycle
}

func TestCommandService_Send_PublishesAndPersistsSentCommand(t *testing.T) {
	sensorRepo := &mockCommandSensorRepository{}
	subRepo := &mockCommandSubscriptionRepository{}
	historyRepo := &mockCommandHistoryRepository{}
	publisher := &mockCommandPublisher{}
	svc, lifecycle := newCommandServiceForTest(sensorRepo, subRepo, historyRepo, publisher, nil)

	sensor := controllableOfficePlug()
	userID := 99
	actor := &gen.User{Id: userID, Permissions: []string{"control_sensors"}}
	sub := gen.MQTTSubscription{BrokerId: 12, DriverType: "mqtt-zigbee2mqtt", Enabled: true}

	sensorRepo.On("GetSensorById", mock.Anything, 7).Return(sensor, nil)
	subRepo.On("ListEnabledByDriverType", mock.Anything, "mqtt-zigbee2mqtt").Return([]gen.MQTTSubscription{sub}, nil)
	historyRepo.On("HasPendingCommand", mock.Anything, 7, "state").Return(false, nil)
	publisher.On("Publish", 12, "zigbee2mqtt/office-plug/set", []byte(`{"state":"ON"}`), byte(1)).Return(nil)
	historyRepo.On("AddSentCommand", mock.Anything, officePlugCommand(&userID, nil)).Return(42, nil)

	result, err := svc.Send(context.Background(), 7, actor, "state", "ON")

	require.NoError(t, err)
	assert.Equal(t, SentCommandResult{
		ID:       42,
		Status:   actuation.CommandStatusSent,
		Property: "state",
		Value:    "ON",
	}, result)
	require.Len(t, lifecycle.tracked, 1)
	assert.Equal(t, 42, lifecycle.tracked[0].ID)
}

func TestCommandService_SendAsSystem_RecordsTheRunAndNoUser(t *testing.T) {
	sensorRepo := &mockCommandSensorRepository{}
	subRepo := &mockCommandSubscriptionRepository{}
	historyRepo := &mockCommandHistoryRepository{}
	publisher := &mockCommandPublisher{}
	svc, lifecycle := newCommandServiceForTest(sensorRepo, subRepo, historyRepo, publisher, nil)

	sensorRepo.On("GetSensorById", mock.Anything, 7).Return(controllableOfficePlug(), nil)
	subRepo.On("ListEnabledByDriverType", mock.Anything, "mqtt-zigbee2mqtt").Return([]gen.MQTTSubscription{{BrokerId: 12, DriverType: "mqtt-zigbee2mqtt", Enabled: true}}, nil)
	historyRepo.On("HasPendingCommand", mock.Anything, 7, "state").Return(false, nil)
	publisher.On("Publish", 12, "zigbee2mqtt/office-plug/set", []byte(`{"state":"ON"}`), byte(1)).Return(nil)
	runID := 31
	historyRepo.On("AddSentCommand", mock.Anything, officePlugCommand(nil, &runID)).Return(42, nil)

	commandID, outcome, err := svc.SendAsSystem(context.Background(), 7, "state", "ON", runID)

	require.NoError(t, err)
	assert.Equal(t, 42, commandID)
	require.Len(t, lifecycle.tracked, 1)
	assert.Equal(t, actuation.CommandStatusAcknowledged, <-outcome)
	historyRepo.AssertExpectations(t)
}

type discardCommandStatus struct{}

func (discardCommandStatus) BroadcastCommandStatus(actuation.CommandStatusMessage) {}

func TestCommandService_AwaitOutcome_GivesACommandThatAlreadySettledItsStatus(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handles, err := database.Open(&appProps.ApplicationConfiguration{
		DatabasePath:              filepath.Join(t.TempDir(), "commands.db"),
		DatabaseReaderConnections: 2,
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { handles.Close() })
	result, err := handles.Writer.Exec("INSERT INTO sensors (name, sensor_driver, config) VALUES ('office-plug', 'mqtt-zigbee2mqtt', '{}')")
	require.NoError(t, err)
	sensorID, err := result.LastInsertId()
	require.NoError(t, err)
	historyRepo := database.NewSensorCommandHistoryRepository(handles, logger)
	commandID, err := historyRepo.AddSentCommand(ctx, database.NewCommand{
		SensorID:       int(sensorID),
		Property:       "state",
		Value:          "ON",
		MQTTTopic:      "zigbee2mqtt/office-plug/set",
		MQTTPayload:    `{"state":"ON"}`,
		TimeoutSeconds: 10,
		SentAt:         time.Now().UTC(),
	})
	require.NoError(t, err)
	_, err = historyRepo.MarkTimedOut(ctx, commandID)
	require.NoError(t, err)

	tracker := actuation.NewCommandTracker(historyRepo, discardCommandStatus{}, logger)
	t.Cleanup(tracker.Close)
	require.NoError(t, tracker.RecoverPending(ctx))
	service := NewCommandService(nil, nil, historyRepo, nil, tracker, logger)

	outcome, err := service.AwaitOutcome(ctx, commandID)

	require.NoError(t, err)
	assert.Equal(t, actuation.CommandStatusTimedOut, <-outcome)
}

func TestCommandService_GetHistory_ReturnsLatestEntriesFromRepository(t *testing.T) {
	sensorRepo := &mockCommandSensorRepository{}
	subRepo := &mockCommandSubscriptionRepository{}
	historyRepo := &mockCommandHistoryRepository{}
	publisher := &mockCommandPublisher{}
	svc, _ := newCommandServiceForTest(sensorRepo, subRepo, historyRepo, publisher, nil)

	sensorRepo.On("GetSensorById", mock.Anything, 7).Return(&gen.Sensor{Id: 7, Name: "office-plug"}, nil)
	expected := []gen.CommandHistoryEntry{
		{
			Id:             42,
			Property:       "state",
			Value:          "ON",
			Status:         gen.CommandHistoryEntryStatusAcknowledged,
			SentAt:         time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
			TimeoutSeconds: 10,
			MqttTopic:      "zigbee2mqtt/office-plug/set",
			MqttPayload:    `{"state":"ON"}`,
			User: &gen.CommandHistoryUser{
				Id:       99,
				Username: "admin",
			},
		},
	}
	historyRepo.On("ListBySensorID", mock.Anything, 7, 50).Return(expected, nil)

	history, err := svc.GetHistory(context.Background(), 7)
	require.NoError(t, err)
	assert.Equal(t, expected, history)
	sensorRepo.AssertExpectations(t)
	historyRepo.AssertExpectations(t)
}

func TestCommandService_Send_InsufficientPermission(t *testing.T) {
	sensorRepo := &mockCommandSensorRepository{}
	subRepo := &mockCommandSubscriptionRepository{}
	historyRepo := &mockCommandHistoryRepository{}
	publisher := &mockCommandPublisher{}
	svc, _ := newCommandServiceForTest(sensorRepo, subRepo, historyRepo, publisher, nil)

	sensorRepo.On("GetSensorById", mock.Anything, 7).Return(&gen.Sensor{
		Id:           7,
		Name:         "office-plug",
		SensorDriver: "mqtt-zigbee2mqtt",
		Status:       gen.SensorStatusActive,
		Enabled:      true,
	}, nil)

	_, err := svc.Send(context.Background(), 7, &gen.User{Id: 5, Permissions: []string{"view_sensors"}}, "state", "ON")

	var commandErr *CommandError
	require.ErrorAs(t, err, &commandErr)
	assert.Equal(t, http.StatusForbidden, commandErr.StatusCode)
}

func TestCommandService_Send_NotControllable(t *testing.T) {
	sensorRepo := &mockCommandSensorRepository{}
	subRepo := &mockCommandSubscriptionRepository{}
	historyRepo := &mockCommandHistoryRepository{}
	publisher := &mockCommandPublisher{}
	svc, _ := newCommandServiceForTest(sensorRepo, subRepo, historyRepo, publisher, nil)

	sensorRepo.On("GetSensorById", mock.Anything, 7).Return(&gen.Sensor{
		Id:           7,
		Name:         "living-room",
		SensorDriver: "sensor-hub-http-temperature",
		Status:       gen.SensorStatusActive,
		Enabled:      true,
	}, nil)

	_, err := svc.Send(context.Background(), 7, &gen.User{Id: 5, Permissions: []string{"control_sensors"}}, "state", "ON")

	var commandErr *CommandError
	require.ErrorAs(t, err, &commandErr)
	assert.Equal(t, http.StatusBadRequest, commandErr.StatusCode)
}

func TestCommandService_Send_SensorPending(t *testing.T) {
	sensorRepo := &mockCommandSensorRepository{}
	subRepo := &mockCommandSubscriptionRepository{}
	historyRepo := &mockCommandHistoryRepository{}
	publisher := &mockCommandPublisher{}
	svc, _ := newCommandServiceForTest(sensorRepo, subRepo, historyRepo, publisher, nil)

	sensorRepo.On("GetSensorById", mock.Anything, 7).Return(&gen.Sensor{
		Id:           7,
		Name:         "office-plug",
		SensorDriver: "mqtt-zigbee2mqtt",
		Status:       gen.SensorStatusPending,
		Enabled:      true,
	}, nil)

	_, err := svc.Send(context.Background(), 7, &gen.User{Id: 5, Permissions: []string{"control_sensors"}}, "state", "ON")

	var commandErr *CommandError
	require.ErrorAs(t, err, &commandErr)
	assert.Equal(t, http.StatusConflict, commandErr.StatusCode)
}

func TestCommandService_Send_SensorDisabled(t *testing.T) {
	sensorRepo := &mockCommandSensorRepository{}
	subRepo := &mockCommandSubscriptionRepository{}
	historyRepo := &mockCommandHistoryRepository{}
	publisher := &mockCommandPublisher{}
	svc, _ := newCommandServiceForTest(sensorRepo, subRepo, historyRepo, publisher, nil)

	sensorRepo.On("GetSensorById", mock.Anything, 7).Return(&gen.Sensor{
		Id:           7,
		Name:         "office-plug",
		SensorDriver: "mqtt-zigbee2mqtt",
		Status:       gen.SensorStatusActive,
		Enabled:      false,
	}, nil)

	_, err := svc.Send(context.Background(), 7, &gen.User{Id: 5, Permissions: []string{"control_sensors"}}, "state", "ON")

	var commandErr *CommandError
	require.ErrorAs(t, err, &commandErr)
	assert.Equal(t, http.StatusConflict, commandErr.StatusCode)
}

func TestCommandService_Send_InvalidValue(t *testing.T) {
	sensorRepo := &mockCommandSensorRepository{}
	subRepo := &mockCommandSubscriptionRepository{}
	historyRepo := &mockCommandHistoryRepository{}
	publisher := &mockCommandPublisher{}
	svc, _ := newCommandServiceForTest(sensorRepo, subRepo, historyRepo, publisher, nil)

	sensorRepo.On("GetSensorById", mock.Anything, 7).Return(&gen.Sensor{
		Id:           7,
		Name:         "office-plug",
		SensorDriver: "mqtt-zigbee2mqtt",
		Status:       gen.SensorStatusActive,
		Enabled:      true,
		Metadata: &map[string]interface{}{
			"exposes": []interface{}{
				map[string]interface{}{
					"type":      "binary",
					"property":  "state",
					"access":    float64(7),
					"value_on":  "ON",
					"value_off": "OFF",
				},
			},
		},
	}, nil)

	_, err := svc.Send(context.Background(), 7, &gen.User{Id: 5, Permissions: []string{"control_sensors"}}, "state", "INVALID")

	var commandErr *CommandError
	require.ErrorAs(t, err, &commandErr)
	assert.Equal(t, http.StatusBadRequest, commandErr.StatusCode)
}

func TestCommandService_Send_DuplicatePendingCommand(t *testing.T) {
	sensorRepo := &mockCommandSensorRepository{}
	subRepo := &mockCommandSubscriptionRepository{}
	historyRepo := &mockCommandHistoryRepository{}
	publisher := &mockCommandPublisher{}
	svc, _ := newCommandServiceForTest(sensorRepo, subRepo, historyRepo, publisher, nil)

	sensorRepo.On("GetSensorById", mock.Anything, 7).Return(&gen.Sensor{
		Id:           7,
		Name:         "office-plug",
		SensorDriver: "mqtt-zigbee2mqtt",
		Status:       gen.SensorStatusActive,
		Enabled:      true,
		Metadata: &map[string]interface{}{
			"exposes": []interface{}{
				map[string]interface{}{
					"type":      "binary",
					"property":  "state",
					"access":    float64(7),
					"value_on":  "ON",
					"value_off": "OFF",
				},
			},
		},
	}, nil)
	historyRepo.On("HasPendingCommand", mock.Anything, 7, "state").Return(true, nil)

	_, err := svc.Send(context.Background(), 7, &gen.User{Id: 5, Permissions: []string{"control_sensors"}}, "state", "ON")

	var commandErr *CommandError
	require.ErrorAs(t, err, &commandErr)
	assert.Equal(t, http.StatusTooManyRequests, commandErr.StatusCode)
}

func TestCommandService_Send_BrokerDisconnected(t *testing.T) {
	sensorRepo := &mockCommandSensorRepository{}
	subRepo := &mockCommandSubscriptionRepository{}
	historyRepo := &mockCommandHistoryRepository{}
	publisher := &mockCommandPublisher{}
	svc, _ := newCommandServiceForTest(sensorRepo, subRepo, historyRepo, publisher, nil)

	sensorRepo.On("GetSensorById", mock.Anything, 7).Return(&gen.Sensor{
		Id:           7,
		Name:         "office-plug",
		SensorDriver: "mqtt-zigbee2mqtt",
		Status:       gen.SensorStatusActive,
		Enabled:      true,
		Metadata: &map[string]interface{}{
			"exposes": []interface{}{
				map[string]interface{}{
					"type":      "binary",
					"property":  "state",
					"access":    float64(7),
					"value_on":  "ON",
					"value_off": "OFF",
				},
			},
		},
	}, nil)
	subRepo.On("ListEnabledByDriverType", mock.Anything, "mqtt-zigbee2mqtt").Return([]gen.MQTTSubscription{{
		BrokerId:   12,
		DriverType: "mqtt-zigbee2mqtt",
		Enabled:    true,
	}}, nil)
	historyRepo.On("HasPendingCommand", mock.Anything, 7, "state").Return(false, nil)
	userID := 5
	historyRepo.On("AddSentCommand", mock.Anything, officePlugCommand(&userID, nil)).Return(42, nil)
	publisher.On("Publish", 12, "zigbee2mqtt/office-plug/set", []byte(`{"state":"ON"}`), byte(1)).Return(errors.New("broker 12 is not connected"))
	historyRepo.On("MarkFailed", mock.Anything, 42).Return(true, nil)

	_, err := svc.Send(context.Background(), 7, &gen.User{Id: userID, Permissions: []string{"control_sensors"}}, "state", "ON")

	var commandErr *CommandError
	require.ErrorAs(t, err, &commandErr)
	assert.Equal(t, http.StatusServiceUnavailable, commandErr.StatusCode)
}

func TestCommandService_Send_SkipsDisconnectedSubscriptionAndPublishesToNext(t *testing.T) {
	sensorRepo := &mockCommandSensorRepository{}
	subRepo := &mockCommandSubscriptionRepository{}
	historyRepo := &mockCommandHistoryRepository{}
	publisher := &mockCommandPublisher{}
	svc, _ := newCommandServiceForTest(sensorRepo, subRepo, historyRepo, publisher, nil)

	sensorRepo.On("GetSensorById", mock.Anything, 7).Return(&gen.Sensor{
		Id:           7,
		Name:         "office-plug",
		SensorDriver: "mqtt-zigbee2mqtt",
		Status:       gen.SensorStatusActive,
		Enabled:      true,
		Metadata: &map[string]interface{}{
			"exposes": []interface{}{
				map[string]interface{}{
					"type":      "binary",
					"property":  "state",
					"access":    float64(7),
					"value_on":  "ON",
					"value_off": "OFF",
				},
			},
		},
	}, nil)
	subRepo.On("ListEnabledByDriverType", mock.Anything, "mqtt-zigbee2mqtt").Return([]gen.MQTTSubscription{
		{BrokerId: 12, DriverType: "mqtt-zigbee2mqtt", Enabled: true},
		{BrokerId: 18, DriverType: "mqtt-zigbee2mqtt", Enabled: true},
	}, nil)
	historyRepo.On("HasPendingCommand", mock.Anything, 7, "state").Return(false, nil)
	historyRepo.On("MarkFailed", mock.Anything, 42).Return(false, nil).Maybe()
	publisher.On("Publish", 12, "zigbee2mqtt/office-plug/set", []byte(`{"state":"ON"}`), byte(1)).Return(errors.New("broker 12 is not connected"))
	publisher.On("Publish", 18, "zigbee2mqtt/office-plug/set", []byte(`{"state":"ON"}`), byte(1)).Return(nil)
	userID := 5
	historyRepo.On("AddSentCommand", mock.Anything, officePlugCommand(&userID, nil)).Return(42, nil)

	result, err := svc.Send(context.Background(), 7, &gen.User{Id: userID, Permissions: []string{"control_sensors"}}, "state", "ON")

	require.NoError(t, err)
	assert.Equal(t, 42, result.ID)
	assert.Equal(t, "sent", result.Status)
}

func TestCommandService_Send_PublishFailureMarksCommandFailed(t *testing.T) {
	sensorRepo := &mockCommandSensorRepository{}
	subRepo := &mockCommandSubscriptionRepository{}
	historyRepo := &mockCommandHistoryRepository{}
	publisher := &mockCommandPublisher{}
	svc, lifecycle := newCommandServiceForTest(sensorRepo, subRepo, historyRepo, publisher, nil)

	sensorRepo.On("GetSensorById", mock.Anything, 7).Return(&gen.Sensor{
		Id:           7,
		Name:         "office-plug",
		SensorDriver: "mqtt-zigbee2mqtt",
		Status:       gen.SensorStatusActive,
		Enabled:      true,
		Metadata: &map[string]interface{}{
			"exposes": []interface{}{
				map[string]interface{}{
					"type":      "binary",
					"property":  "state",
					"access":    float64(7),
					"value_on":  "ON",
					"value_off": "OFF",
				},
			},
		},
	}, nil)
	subRepo.On("ListEnabledByDriverType", mock.Anything, "mqtt-zigbee2mqtt").Return([]gen.MQTTSubscription{{
		BrokerId:   12,
		DriverType: "mqtt-zigbee2mqtt",
		Enabled:    true,
	}}, nil)
	historyRepo.On("HasPendingCommand", mock.Anything, 7, "state").Return(false, nil)
	userID := 5
	historyRepo.On("AddSentCommand", mock.Anything, officePlugCommand(&userID, nil)).Return(42, nil)
	publisher.On("Publish", 12, "zigbee2mqtt/office-plug/set", []byte(`{"state":"ON"}`), byte(1)).Return(errors.New("publish exploded"))
	historyRepo.On("MarkFailed", mock.Anything, 42).Return(true, nil)

	_, err := svc.Send(context.Background(), 7, &gen.User{Id: userID, Permissions: []string{"control_sensors"}}, "state", "ON")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "publish command")
	require.Len(t, lifecycle.failed, 1)
	assert.Equal(t, 42, lifecycle.failed[0].ID)
}
