package automation

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

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

// load runs once, on startup, before the scheduler runs.
func (e *engine) load(runCtx context.Context, zone *time.Location, automations []Automation, active []Run, grace time.Duration) {
	now := e.now()
	type overdueTrigger struct {
		automation Automation
		trigger    Trigger
		due        time.Time
	}
	var catchUp, missed []overdueTrigger

	e.mu.Lock()
	e.runCtx = runCtx
	e.zone = zone
	for _, automation := range automations {
		caughtUp := false
		for _, trigger := range automation.Triggers {
			due, ok := e.lastDueBefore(automation, trigger, now)
			switch {
			case !ok:
			case now.Sub(due) > grace:
				missed = append(missed, overdueTrigger{automation, trigger, due})
			case !caughtUp:
				caughtUp = true
				e.lastDue[automation.ID] = due
				catchUp = append(catchUp, overdueTrigger{automation, trigger, due})
			}
		}
		e.putLocked(automation)
	}
	e.mu.Unlock()

	for _, overdue := range missed {
		e.recordMissed(runCtx, overdue.automation, overdue.trigger, overdue.due, now.Sub(overdue.due)-grace)
	}
	for _, run := range active {
		if run.Status == RunWaiting && run.ResumeAt != nil && run.ResumeAt.After(now) {
			e.scheduler.set(resumeKey(run.ID), *run.ResumeAt)
			continue
		}
		if run.Status == RunWaiting {
			go e.resume(runCtx, run.ID)
			continue
		}
		go e.run(runCtx, e.automationOf(run), run)
	}
	for _, overdue := range catchUp {
		e.lateness.Record(runCtx, float64(now.Sub(overdue.due).Milliseconds()))
		e.startRun(runCtx, overdue.automation, overdue.trigger, overdue.due)
	}
}

// lastDueBefore returns the latest time before now that the trigger came due
// while the hub was down.
func (e *engine) lastDueBefore(automation Automation, trigger Trigger, now time.Time) (time.Time, bool) {
	if !automation.Enabled || trigger.Schedule == nil || trigger.NextDueAt == nil || trigger.NextDueAt.After(now) {
		return time.Time{}, false
	}
	due := *trigger.NextDueAt
	for {
		next := trigger.Schedule.NextAfter(due, e.zone)
		if next.After(now) {
			return due, true
		}
		due = next
	}
}

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

func (e *engine) nextFireAt(automationID int) *time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	automation, ok := e.automations[automationID]
	if !ok || !automation.Enabled {
		return nil
	}
	var next *time.Time
	for _, trigger := range automation.Triggers {
		if due, ok := e.scheduler.due(triggerKey(trigger.ID)); ok && (next == nil || due.Before(*next)) {
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
		e.armTriggerLocked(trigger, now)
	}
}

func (e *engine) armTriggerLocked(trigger Trigger, after time.Time) {
	if trigger.Schedule == nil {
		return
	}
	due := trigger.Schedule.NextAfter(after, e.zone)
	e.scheduler.set(triggerKey(trigger.ID), due)
	if err := e.store.SetTriggerDue(e.runCtx, trigger.ID, due); err != nil {
		e.logger.Error("could not save when an automation trigger is next due", "trigger_id", trigger.ID, "error", err)
	}
}

func (e *engine) dropLocked(automationID int) {
	automation, ok := e.automations[automationID]
	if !ok {
		return
	}
	for _, trigger := range automation.Triggers {
		e.scheduler.remove(triggerKey(trigger.ID))
		delete(e.triggers, trigger.ID)
	}
	delete(e.automations, automationID)
	delete(e.lastDue, automationID)
}

func (e *engine) fire(key dueKey, due time.Time) {
	switch key.kind {
	case dueTrigger:
		e.fireTrigger(key.id, due)
	case dueResume:
		e.mu.Lock()
		runCtx := e.runCtx
		e.mu.Unlock()
		go e.resume(runCtx, key.id)
	}
}

// Two triggers of one automation due at the same moment start one run
// between them.
func (e *engine) fireTrigger(triggerID int, due time.Time) {
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
	// A scheduler that fell behind, such as on a host that slept, arms the
	// next time after now, so the times it missed do not all fire at once.
	e.armTriggerLocked(trigger, later(due, e.now()))
	duplicate := e.lastDue[automation.ID].Equal(due)
	e.lastDue[automation.ID] = due
	runCtx := e.runCtx
	e.mu.Unlock()

	if duplicate {
		return
	}
	e.lateness.Record(runCtx, float64(e.now().Sub(due).Milliseconds()))
	e.startRun(runCtx, automation, trigger, due)
}

func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

func (e *engine) startRun(ctx context.Context, automation Automation, trigger Trigger, due time.Time) {
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
		e.logger.Error("could not start automation run", "automation_id", automation.ID, "trigger_id", triggerID, "due", due, "error", err)
		return
	}
	run.ID = id
	go e.run(ctx, automation, run)
}

func (e *engine) recordMissed(ctx context.Context, automation Automation, trigger Trigger, due time.Time, pastGrace time.Duration) {
	triggerID := trigger.ID
	now := e.now()
	run := Run{
		AutomationID: automation.ID,
		TriggerID:    &triggerID,
		TriggerKind:  trigger.Kind,
		Status:       RunMissed,
		Steps:        automation.Steps,
		StartedAt:    now,
		FinishedAt:   &now,
		DueAt:        &due,
		PastGrace:    &pastGrace,
	}
	id, err := e.store.CreateRun(ctx, run)
	if err != nil {
		e.logger.Error("could not record missed automation run", "automation_id", automation.ID, "trigger_id", triggerID, "due", due, "error", err)
		return
	}
	e.executor.runsEnded.Add(ctx, 1, metric.WithAttributes(attribute.String("status", string(RunMissed))))
	e.logger.Warn("automation trigger came due while the hub was down, past the grace window; not running it",
		"automation_id", automation.ID, "run_id", id, "trigger_id", triggerID, "due", due, "past_grace", pastGrace)
}

func (e *engine) resume(ctx context.Context, runID int) {
	run, err := e.store.ResumeRun(ctx, runID, e.now())
	if errors.Is(err, ErrRunGone) {
		e.logger.Info("waiting automation run was deleted with its automation", "run_id", runID)
		return
	}
	if err != nil {
		e.logger.Error("could not resume waiting automation run", "run_id", runID, "error", err)
		return
	}
	e.run(ctx, e.automationOf(run), run)
}

func (e *engine) run(ctx context.Context, automation Automation, run Run) {
	if resumeAt, waiting := e.executor.execute(ctx, automation, run); waiting {
		e.scheduler.set(resumeKey(run.ID), resumeAt)
	}
}

// A run's automation is only missing when it was deleted, and the run with
// it, which the executor finds out on its next step.
func (e *engine) automationOf(run Run) Automation {
	e.mu.Lock()
	defer e.mu.Unlock()
	if automation, ok := e.automations[run.AutomationID]; ok {
		return automation
	}
	return Automation{ID: run.AutomationID}
}
