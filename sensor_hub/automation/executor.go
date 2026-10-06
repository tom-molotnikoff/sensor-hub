package automation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	appProps "example/sensorHub/application_properties"
	gen "example/sensorHub/gen"
	"example/sensorHub/notifications"
	"example/sensorHub/telemetry"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	failureRecipients = "manage_automations"

	// outcomeGrace is how long past the command timeout a step keeps waiting
	// for the tracker's verdict before giving up on the command itself.
	outcomeGrace = 5 * time.Second

	defaultCommandTimeout = 10 * time.Second

	// Command statuses, as the command tracker reports them.
	commandAcknowledged = "acknowledged"
	commandTimedOut     = "timed_out"
)

type executor struct {
	store     Store
	sensors   SensorLookup
	commands  CommandSender
	notifier  Notifier
	logger    *slog.Logger
	now       func() time.Time
	tracer    trace.Tracer
	runsEnded metric.Int64Counter
}

// execute carries out a run's steps in order, stopping at the first that
// fails, and records the run's outcome. It returns early, leaving the run
// unfinished, when ctx is cancelled or the run is deleted under it.
func (e *executor) execute(ctx context.Context, automation Automation, run Run) {
	logger := e.logger.With("automation_id", automation.ID, "run_id", run.ID)
	ctx, span := e.tracer.Start(ctx, "automation.run", trace.WithAttributes(
		attribute.Int("automation_id", automation.ID),
		attribute.Int("run_id", run.ID),
	))
	defer span.End()
	logger.Info("automation run started", "automation", automation.Name, "trigger_kind", run.TriggerKind)

	for i, step := range run.Steps {
		position := i + 1
		reason, finished := e.executeStep(ctx, logger, run, position, step)
		if !finished {
			return
		}
		if reason != "" {
			message := fmt.Sprintf("step %d (%s) failed: %s", position, describeStep(ctx, e.sensors, step), reason)
			span.SetStatus(codes.Error, message)
			e.finish(ctx, logger, run, RunFailed, &message)
			e.notifyFailure(ctx, logger, automation, message)
			return
		}
	}
	e.finish(ctx, logger, run, RunSucceeded, nil)
}

// executeStep runs one step and records its outcome. reason explains a
// failure and is empty on success; finished is false when the step could not
// be recorded and the run has to stop where it is.
func (e *executor) executeStep(ctx context.Context, logger *slog.Logger, run Run, position int, step Step) (reason string, finished bool) {
	ctx, span := e.tracer.Start(ctx, "automation.step", trace.WithAttributes(
		attribute.Int("position", position),
		attribute.String("kind", string(step.Kind)),
	))
	defer span.End()

	stepID, err := e.store.StartRunStep(ctx, run.ID, position, step.Kind, e.now())
	if errors.Is(err, ErrRunGone) {
		logger.Info("automation run deleted with its automation; stopping", "position", position)
		return "", false
	}
	if err != nil {
		logger.Error("could not record automation step start; abandoning run", "position", position, "error", err)
		return "", false
	}

	commandID, reason, interrupted := e.set(ctx, logger, run, step)
	if interrupted {
		logger.Warn("automation run interrupted", "position", position)
		return "", false
	}

	outcome := StepSucceeded
	if reason != "" {
		outcome = StepFailed
		span.SetStatus(codes.Error, reason)
	}
	if err := e.store.FinishRunStep(ctx, stepID, outcome, commandID, e.now()); err != nil {
		logger.Error("could not record automation step outcome", "position", position, "error", err)
	}
	logger.Info("automation step finished", "position", position, "outcome", outcome, "reason", reason)
	return reason, true
}

// set sends a set step's command and waits for the device to acknowledge it.
// The sensor and value are checked again here because either may have
// changed since the automation was saved.
func (e *executor) set(ctx context.Context, logger *slog.Logger, run Run, step Step) (commandID *int, reason string, interrupted bool) {
	sensor, err := e.sensors.ServiceGetSensorById(ctx, step.SensorID)
	if err != nil || sensor == nil {
		return nil, fmt.Sprintf("sensor %d could not be loaded", step.SensorID), false
	}
	if !sensor.Enabled {
		return nil, fmt.Sprintf("%s is disabled", sensor.Name), false
	}
	if sensor.Status != gen.SensorStatusActive {
		return nil, fmt.Sprintf("%s is %s, not active", sensor.Name, sensor.Status), false
	}
	capability, err := writableCapability(*sensor, step.Property)
	if err != nil {
		return nil, err.Error(), false
	}
	if err := checkValue(capability, step.Value); err != nil {
		return nil, err.Error(), false
	}

	id, outcome, err := e.commands.SendAsSystem(ctx, step.SensorID, step.Property, step.Value, run.ID)
	if id != 0 {
		commandID = &id
	}
	if err != nil {
		return commandID, err.Error(), false
	}
	logger.Info("automation command sent", "command_id", id, "sensor", sensor.Name, "property", step.Property, "value", step.Value)

	timeout := commandTimeout()
	select {
	case status := <-outcome:
		switch status {
		case commandAcknowledged:
			return commandID, "", false
		case commandTimedOut:
			return commandID, fmt.Sprintf("%s did not acknowledge the command within %s", sensor.Name, timeout), false
		default:
			return commandID, fmt.Sprintf("the command to %s %s", sensor.Name, status), false
		}
	case <-time.After(timeout + outcomeGrace):
		return commandID, fmt.Sprintf("no outcome was recorded for the command to %s", sensor.Name), false
	case <-ctx.Done():
		return commandID, "", true
	}
}

func (e *executor) finish(ctx context.Context, logger *slog.Logger, run Run, status RunStatus, message *string) {
	if err := e.store.FinishRun(ctx, run.ID, status, message, e.now()); err != nil {
		logger.Error("could not record automation run outcome", "status", status, "error", err)
	}
	e.runsEnded.Add(ctx, 1, metric.WithAttributes(attribute.String("status", string(status))))
	logger.Info("automation run finished", "status", status)
}

func (e *executor) notifyFailure(ctx context.Context, logger *slog.Logger, automation Automation, message string) {
	_, err := e.notifier.CreateNotification(ctx, notifications.Notification{
		Category: notifications.CategoryAutomationFailure,
		Severity: notifications.SeverityError,
		Title:    fmt.Sprintf("Automation failed: %s", automation.Name),
		Message:  fmt.Sprintf("%s: %s", automation.Name, message),
		Metadata: map[string]interface{}{
			"automation_id":   automation.ID,
			"automation_name": automation.Name,
		},
	}, failureRecipients)
	if err != nil {
		logger.Error("could not send automation failure notification", "error", err)
	}
}

// describeStep names a step for people, such as "set office-plug state to ON".
func describeStep(ctx context.Context, sensors SensorLookup, step Step) string {
	name := fmt.Sprintf("sensor %d", step.SensorID)
	if sensor, err := sensors.ServiceGetSensorById(ctx, step.SensorID); err == nil && sensor != nil {
		name = sensor.Name
	}
	return fmt.Sprintf("set %s %s to %s", name, step.Property, step.Value)
}

func commandTimeout() time.Duration {
	if cfg := appProps.AppConfig(); cfg != nil && cfg.ActuatorCommandTimeoutSeconds > 0 {
		return time.Duration(cfg.ActuatorCommandTimeoutSeconds) * time.Second
	}
	return defaultCommandTimeout
}

func newRunsEndedCounter() metric.Int64Counter {
	counter, _ := telemetry.Meter("automation").Int64Counter("automation.runs",
		metric.WithDescription("Automation runs that have ended, by final status"),
		metric.WithUnit("{run}"))
	return counter
}
