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

// Store persists automations and their runs. Automation writes go through
// the writer connection, and none of them holds a transaction across a
// command round trip.
type Store interface {
	ListAutomations(ctx context.Context) ([]Automation, error)
	// GetAutomation returns ErrNotFound when there is no such automation, as
	// do the update, enable and delete methods.
	GetAutomation(ctx context.Context, id int) (Automation, error)
	CreateAutomation(ctx context.Context, automation Automation) (Automation, error)
	UpdateAutomation(ctx context.Context, automation Automation) (Automation, error)
	SetAutomationEnabled(ctx context.Context, id int, enabled bool) (Automation, error)
	DeleteAutomation(ctx context.Context, id int) error
	RunStates(ctx context.Context) (map[int]RunState, error)

	CreateRun(ctx context.Context, run Run) (int, error)
	// StartRunStep records that a step has started and moves the run onto it.
	// It returns ErrRunGone when the run has been deleted.
	StartRunStep(ctx context.Context, runID int, position int, kind StepKind, at time.Time) (int, error)
	FinishRunStep(ctx context.Context, stepID int, outcome StepOutcome, commandID *int, at time.Time) error
	FinishRun(ctx context.Context, runID int, status RunStatus, message *string, at time.Time) error
	ListRuns(ctx context.Context, automationID int) ([]Run, error)
	// FailRunningRuns ends every run still marked running, returning how many.
	FailRunningRuns(ctx context.Context, message string, at time.Time) (int, error)
}

// SensorLookup returns a sensor with its writable capabilities filled in, or
// an error wrapping sql.ErrNoRows when there is no such sensor.
type SensorLookup interface {
	ServiceGetSensorById(ctx context.Context, id int) (*gen.Sensor, error)
}

// CommandSender sends a command as the system actor for an automation run.
// The ID is non-zero whenever the command was recorded, even alongside an
// error. The channel receives the command's final status once.
type CommandSender interface {
	SendAsSystem(ctx context.Context, sensorID int, property string, value string, automationRunID int) (int, <-chan string, error)
}

type Notifier interface {
	CreateNotification(ctx context.Context, notification notifications.Notification, targetPermission string) (int, error)
}

// Service is how the rest of the hub manages automations. Saves take effect
// on the running schedule straight away.
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

// Start loads every automation, arms the scheduler and keeps it running
// until ctx is cancelled. Runs left running by a previous process are ended
// as failed first.
func (s *Service) Start(ctx context.Context) error {
	interrupted, err := s.store.FailRunningRuns(ctx, "the hub stopped during the run", s.now())
	if err != nil {
		return fmt.Errorf("end interrupted automation runs: %w", err)
	}
	if interrupted > 0 {
		s.logger.Warn("ended automation runs interrupted by a restart", "count", interrupted)
	}

	automations, err := s.store.ListAutomations(ctx)
	if err != nil {
		return fmt.Errorf("load automations: %w", err)
	}
	s.engine.load(ctx, hubZone(), automations)

	stopListening := appProps.OnReload(func(cfg *appProps.ApplicationConfiguration) {
		s.engine.setZone(cfg.HubLocation())
	})
	go func() {
		<-ctx.Done()
		stopListening()
	}()

	periodic.Supervise(ctx, "automation-scheduler", s.logger, s.engine.scheduler.run)
	s.logger.Info("automation scheduler started", "automations", len(automations), "hub_timezone", s.engine.zoneName())
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

// Delete removes an automation with its triggers, steps and runs. The
// commands its runs sent stay in command history.
func (s *Service) Delete(ctx context.Context, id int) error {
	if err := s.store.DeleteAutomation(ctx, id); err != nil {
		return err
	}
	s.engine.forget(id)
	s.logger.Info("automation deleted", "automation_id", id)
	return nil
}

// Runs returns an automation's runs, newest first.
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

func hubZone() *time.Location {
	if cfg := appProps.AppConfig(); cfg != nil {
		return cfg.HubLocation()
	}
	return time.UTC
}
