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

	// outcomeGrace covers the tracker failing to record a verdict, so a step
	// cannot wait forever.
	outcomeGrace = 5 * time.Second

	defaultCommandTimeout = 10 * time.Second

	// The command tracker's statuses. Importing actuation would be a cycle through db.
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

// execute carries a new run from its first step, and a resumed one from the
// step it is on. It returns early, leaving the run unfinished, when ctx is
// cancelled or the run is deleted under it.
func (e *executor) execute(ctx context.Context, automation Automation, run Run) (resumeAt time.Time, waiting bool) {
	logger := e.logger.With("automation_id", automation.ID, "run_id", run.ID)
	ctx, span := e.tracer.Start(ctx, "automation.run", trace.WithAttributes(
		attribute.Int("automation_id", automation.ID),
		attribute.Int("run_id", run.ID),
	))
	defer span.End()

	from := 1
	if last, ok := currentStep(run); ok {
		logger.Info("automation run resumed", "automation", automation.Name, "position", last.Position)
		reason, finished := e.settleInterrupted(ctx, logger, run, last)
		if !finished {
			return time.Time{}, false
		}
		if reason != "" {
			e.fail(ctx, logger, span, automation, run, last.Position, reason)
			return time.Time{}, false
		}
		from = last.Position + 1
	} else {
		logger.Info("automation run started", "automation", automation.Name, "trigger_kind", run.TriggerKind)
	}

	for position := from; position <= len(run.Steps); position++ {
		step := run.Steps[position-1]
		if step.Kind == StepWait {
			return e.wait(ctx, logger, run, position, step)
		}
		reason, finished := e.executeStep(ctx, logger, run, position, step)
		if !finished {
			return time.Time{}, false
		}
		if reason != "" {
			e.fail(ctx, logger, span, automation, run, position, reason)
			return time.Time{}, false
		}
	}
	e.finish(ctx, logger, run, RunSucceeded, nil)
	return time.Time{}, false
}

func currentStep(run Run) (RunStep, bool) {
	for _, step := range run.StepOutcomes {
		if step.Position == run.CurrentStep {
			return step, true
		}
	}
	return RunStep{}, false
}

func (e *executor) fail(ctx context.Context, logger *slog.Logger, span trace.Span, automation Automation, run Run, position int, reason string) {
	message := fmt.Sprintf("step %d (%s) failed: %s", position, describeStep(ctx, e.sensors, run.Steps[position-1]), reason)
	span.SetStatus(codes.Error, message)
	e.finish(ctx, logger, run, RunFailed, &message)
	e.notifyFailure(ctx, logger, automation, message)
}

// reason is empty on success. finished is false when the run has to stop
// where it is.
func (e *executor) executeStep(ctx context.Context, logger *slog.Logger, run Run, position int, step Step) (reason string, finished bool) {
	ctx, span := e.startStepSpan(ctx, position, step)
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
	return e.finishStep(ctx, logger, span, stepID, position, commandID, reason, interrupted)
}

// settleInterrupted gives the step a restart interrupted its outcome. Only a
// set step can be interrupted: a wait step is never left running in a running
// run, because waiting and resuming each happen in one transaction.
func (e *executor) settleInterrupted(ctx context.Context, logger *slog.Logger, run Run, last RunStep) (string, bool) {
	switch last.Outcome {
	case StepSucceeded:
		return "", true
	case StepFailed:
		return "the hub stopped before the failure was recorded", true
	}

	step := run.Steps[last.Position-1]
	ctx, span := e.startStepSpan(ctx, last.Position, step)
	defer span.End()

	commandID, recorded, err := e.store.LatestRunCommand(ctx, run.ID, step, last.StartedAt)
	if err != nil {
		logger.Error("could not look up the command of an interrupted automation step; abandoning run", "position", last.Position, "error", err)
		return "", false
	}
	if !recorded {
		logger.Info("no command was recorded for the interrupted automation step; sending it", "position", last.Position)
		sent, reason, interrupted := e.set(ctx, logger, run, step)
		return e.finishStep(ctx, logger, span, last.ID, last.Position, sent, reason, interrupted)
	}

	logger.Info("taking the outcome of the command the interrupted automation step sent", "position", last.Position, "command_id", commandID)
	name := sensorName(ctx, e.sensors, step.SensorID)
	outcome, err := e.commands.AwaitOutcome(ctx, commandID)
	if err != nil {
		logger.Error("could not follow the command of an interrupted automation step", "command_id", commandID, "error", err)
		return e.finishStep(ctx, logger, span, last.ID, last.Position, &commandID, fmt.Sprintf("no outcome was recorded for the command to %s", name), false)
	}
	reason, interrupted := e.await(ctx, name, outcome)
	return e.finishStep(ctx, logger, span, last.ID, last.Position, &commandID, reason, interrupted)
}

func (e *executor) finishStep(ctx context.Context, logger *slog.Logger, span trace.Span, stepID int, position int, commandID *int, reason string, interrupted bool) (string, bool) {
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

func (e *executor) wait(ctx context.Context, logger *slog.Logger, run Run, position int, step Step) (time.Time, bool) {
	ctx, span := e.startStepSpan(ctx, position, step)
	defer span.End()

	at := e.now()
	resumeAt := at.Add(step.Wait())
	err := e.store.WaitRun(ctx, run.ID, position, at, resumeAt)
	if errors.Is(err, ErrRunGone) {
		logger.Info("automation run deleted with its automation; stopping", "position", position)
		return time.Time{}, false
	}
	if err != nil {
		logger.Error("could not record automation wait; abandoning run", "position", position, "error", err)
		return time.Time{}, false
	}
	logger.Info("automation run waiting", "position", position, "resume_at", resumeAt)
	return resumeAt, true
}

func (e *executor) startStepSpan(ctx context.Context, position int, step Step) (context.Context, trace.Span) {
	return e.tracer.Start(ctx, "automation.step", trace.WithAttributes(
		attribute.Int("position", position),
		attribute.String("kind", string(step.Kind)),
	))
}

// The sensor and value are checked again because either may have changed
// since the automation was saved.
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

	reason, interrupted = e.await(ctx, sensor.Name, outcome)
	return commandID, reason, interrupted
}

func (e *executor) await(ctx context.Context, name string, outcome <-chan string) (reason string, interrupted bool) {
	select {
	case status := <-outcome:
		return verdict(name, status), false
	case <-time.After(commandTimeout() + outcomeGrace):
		return fmt.Sprintf("no outcome was recorded for the command to %s", name), false
	case <-ctx.Done():
		return "", true
	}
}

// verdict is empty for an acknowledged command.
func verdict(name string, status string) string {
	switch status {
	case commandAcknowledged:
		return ""
	case commandTimedOut:
		return fmt.Sprintf("%s did not acknowledge the command within %s", name, commandTimeout())
	default:
		return fmt.Sprintf("the command to %s %s", name, status)
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

func describeStep(ctx context.Context, sensors SensorLookup, step Step) string {
	return fmt.Sprintf("set %s %s to %s", sensorName(ctx, sensors, step.SensorID), step.Property, step.Value)
}

func sensorName(ctx context.Context, sensors SensorLookup, id int) string {
	if sensor, err := sensors.ServiceGetSensorById(ctx, id); err == nil && sensor != nil {
		return sensor.Name
	}
	return fmt.Sprintf("sensor %d", id)
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
