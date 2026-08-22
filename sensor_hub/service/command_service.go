package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"example/sensorHub/actuation"
	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	"example/sensorHub/drivers"
	gen "example/sensorHub/gen"
)

const (
	defaultCommandTimeoutSeconds = 10
	defaultCommandHistoryLimit   = 50
	commandPublishQOS            = 1
)

type CommandSensorRepository interface {
	GetSensorById(ctx context.Context, id int) (*gen.Sensor, error)
}

type CommandSubscriptionRepository interface {
	ListEnabledByDriverType(ctx context.Context, driverType string) ([]gen.MQTTSubscription, error)
}

type CommandHistoryRepository interface {
	HasPendingCommand(ctx context.Context, sensorID int, property string) (bool, error)
	AddSentCommand(ctx context.Context, sensorID int, userID *int, property string, value string, mqttTopic string, mqttPayload string, timeoutSeconds int, sentAt time.Time) (int, error)
	ListBySensorID(ctx context.Context, sensorID int, limit int) ([]gen.CommandHistoryEntry, error)
}

type CommandPublisher interface {
	Publish(brokerID int, topic string, payload []byte, qos byte) error
}

type SentCommandResult struct {
	ID       int
	Status   string
	Property string
	Value    string
}

type CommandError struct {
	StatusCode int
	Message    string
}

func (e *CommandError) Error() string {
	return e.Message
}

func newCommandError(statusCode int, message string) *CommandError {
	return &CommandError{StatusCode: statusCode, Message: message}
}

type CommandService struct {
	sensorRepo  CommandSensorRepository
	subRepo     CommandSubscriptionRepository
	historyRepo CommandHistoryRepository
	publisher   CommandPublisher
	lifecycle   actuation.LifecycleManager
	logger      *slog.Logger
}

func NewCommandService(sensorRepo CommandSensorRepository, subRepo CommandSubscriptionRepository, historyRepo CommandHistoryRepository, publisher CommandPublisher, lifecycle actuation.LifecycleManager, logger *slog.Logger) *CommandService {
	if logger == nil {
		logger = slog.Default()
	}
	return &CommandService{
		sensorRepo:  sensorRepo,
		subRepo:     subRepo,
		historyRepo: historyRepo,
		publisher:   publisher,
		lifecycle:   lifecycle,
		logger:      logger.With("component", "command_service"),
	}
}

func (s *CommandService) GetHistory(ctx context.Context, sensorID int) ([]gen.CommandHistoryEntry, error) {
	sensor, err := s.sensorRepo.GetSensorById(ctx, sensorID)
	if err != nil || sensor == nil {
		if err != nil {
			return nil, newCommandError(http.StatusNotFound, fmt.Sprintf("sensor %d not found", sensorID))
		}
		return nil, newCommandError(http.StatusNotFound, fmt.Sprintf("sensor %d not found", sensorID))
	}

	history, err := s.historyRepo.ListBySensorID(ctx, sensor.Id, defaultCommandHistoryLimit)
	if err != nil {
		return nil, fmt.Errorf("list command history: %w", err)
	}
	if history == nil {
		return []gen.CommandHistoryEntry{}, nil
	}
	return history, nil
}

func (s *CommandService) Send(ctx context.Context, sensorID int, actor *gen.User, property string, value string) (SentCommandResult, error) {
	sensor, err := s.sensorRepo.GetSensorById(ctx, sensorID)
	if err != nil || sensor == nil {
		if err != nil {
			return SentCommandResult{}, newCommandError(http.StatusNotFound, fmt.Sprintf("sensor %d not found", sensorID))
		}
		return SentCommandResult{}, newCommandError(http.StatusNotFound, fmt.Sprintf("sensor %d not found", sensorID))
	}

	if actor == nil || !hasPermission(actor.Permissions, "control_sensors") {
		return SentCommandResult{}, newCommandError(http.StatusForbidden, "missing control_sensors permission")
	}

	commandDriver, ok := drivers.GetCommandDriver(sensor.SensorDriver)
	if !ok {
		return SentCommandResult{}, newCommandError(http.StatusBadRequest, fmt.Sprintf("sensor %d is not controllable", sensorID))
	}

	if !sensor.Enabled || sensor.Status != gen.SensorStatusActive {
		return SentCommandResult{}, newCommandError(http.StatusConflict, fmt.Sprintf("sensor %d is not in a controllable state", sensorID))
	}

	topic, payload, err := commandDriver.BuildCommand(*sensor, property, value)
	if err != nil {
		return SentCommandResult{}, newCommandError(http.StatusBadRequest, err.Error())
	}

	hasPending, err := s.historyRepo.HasPendingCommand(ctx, sensor.Id, property)
	if err != nil {
		return SentCommandResult{}, fmt.Errorf("check pending command: %w", err)
	}
	if hasPending {
		return SentCommandResult{}, newCommandError(http.StatusTooManyRequests, fmt.Sprintf("sensor %d already has a pending command for property %q", sensorID, property))
	}

	subscriptions, err := s.subRepo.ListEnabledByDriverType(ctx, sensor.SensorDriver)
	if err != nil {
		return SentCommandResult{}, fmt.Errorf("lookup MQTT subscriptions: %w", err)
	}
	if len(subscriptions) == 0 {
		return SentCommandResult{}, newCommandError(http.StatusServiceUnavailable, fmt.Sprintf("no enabled MQTT subscription for driver %q", sensor.SensorDriver))
	}

	timeoutSeconds := resolveCommandTimeoutSeconds()
	sentAt := time.Now().UTC()
	userID := actor.Id
	commandID, err := s.historyRepo.AddSentCommand(ctx, sensor.Id, &userID, property, value, topic, string(payload), timeoutSeconds, sentAt)
	if err != nil {
		return SentCommandResult{}, fmt.Errorf("persist sent command: %w", err)
	}

	commandRecord := database.PendingCommandRecord{
		ID:             commandID,
		SensorID:       sensor.Id,
		Property:       property,
		Value:          value,
		Status:         actuation.CommandStatusSent,
		TimeoutSeconds: timeoutSeconds,
		SentAt:         sentAt,
	}
	backgroundCtx := context.Background()
	if s.lifecycle != nil {
		s.lifecycle.Track(backgroundCtx, commandRecord)
	}

	var lastDisconnectedErr error
	for _, subscription := range subscriptions {
		if err := s.publisher.Publish(subscription.BrokerId, topic, payload, commandPublishQOS); err != nil {
			if strings.Contains(err.Error(), "not connected") {
				lastDisconnectedErr = err
				continue
			}
			if s.lifecycle != nil {
				s.lifecycle.MarkFailed(backgroundCtx, commandRecord)
			}
			return SentCommandResult{}, fmt.Errorf("publish command: %w", err)
		}
		lastDisconnectedErr = nil
		break
	}
	if lastDisconnectedErr != nil {
		if s.lifecycle != nil {
			s.lifecycle.MarkFailed(backgroundCtx, commandRecord)
		}
		return SentCommandResult{}, newCommandError(http.StatusServiceUnavailable, lastDisconnectedErr.Error())
	}

	return SentCommandResult{
		ID:       commandID,
		Status:   actuation.CommandStatusSent,
		Property: property,
		Value:    value,
	}, nil
}

func hasPermission(permissions []string, required string) bool {
	for _, permission := range permissions {
		if strings.EqualFold(permission, required) {
			return true
		}
	}
	return false
}

func resolveCommandTimeoutSeconds() int {
	if cfg := appProps.AppConfig(); cfg != nil && cfg.ActuatorCommandTimeoutSeconds > 0 {
		return cfg.ActuatorCommandTimeoutSeconds
	}
	return defaultCommandTimeoutSeconds
}
