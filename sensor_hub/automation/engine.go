package automation

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel/metric"
)

// engine keeps the schedule of every enabled automation in the scheduler and
// starts a run when one of its triggers comes due.
type engine struct {
	store     Store
	executor  *executor
	scheduler *scheduler
	logger    *slog.Logger
	now       func() time.Time
	lateness  metric.Float64Histogram

	mu          sync.Mutex
	runCtx      context.Context
	zone        *time.Location
	automations map[int]Automation
	triggers    map[int]int       // trigger ID to automation ID
	lastDue     map[int]time.Time // automation ID to the due time of its latest scheduled run
}

func newEngine(store Store, executor *executor, logger *slog.Logger, now func() time.Time, lateness metric.Float64Histogram) *engine {
	e := &engine{
		store:       store,
		executor:    executor,
		logger:      logger,
		now:         now,
		lateness:    lateness,
		runCtx:      context.Background(),
		zone:        time.UTC,
		automations: make(map[int]Automation),
		triggers:    make(map[int]int),
		lastDue:     make(map[int]time.Time),
	}
	e.scheduler = newScheduler(e.fire, now)
	return e
}

// load replaces everything the engine knows. Runs it starts afterwards run
// under runCtx.
func (e *engine) load(runCtx context.Context, zone *time.Location, automations []Automation) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.runCtx = runCtx
	e.zone = zone
	for id := range e.automations {
		e.dropLocked(id)
	}
	for _, automation := range automations {
		e.putLocked(automation)
	}
}

// put arms a new or changed automation, recomputing its triggers' due times.
func (e *engine) put(automation Automation) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.dropLocked(automation.ID)
	e.putLocked(automation)
}

func (e *engine) forget(automationID int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.dropLocked(automationID)
}

// setZone recomputes every due time in a new hub timezone.
func (e *engine) setZone(zone *time.Location) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if zone.String() == e.zone.String() {
		return
	}
	e.logger.Info("hub timezone changed; recomputing automation schedules", "from", e.zone.String(), "to", zone.String())
	e.zone = zone
	for _, automation := range e.automations {
		e.armLocked(automation)
	}
}

func (e *engine) zoneName() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.zone.String()
}

// nextFireAt is when the automation's earliest trigger next comes due, or nil
// when it is off.
func (e *engine) nextFireAt(automationID int) *time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	automation, ok := e.automations[automationID]
	if !ok || !automation.Enabled {
		return nil
	}
	var next *time.Time
	for _, trigger := range automation.Triggers {
		if due, ok := e.scheduler.due(trigger.ID); ok && (next == nil || due.Before(*next)) {
			next = &due
		}
	}
	return next
}

func (e *engine) putLocked(automation Automation) {
	e.automations[automation.ID] = automation
	for _, trigger := range automation.Triggers {
		e.triggers[trigger.ID] = automation.ID
	}
	e.armLocked(automation)
}

func (e *engine) armLocked(automation Automation) {
	if !automation.Enabled {
		return
	}
	now := e.now()
	for _, trigger := range automation.Triggers {
		if trigger.Schedule != nil {
			e.scheduler.set(trigger.ID, trigger.Schedule.NextAfter(now, e.zone))
		}
	}
}

func (e *engine) dropLocked(automationID int) {
	automation, ok := e.automations[automationID]
	if !ok {
		return
	}
	for _, trigger := range automation.Triggers {
		e.scheduler.remove(trigger.ID)
		delete(e.triggers, trigger.ID)
	}
	delete(e.automations, automationID)
	delete(e.lastDue, automationID)
}

// fire is the scheduler's callback for a trigger that has come due. It arms
// the trigger's next due time and starts a run. Two triggers of one
// automation due at the same moment start one run between them.
func (e *engine) fire(triggerID int, due time.Time) {
	e.mu.Lock()
	automation, ok := e.automations[e.triggers[triggerID]]
	if !ok || !automation.Enabled {
		e.mu.Unlock()
		return
	}
	var trigger Trigger
	for _, candidate := range automation.Triggers {
		if candidate.ID == triggerID {
			trigger = candidate
		}
	}
	if trigger.Schedule != nil {
		// A scheduler that fell behind, such as on a host that slept, arms
		// the next time after now, so the times it missed do not all fire
		// at once.
		e.scheduler.set(triggerID, trigger.Schedule.NextAfter(later(due, e.now()), e.zone))
	}
	duplicate := e.lastDue[automation.ID].Equal(due)
	e.lastDue[automation.ID] = due
	runCtx := e.runCtx
	e.mu.Unlock()

	if duplicate {
		return
	}
	e.lateness.Record(runCtx, float64(e.now().Sub(due).Milliseconds()))
	e.start(runCtx, automation, trigger)
}

func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

func (e *engine) start(ctx context.Context, automation Automation, trigger Trigger) {
	triggerID := trigger.ID
	run := Run{
		AutomationID: automation.ID,
		TriggerID:    &triggerID,
		TriggerKind:  trigger.Kind,
		Status:       RunRunning,
		Steps:        automation.Steps,
		StartedAt:    e.now(),
	}
	id, err := e.store.CreateRun(ctx, run)
	if err != nil {
		e.logger.Error("could not start automation run", "automation_id", automation.ID, "trigger_id", triggerID, "error", err)
		return
	}
	run.ID = id
	go e.executor.execute(ctx, automation, run)
}
