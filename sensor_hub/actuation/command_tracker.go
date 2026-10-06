package actuation

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
)

const (
	CommandStatusSent         = "sent"
	CommandStatusAcknowledged = "acknowledged"
	CommandStatusTimedOut     = "timed_out"
	CommandStatusFailed       = "failed"
)

type CommandStatusMessage struct {
	Type              string     `json:"type"`
	ID                int        `json:"id"`
	SensorID          int        `json:"sensor_id"`
	Property          string     `json:"property"`
	Value             string     `json:"value"`
	Status            string     `json:"status"`
	AcknowledgedValue *string    `json:"acknowledged_value,omitempty"`
	AcknowledgedAt    *time.Time `json:"acknowledged_at,omitempty"`
}

type CommandRepository interface {
	MarkAcknowledged(ctx context.Context, id int, acknowledgedValue string, acknowledgedAt time.Time) (bool, error)
	MarkTimedOut(ctx context.Context, id int) (bool, error)
	MarkFailed(ctx context.Context, id int) (bool, error)
	ListPendingCommands(ctx context.Context) ([]database.PendingCommandRecord, error)
}

type CommandStatusBroadcaster interface {
	BroadcastCommandStatus(message CommandStatusMessage)
}

type LifecycleManager interface {
	// The returned channel receives the command's final status once.
	Track(ctx context.Context, command database.PendingCommandRecord) <-chan string
	MarkFailed(ctx context.Context, command database.PendingCommandRecord)
	RecoverPending(ctx context.Context) error
}

type CommandTracker struct {
	repo        CommandRepository
	broadcaster CommandStatusBroadcaster
	logger      *slog.Logger
	now         func() time.Time
	schedule    func(delay time.Duration, fn func()) func()

	mu       sync.Mutex
	commands map[int]database.PendingCommandRecord
	cancels  map[int]func()
	outcomes map[int]chan string
}

func NewCommandTracker(repo CommandRepository, broadcaster CommandStatusBroadcaster, logger *slog.Logger) *CommandTracker {
	if logger == nil {
		logger = slog.Default()
	}
	return &CommandTracker{
		repo:        repo,
		broadcaster: broadcaster,
		logger:      logger.With("component", "command_tracker"),
		now: func() time.Time {
			return time.Now().UTC()
		},
		schedule: defaultSchedule,
		commands: make(map[int]database.PendingCommandRecord),
		cancels:  make(map[int]func()),
		outcomes: make(map[int]chan string),
	}
}

func (t *CommandTracker) Track(ctx context.Context, command database.PendingCommandRecord) <-chan string {
	outcome := make(chan string, 1)
	delay := t.remaining(command)
	if delay <= 0 {
		t.mu.Lock()
		if existingCancel, ok := t.cancels[command.ID]; ok {
			existingCancel()
			delete(t.cancels, command.ID)
		}
		t.commands[command.ID] = command
		t.outcomes[command.ID] = outcome
		t.mu.Unlock()
		t.logger.Debug("command already expired during tracking", "command_id", command.ID)
		t.handleTimeout(ctx, command.ID)
		return outcome
	}

	started := make(chan struct{})
	cancel := t.schedule(delay, func() {
		<-started
		t.handleTimeout(ctx, command.ID)
	})

	t.mu.Lock()
	if existingCancel, ok := t.cancels[command.ID]; ok {
		existingCancel()
		delete(t.cancels, command.ID)
	}
	t.commands[command.ID] = command
	t.cancels[command.ID] = cancel
	t.outcomes[command.ID] = outcome
	t.mu.Unlock()
	close(started)
	t.logger.Debug("tracking command", "command_id", command.ID, "sensor_id", command.SensorID, "property", command.Property, "timeout_seconds", command.TimeoutSeconds, "delay_ms", delay.Milliseconds())
	return outcome
}

func (t *CommandTracker) Consume(ctx context.Context, sensor gen.Sensor, readings []gen.Reading) {
	for _, reading := range readings {
		if reading.MeasurementType == "" {
			continue
		}

		value := readingValue(reading)
		if value == "" {
			continue
		}

		command, ok := t.matchingCommand(sensor.Id, reading.MeasurementType)
		if !ok {
			continue
		}

		acknowledgedAt := t.now()
		updated, err := t.repo.MarkAcknowledged(ctx, command.ID, value, acknowledgedAt)
		if err != nil {
			t.logger.Error("failed to mark command acknowledged", "command_id", command.ID, "error", err)
			continue
		}
		if !updated {
			continue
		}

		command.Status = CommandStatusAcknowledged
		command.AcknowledgedAt = &acknowledgedAt
		command.AcknowledgedValue = &value
		t.settle(command)
		t.logger.Info("command acknowledged", "command_id", command.ID, "sensor_id", command.SensorID, "property", command.Property, "acknowledged_value", value)
		t.broadcast(command)
	}
}

func (t *CommandTracker) MarkFailed(ctx context.Context, command database.PendingCommandRecord) {
	updated, err := t.repo.MarkFailed(ctx, command.ID)
	if err != nil {
		t.logger.Error("failed to mark command failed", "command_id", command.ID, "error", err)
		return
	}
	if !updated {
		return
	}

	command.Status = CommandStatusFailed
	t.settle(command)
	t.logger.Info("command failed", "command_id", command.ID, "sensor_id", command.SensorID, "property", command.Property)
	t.broadcast(command)
}

func (t *CommandTracker) RecoverPending(ctx context.Context) error {
	commands, err := t.repo.ListPendingCommands(ctx)
	if err != nil {
		return err
	}

	for _, command := range commands {
		t.Track(ctx, command)
	}

	return nil
}

func (t *CommandTracker) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, cancel := range t.cancels {
		cancel()
	}
	t.commands = make(map[int]database.PendingCommandRecord)
	t.cancels = make(map[int]func())
	t.outcomes = make(map[int]chan string)
}

func (t *CommandTracker) matchingCommand(sensorID int, property string) (database.PendingCommandRecord, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var matched database.PendingCommandRecord
	found := false
	for _, command := range t.commands {
		if command.SensorID != sensorID || command.Property != property {
			continue
		}
		if !found || command.SentAt.Before(matched.SentAt) || (command.SentAt.Equal(matched.SentAt) && command.ID < matched.ID) {
			matched = command
			found = true
		}
	}

	return matched, found
}

func (t *CommandTracker) settle(command database.PendingCommandRecord) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if cancel, ok := t.cancels[command.ID]; ok {
		cancel()
		delete(t.cancels, command.ID)
	}
	delete(t.commands, command.ID)
	if outcome, ok := t.outcomes[command.ID]; ok {
		outcome <- command.Status
		delete(t.outcomes, command.ID)
	}
}

func (t *CommandTracker) broadcast(command database.PendingCommandRecord) {
	if t.broadcaster == nil {
		return
	}

	t.broadcaster.BroadcastCommandStatus(CommandStatusMessage{
		Type:              "command_status",
		ID:                command.ID,
		SensorID:          command.SensorID,
		Property:          command.Property,
		Value:             command.Value,
		Status:            command.Status,
		AcknowledgedValue: command.AcknowledgedValue,
		AcknowledgedAt:    command.AcknowledgedAt,
	})
}

func readingValue(reading gen.Reading) string {
	if reading.TextState != nil {
		return *reading.TextState
	}
	if reading.NumericValue != nil {
		return strconv.FormatFloat(*reading.NumericValue, 'f', -1, 64)
	}
	return ""
}

func (t *CommandTracker) remaining(command database.PendingCommandRecord) time.Duration {
	return command.SentAt.Add(time.Duration(command.TimeoutSeconds) * time.Second).Sub(t.now())
}

func (t *CommandTracker) handleTimeout(ctx context.Context, id int) {
	command, ok := t.get(id)
	if !ok {
		return
	}

	updated, err := t.repo.MarkTimedOut(ctx, id)
	if err != nil {
		t.logger.Error("failed to mark command timed out", "command_id", id, "error", err)
		return
	}
	if !updated {
		return
	}

	command.Status = CommandStatusTimedOut
	t.settle(command)
	t.logger.Info("command timed out", "command_id", id, "sensor_id", command.SensorID, "property", command.Property)
	t.broadcast(command)
}

func (t *CommandTracker) get(id int) (database.PendingCommandRecord, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	command, ok := t.commands[id]
	return command, ok
}

func defaultSchedule(delay time.Duration, fn func()) func() {
	timer := time.AfterFunc(delay, fn)
	return func() {
		_ = timer.Stop()
	}
}
