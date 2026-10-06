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
	"example/sensorHub/periodic"
	"example/sensorHub/telemetry"

	"go.opentelemetry.io/otel/metric"
)

var ErrNotFound = errors.New("automation not found")

const defaultMissedGrace = 10 * time.Minute

// A Store must not hold a transaction across a command round trip.
type Store interface {
	ListAutomations(ctx context.Context) ([]Automation, error)
	// Missing automations are ErrNotFound here and in the update, enable and
	// delete methods.
	GetAutomation(ctx context.Context, id int) (Automation, error)
	CreateAutomation(ctx context.Context, automation Automation) (Automation, error)
	UpdateAutomation(ctx context.Context, automation Automation) (Automation, error)
	SetAutomationEnabled(ctx context.Context, id int, enabled bool) (Automation, error)
	// DeleteAutomation returns ErrActiveRun, and deletes nothing, while the
	// automation has a running or waiting run.
	DeleteAutomation(ctx context.Context, id int) error
	RunStates(ctx context.Context) (map[int]RunState, error)

	SetTriggerDue(ctx context.Context, triggerID int, due time.Time) error

	CreateRun(ctx context.Context, run Run) (int, error)
	// AdmitRun records a run as running unless the automation already has an
	// active run. Then it records a skipped run in single mode, and in restart
	// mode cancels the active runs first, in the same transaction.
	AdmitRun(ctx context.Context, run Run, mode Mode) (RunAdmission, error)
	// CancelRun returns ErrRunNotFound when the automation has no such run, and
	// ErrRunNotActive when the run has already ended.
	CancelRun(ctx context.Context, automationID int, runID int, at time.Time) (Run, error)
	// StartRunStep, WaitRun, ResumeRun and FinishRun return ErrRunGone when the
	// run has been cancelled or deleted.
	StartRunStep(ctx context.Context, runID int, position int, kind StepKind, at time.Time) (int, error)
	FinishRunStep(ctx context.Context, stepID int, outcome StepOutcome, commandID *int, at time.Time) error
	// WaitRun records a wait step as started and the run as waiting until
	// resumeAt, in one transaction.
	WaitRun(ctx context.Context, runID int, position int, at time.Time, resumeAt time.Time) error
	// ResumeRun finishes the wait step a run is on and sets it running again,
	// in one transaction.
	ResumeRun(ctx context.Context, runID int, at time.Time) (Run, error)
	FinishRun(ctx context.Context, runID int, status RunStatus, message *string, at time.Time) error
	ListRuns(ctx context.Context, automationID int) ([]Run, error)
	// ActiveRuns returns the running and waiting runs with their step outcomes.
	ActiveRuns(ctx context.Context) ([]Run, error)
	// LatestRunCommand returns the ID of the newest command the run sent for
	// the step's sensor and property since a time, and false when there is none.
	LatestRunCommand(ctx context.Context, runID int, step Step, since time.Time) (int, bool, error)
}

// SensorLookup returns a sensor with its writable capabilities filled in, or
// an error wrapping sql.ErrNoRows when there is no such sensor.
type SensorLookup interface {
	ServiceGetSensorById(ctx context.Context, id int) (*gen.Sensor, error)
}

// The ID is non-zero whenever the command was recorded, even alongside an
// error. The channel receives the command's final status once.
type CommandSender interface {
	SendAsSystem(ctx context.Context, sensorID int, property string, value string, automationRunID int) (int, <-chan string, error)
	// AwaitOutcome is the outcome channel of a command sent before a restart.
	AwaitOutcome(ctx context.Context, commandID int) (<-chan string, error)
}

type Notifier interface {
	CreateNotification(ctx context.Context, notification notifications.Notification, targetPermission string) (int, error)
}

type Service struct {
	store   Store
	sensors SensorLookup
	engine  *engine
	logger  *slog.Logger
	now     func() time.Time
}

func NewService(store Store, sensors SensorLookup, commands CommandSender, notifier Notifier, logger *slog.Logger) *Service {
	logger = logger.With("component", "automation")
	now := func() time.Time { return time.Now().UTC() }
	executor := &executor{
		store:     store,
		sensors:   sensors,
		commands:  commands,
		notifier:  notifier,
		logger:    logger,
		now:       now,
		tracer:    telemetry.Tracer("automation"),
		runsEnded: newRunsEndedCounter(),
	}
	lateness, _ := telemetry.Meter("automation").Float64Histogram("automation.scheduler.lateness",
		metric.WithDescription("How long after its due time a scheduled run started"),
		metric.WithUnit("ms"))
	return &Service{
		store:   store,
		sensors: sensors,
		engine:  newEngine(store, executor, logger, now, lateness),
		logger:  logger,
		now:     now,
	}
}

func (s *Service) Start(ctx context.Context) error {
	automations, err := s.store.ListAutomations(ctx)
	if err != nil {
		return fmt.Errorf("load automations: %w", err)
	}
	active, err := s.store.ActiveRuns(ctx)
	if err != nil {
		return fmt.Errorf("load active automation runs: %w", err)
	}
	s.engine.load(ctx, hubZone(), automations, active, missedGrace())

	stopListening := appProps.OnReload(func(cfg *appProps.ApplicationConfiguration) {
		s.engine.setZone(cfg.HubLocation())
	})
	go func() {
		<-ctx.Done()
		stopListening()
	}()

	periodic.Supervise(ctx, "automation-scheduler", s.logger, s.engine.scheduler.run)
	s.logger.Info("automation scheduler started", "automations", len(automations), "resumed_runs", len(active), "hub_timezone", s.engine.zoneName())
	return nil
}

func (s *Service) List(ctx context.Context) ([]gen.Automation, error) {
	automations, err := s.store.ListAutomations(ctx)
	if err != nil {
		return nil, err
	}
	states, err := s.store.RunStates(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]gen.Automation, 0, len(automations))
	for _, automation := range automations {
		views = append(views, s.view(automation, states[automation.ID]))
	}
	return views, nil
}

func (s *Service) Get(ctx context.Context, id int) (gen.Automation, error) {
	automation, err := s.store.GetAutomation(ctx, id)
	if err != nil {
		return gen.Automation{}, err
	}
	return s.viewWithState(ctx, automation)
}

func (s *Service) Create(ctx context.Context, input gen.AutomationInput) (gen.Automation, error) {
	automation, err := fromInput(ctx, s.sensors, input)
	if err != nil {
		return gen.Automation{}, err
	}
	saved, err := s.store.CreateAutomation(ctx, automation)
	if err != nil {
		return gen.Automation{}, err
	}
	s.engine.put(saved)
	s.logger.Info("automation created", "automation_id", saved.ID, "name", saved.Name)
	return s.viewWithState(ctx, saved)
}

func (s *Service) Update(ctx context.Context, id int, input gen.AutomationInput) (gen.Automation, error) {
	current, err := s.store.GetAutomation(ctx, id)
	if err != nil {
		return gen.Automation{}, err
	}
	automation, err := fromInput(ctx, s.sensors, input)
	if err != nil {
		return gen.Automation{}, err
	}
	automation.ID = id
	if input.Enabled == nil {
		automation.Enabled = current.Enabled
	}
	if input.Mode == nil {
		automation.Mode = current.Mode
	}
	saved, err := s.store.UpdateAutomation(ctx, automation)
	if err != nil {
		return gen.Automation{}, err
	}
	s.engine.put(saved)
	s.logger.Info("automation updated", "automation_id", saved.ID, "name", saved.Name)
	return s.viewWithState(ctx, saved)
}

func (s *Service) SetEnabled(ctx context.Context, id int, enabled bool) (gen.Automation, error) {
	saved, err := s.store.SetAutomationEnabled(ctx, id, enabled)
	if err != nil {
		return gen.Automation{}, err
	}
	s.engine.put(saved)
	s.logger.Info("automation switched", "automation_id", id, "enabled", enabled)
	return s.viewWithState(ctx, saved)
}

func (s *Service) Delete(ctx context.Context, id int) error {
	if err := s.store.DeleteAutomation(ctx, id); err != nil {
		return err
	}
	s.engine.forget(id)
	s.logger.Info("automation deleted", "automation_id", id)
	return nil
}

// RunNow applies the automation's mode as a trigger would, so the run it
// returns can be a skipped one.
func (s *Service) RunNow(ctx context.Context, id int, userID int) (gen.AutomationRun, error) {
	automation, err := s.store.GetAutomation(ctx, id)
	if err != nil {
		return gen.AutomationRun{}, err
	}
	run, err := s.engine.runNow(automation, User{ID: userID})
	if err != nil {
		return gen.AutomationRun{}, err
	}
	s.logger.Info("automation run requested", "automation_id", id, "run_id", run.ID, "user_id", userID, "status", run.Status)
	return runView(run), nil
}

func (s *Service) CancelRun(ctx context.Context, id int, runID int) (gen.AutomationRun, error) {
	run, err := s.engine.cancel(ctx, id, runID)
	if err != nil {
		return gen.AutomationRun{}, err
	}
	return runView(run), nil
}

func (s *Service) Runs(ctx context.Context, id int) ([]gen.AutomationRun, error) {
	if _, err := s.store.GetAutomation(ctx, id); err != nil {
		return nil, err
	}
	runs, err := s.store.ListRuns(ctx, id)
	if err != nil {
		return nil, err
	}
	views := make([]gen.AutomationRun, 0, len(runs))
	for _, run := range runs {
		views = append(views, runView(run))
	}
	return views, nil
}

func (s *Service) viewWithState(ctx context.Context, automation Automation) (gen.Automation, error) {
	states, err := s.store.RunStates(ctx)
	if err != nil {
		return gen.Automation{}, err
	}
	return s.view(automation, states[automation.ID]), nil
}

func (s *Service) view(automation Automation, state RunState) gen.Automation {
	return automationView(automation, state, s.engine.nextFireAt(automation.ID), s.engine.zoneName())
}

func missedGrace() time.Duration {
	if cfg := appProps.AppConfig(); cfg != nil {
		return time.Duration(cfg.AutomationMissedGraceMinutes) * time.Minute
	}
	return defaultMissedGrace
}

func hubZone() *time.Location {
	if cfg := appProps.AppConfig(); cfg != nil {
		return cfg.HubLocation()
	}
	return time.UTC
}
