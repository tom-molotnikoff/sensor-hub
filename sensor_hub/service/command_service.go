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
	CommandStatus(ctx context.Context, id int) (string, error)
	AddSentCommand(ctx context.Context, command database.NewCommand) (int, error)
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
	sensor, err := s.commandableSensor(ctx, sensorID)
	if err != nil {
		return SentCommandResult{}, err
	}

	if actor == nil || !hasPermission(actor.Permissions, "control_sensors") {
		return SentCommandResult{}, newCommandError(http.StatusForbidden, "missing control_sensors permission")
	}

	userID := actor.Id
	result, _, err := s.send(ctx, sensor, database.NewCommand{UserID: &userID, Property: property, Value: value})
	return result, err
}

// There is no permission check: whoever saved or enabled the automation was
// checked for control_sensors then.
func (s *CommandService) SendAsSystem(ctx context.Context, sensorID int, property string, value string, automationRunID int) (int, <-chan string, error) {
	sensor, err := s.commandableSensor(ctx, sensorID)
	if err != nil {
		return 0, nil, err
	}
	result, outcome, err := s.send(ctx, sensor, database.NewCommand{AutomationRunID: &automationRunID, Property: property, Value: value})
	return result.ID, outcome, err
}

// AwaitOutcome gives a command sent before a restart the outcome channel that
// SendAsSystem would have returned.
func (s *CommandService) AwaitOutcome(ctx context.Context, commandID int) (<-chan string, error) {
	if s.lifecycle != nil {
		if outcome, ok := s.lifecycle.Await(commandID); ok {
			return outcome, nil
		}
	}
	status, err := s.historyRepo.CommandStatus(ctx, commandID)
	if err != nil {
		return nil, fmt.Errorf("read command %d status: %w", commandID, err)
	}
	if status == actuation.CommandStatusSent {
		return nil, fmt.Errorf("command %d is pending but not tracked", commandID)
	}
	outcome := make(chan string, 1)
	outcome <- status
	return outcome, nil
}

func (s *CommandService) commandableSensor(ctx context.Context, sensorID int) (*gen.Sensor, error) {
	sensor, err := s.sensorRepo.GetSensorById(ctx, sensorID)
	if err != nil || sensor == nil {
		return nil, newCommandError(http.StatusNotFound, fmt.Sprintf("sensor %d not found", sensorID))
	}
	return sensor, nil
}

func (s *CommandService) send(ctx context.Context, sensor *gen.Sensor, command database.NewCommand) (SentCommandResult, <-chan string, error) {
	commandDriver, ok := drivers.GetCommandDriver(sensor.SensorDriver)
	if !ok {
		return SentCommandResult{}, nil, newCommandError(http.StatusBadRequest, fmt.Sprintf("sensor %d is not controllable", sensor.Id))
	}

	if !sensor.Enabled || sensor.Status != gen.SensorStatusActive {
		return SentCommandResult{}, nil, newCommandError(http.StatusConflict, fmt.Sprintf("sensor %d is not in a controllable state", sensor.Id))
	}

	topic, payload, err := commandDriver.BuildCommand(*sensor, command.Property, command.Value)
	if err != nil {
		return SentCommandResult{}, nil, newCommandError(http.StatusBadRequest, err.Error())
	}

	hasPending, err := s.historyRepo.HasPendingCommand(ctx, sensor.Id, command.Property)
	if err != nil {
		return SentCommandResult{}, nil, fmt.Errorf("check pending command: %w", err)
	}
	if hasPending {
		return SentCommandResult{}, nil, newCommandError(http.StatusTooManyRequests, fmt.Sprintf("sensor %d already has a pending command for property %q", sensor.Id, command.Property))
	}

	subscriptions, err := s.subRepo.ListEnabledByDriverType(ctx, sensor.SensorDriver)
	if err != nil {
		return SentCommandResult{}, nil, fmt.Errorf("lookup MQTT subscriptions: %w", err)
	}
	if len(subscriptions) == 0 {
		return SentCommandResult{}, nil, newCommandError(http.StatusServiceUnavailable, fmt.Sprintf("no enabled MQTT subscription for driver %q", sensor.SensorDriver))
	}

	command.SensorID = sensor.Id
	command.MQTTTopic = topic
	command.MQTTPayload = string(payload)
	command.TimeoutSeconds = resolveCommandTimeoutSeconds()
	command.SentAt = time.Now().UTC()
	commandID, err := s.historyRepo.AddSentCommand(ctx, command)
	if err != nil {
		return SentCommandResult{}, nil, fmt.Errorf("persist sent command: %w", err)
	}

	commandRecord := database.PendingCommandRecord{
		ID:             commandID,
		SensorID:       sensor.Id,
		Property:       command.Property,
		Value:          command.Value,
		Status:         actuation.CommandStatusSent,
		TimeoutSeconds: command.TimeoutSeconds,
		SentAt:         command.SentAt,
	}
	backgroundCtx := context.Background()
	var outcome <-chan string
	if s.lifecycle != nil {
		outcome = s.lifecycle.Track(backgroundCtx, commandRecord)
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
			return SentCommandResult{ID: commandID}, nil, fmt.Errorf("publish command: %w", err)
		}
		lastDisconnectedErr = nil
		break
	}
	if lastDisconnectedErr != nil {
		if s.lifecycle != nil {
			s.lifecycle.MarkFailed(backgroundCtx, commandRecord)
		}
		return SentCommandResult{ID: commandID}, nil, newCommandError(http.StatusServiceUnavailable, lastDisconnectedErr.Error())
	}

	return SentCommandResult{
		ID:       commandID,
		Status:   actuation.CommandStatusSent,
		Property: command.Property,
		Value:    command.Value,
	}, outcome, nil
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
