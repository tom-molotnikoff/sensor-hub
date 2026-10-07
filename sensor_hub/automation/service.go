package automation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	appProps "example/sensorHub/application_properties"
	gen "example/sensorHub/gen"
	"example/sensorHub/notifications"
	"example/sensorHub/periodic"
	"example/sensorHub/telemetry"

	"go.opentelemetry.io/otel/metric"
)

var ErrNotFound = errors.New("automation not found")

const (
	defaultMissedGrace   = 10 * time.Minute
	defaultMaxCauseChain = 5
)

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
	// SetBrokenReason stores the reason, empty when the automation is not
	// broken, and returns the reason it replaced.
	SetBrokenReason(ctx context.Context, id int, reason string) (string, error)

	SetTriggerDue(ctx context.Context, triggerID int, due time.Time) error
	// SetMarginHint clears the hint when given nil.
	SetMarginHint(ctx context.Context, triggerID int, hint *MarginHint) error

	CreateRun(ctx context.Context, run Run) (int, error)
	// CauseChain returns the run and the runs that caused it, newest first,
	// up to limit of them.
	CauseChain(ctx context.Context, runID int, limit int) ([]CauseRun, error)
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
	// CancelledRunCommand returns the ID of a command still in flight for the
	// step's sensor and property that a cancelled run of the automation sent,
	// and false when there is none.
	CancelledRunCommand(ctx context.Context, automationID int, step Step) (int, bool, error)
}

// SensorLookup returns a sensor with its writable capabilities filled in, or
// an error wrapping sql.ErrNoRows when there is no such sensor.
type SensorLookup interface {
	ServiceGetSensorById(ctx context.Context, id int) (*gen.Sensor, error)
	ServiceGetMeasurementTypesForSensor(ctx context.Context, sensorID int) ([]gen.MeasurementType, error)
}

// The ID is non-zero whenever the command was recorded, even alongside an
// error. The channel receives the command's final status once.
type CommandSender interface {
	SendAsSystem(ctx context.Context, sensorID int, property string, value string, automationRunID int) (int, <-chan string, error)
	// AwaitOutcome is another channel for a command's final status, for a
	// command sent before a restart or by another run.
	AwaitOutcome(ctx context.Context, commandID int) (<-chan string, error)
}

type Notifier interface {
	CreateNotification(ctx context.Context, notification notifications.Notification, targetPermission string) (int, error)
}

type Service struct {
	store    Store
	sensors  SensorLookup
	readings *ReadingConsumer
	history  ReadingHistory
	engine   *engine
	logger   *slog.Logger
	now      func() time.Time
	// rechecking keeps two re-checks of one automation from storing their
	// reasons in the opposite order to the sensor changes they saw, and holds
	// a re-check off until the engine has loaded what startup found.
	rechecking sync.Mutex
}

func NewService(store Store, sensors SensorLookup, commands CommandSender, notifier Notifier, readings *ReadingConsumer, history ReadingHistory, logger *slog.Logger) *Service {
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
		store:    store,
		sensors:  sensors,
		readings: readings,
		history:  history,
		engine:   newEngine(store, executor, logger, now, lateness),
		logger:   logger,
		now:      now,
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
	s.rechecking.Lock()
	// A sensor can change while the hub is down, or without the re-check
	// being told.
	for i := range automations {
		automations[i].BrokenReason = s.recheckLocked(ctx, automations[i])
	}
	s.engine.load(ctx, hubZone(), automations, active, missedGrace())
	s.rechecking.Unlock()

	stopListening := appProps.OnReload(func(cfg *appProps.ApplicationConfiguration) {
		s.engine.setZone(cfg.HubLocation())
	})
	go func() {
		<-ctx.Done()
		stopListening()
	}()

	periodic.Supervise(ctx, "automation-scheduler", s.logger, s.engine.scheduler.run)
	periodic.Supervise(ctx, "automation-readings", s.logger, func(ctx context.Context, healthy func()) {
		s.readings.drain(ctx, healthy, s.engine.observe)
	})
	periodic.RunTask(ctx, periodic.TaskConfig{
		Name:           "automation-margin-check",
		Interval:       func() time.Duration { return marginCheckInterval },
		Logger:         s.logger,
		RunImmediately: true,
	}, s.checkMargins)
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
	s.rechecking.Lock()
	saved.BrokenReason = s.recheckLocked(ctx, saved)
	s.rechecking.Unlock()
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

// SensorChanged must hear about every change to a sensor's metadata or driver.
func (s *Service) SensorChanged(ctx context.Context, sensorID int) {
	s.rechecking.Lock()
	defer s.rechecking.Unlock()
	automations, err := s.store.ListAutomations(ctx)
	if err != nil {
		s.logger.Error("could not load automations to re-check them against a changed sensor", "sensor_id", sensorID, "error", err)
		return
	}
	for _, automation := range automations {
		if automation.uses(sensorID) {
			s.recheckLocked(ctx, automation)
		}
	}
}

// SensorDeleted is told about a sensor once it has been deleted along with
// every automation that used it.
func (s *Service) SensorDeleted(sensorID int) {
	for _, id := range s.engine.forgetUsing(sensorID) {
		s.logger.Info("automation deleted with its sensor", "automation_id", id, "sensor_id", sensorID)
	}
}

func (s *Service) recheckLocked(ctx context.Context, automation Automation) string {
	logger := s.logger.With("automation_id", automation.ID)
	reason, err := brokenReason(ctx, s.sensors, automation.Steps)
	if err != nil {
		logger.Error("could not re-check whether an automation is broken", "error", err)
		return automation.BrokenReason
	}
	previous, err := s.store.SetBrokenReason(ctx, automation.ID, reason)
	if errors.Is(err, ErrNotFound) {
		return reason
	}
	if err != nil {
		logger.Error("could not store whether an automation is broken", "error", err)
		return automation.BrokenReason
	}
	if previous == reason {
		return reason
	}
	s.engine.setBroken(automation.ID, reason)
	switch {
	case previous == "":
		logger.Warn("automation broken", "reason", reason)
		s.engine.executor.notifyBroken(ctx, logger, automation, reason)
	case reason == "":
		logger.Info("automation no longer broken")
	}
	return reason
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

func maxCauseChain() int {
	if cfg := appProps.AppConfig(); cfg != nil && cfg.AutomationLoopMaxChain > 0 {
		return cfg.AutomationLoopMaxChain
	}
	return defaultMaxCauseChain
}

func hubZone() *time.Location {
	if cfg := appProps.AppConfig(); cfg != nil {
		return cfg.HubLocation()
	}
	return time.UTC
}
