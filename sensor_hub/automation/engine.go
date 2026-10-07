package automation

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	gen "example/sensorHub/gen"

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
	// Only enabled automations' reading triggers are in series and edges.
	series map[Series][]Trigger
	edges  map[int]*edge // trigger ID to edge
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
		series:      make(map[Series][]Trigger),
		edges:       make(map[int]*edge),
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
		e.executor.recordMissed(runCtx, overdue.automation, overdue.trigger, overdue.due, now.Sub(overdue.due)-grace)
	}
	// A caught-up run is admitted like any other, so in restart mode it
	// cancels a run that would otherwise resume below.
	cancelled := make(map[int]bool)
	for _, overdue := range catchUp {
		e.lateness.Record(runCtx, float64(now.Sub(overdue.due).Milliseconds()))
		for _, runID := range e.startTriggeredRun(runCtx, overdue.automation, overdue.trigger, overdue.due).Cancelled {
			cancelled[runID] = true
		}
	}
	for _, run := range active {
		if cancelled[run.ID] {
			continue
		}
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
}

// lastDueBefore returns the latest time before now that the trigger came due
// while the hub was down.
func (e *engine) lastDueBefore(automation Automation, trigger Trigger, now time.Time) (time.Time, bool) {
	if !automation.Enabled || trigger.NextDueAt == nil || trigger.NextDueAt.After(now) {
		return time.Time{}, false
	}
	due := *trigger.NextDueAt
	for {
		next := e.nextDue(trigger, due, due)
		if next.After(now) {
			return due, true
		}
		due = next
	}
}

func (e *engine) put(automation Automation) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.automations[automation.ID].Enabled {
		// Switching an automation on starts its interval triggers counting
		// from now, not from when they were due before it was switched off.
		automation.Triggers = slices.Clone(automation.Triggers)
		for i := range automation.Triggers {
			automation.Triggers[i].NextDueAt = nil
		}
	}
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
	now := e.now()
	for _, automation := range e.automations {
		if !automation.Enabled {
			continue
		}
		for _, trigger := range automation.Triggers {
			if trigger.Kind == TriggerSchedule {
				e.armTriggerLocked(trigger, now, now)
			}
		}
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
		if trigger.Kind == TriggerReading {
			e.series[trigger.Reading.Series] = append(e.series[trigger.Reading.Series], trigger)
			e.edges[trigger.ID] = &edge{}
			continue
		}
		from := now
		if trigger.NextDueAt != nil {
			from = *trigger.NextDueAt
		}
		e.armTriggerLocked(trigger, from, now)
	}
}

func (e *engine) armTriggerLocked(trigger Trigger, from, after time.Time) {
	due := e.nextDue(trigger, from, after)
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
		e.scheduler.remove(holdKey(trigger.ID))
		delete(e.triggers, trigger.ID)
		delete(e.edges, trigger.ID)
		if trigger.Kind == TriggerReading {
			series := trigger.Reading.Series
			e.series[series] = slices.DeleteFunc(e.series[series], func(each Trigger) bool { return each.ID == trigger.ID })
			if len(e.series[series]) == 0 {
				delete(e.series, series)
			}
		}
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
	case dueHold:
		e.fireHold(key.id, due)
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
	trigger := triggerOf(automation, triggerID)
	// A scheduler that fell behind, such as on a host that slept, arms the
	// next time after now, so the times it missed do not all fire at once.
	e.armTriggerLocked(trigger, due, later(due, e.now()))
	duplicate := e.lastDue[automation.ID].Equal(due)
	e.lastDue[automation.ID] = due
	runCtx := e.runCtx
	e.mu.Unlock()

	if duplicate {
		return
	}
	e.lateness.Record(runCtx, float64(e.now().Sub(due).Milliseconds()))
	e.startTriggeredRun(runCtx, automation, trigger, due)
}

func triggerOf(automation Automation, triggerID int) Trigger {
	for _, trigger := range automation.Triggers {
		if trigger.ID == triggerID {
			return trigger
		}
	}
	return Trigger{}
}

// Readings must arrive one at a time, in the order they were ingested.
func (e *engine) observe(series Series, reading gen.Reading) {
	now := e.now()
	type toStart struct {
		automation Automation
		trigger    Trigger
	}
	var fired []toStart
	started := make(map[int]bool)
	e.mu.Lock()
	for _, trigger := range e.series[series] {
		switch e.edges[trigger.ID].observe(*trigger.Reading, reading, now) {
		case edgeFire:
			automation := e.automations[e.triggers[trigger.ID]]
			// Two triggers of one automation met by the same reading start
			// one run between them.
			if !started[automation.ID] {
				started[automation.ID] = true
				fired = append(fired, toStart{automation, trigger})
			}
		case edgeHold:
			e.scheduler.set(holdKey(trigger.ID), e.edges[trigger.ID].holdUntil)
		case edgeRelease:
			e.scheduler.remove(holdKey(trigger.ID))
		}
	}
	runCtx := e.runCtx
	e.mu.Unlock()

	for _, each := range fired {
		e.startTriggeredRun(runCtx, each.automation, each.trigger, now)
	}
}

func (e *engine) fireHold(triggerID int, due time.Time) {
	e.mu.Lock()
	held, ok := e.edges[triggerID]
	if !ok || !held.holdElapsed(due) {
		e.mu.Unlock()
		return
	}
	automation := e.automations[e.triggers[triggerID]]
	runCtx := e.runCtx
	e.mu.Unlock()

	e.lateness.Record(runCtx, float64(e.now().Sub(due).Milliseconds()))
	e.startTriggeredRun(runCtx, automation, triggerOf(automation, triggerID), due)
}

// An interval trigger keeps the phase of from, a time it came due or the
// moment it started counting.
func (e *engine) nextDue(trigger Trigger, from, after time.Time) time.Time {
	if trigger.Kind == TriggerInterval {
		if from.After(after) {
			return from
		}
		return from.Add((after.Sub(from)/trigger.Interval + 1) * trigger.Interval)
	}
	return trigger.Schedule.NextAfter(after, e.zone)
}

func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

func (e *engine) startTriggeredRun(ctx context.Context, automation Automation, trigger Trigger, due time.Time) RunAdmission {
	triggerID := trigger.ID
	admission, err := e.startRun(ctx, automation, Run{TriggerID: &triggerID, TriggerKind: trigger.Kind})
	if err != nil {
		e.logger.Error("could not start automation run", "automation_id", automation.ID, "trigger_id", triggerID, "due", due, "error", err)
	}
	return admission
}

// runNow starts a run whether or not the automation is enabled.
func (e *engine) runNow(automation Automation, user User) (Run, error) {
	e.mu.Lock()
	runCtx := e.runCtx
	e.mu.Unlock()
	admission, err := e.startRun(runCtx, automation, Run{TriggerKind: TriggerManual, InitiatedBy: &user})
	if err != nil {
		return Run{}, err
	}
	return admission.Run, nil
}

func (e *engine) startRun(ctx context.Context, automation Automation, run Run) (RunAdmission, error) {
	run.AutomationID = automation.ID
	run.Steps = automation.Steps
	run.StartedAt = e.now()
	admission, err := e.store.AdmitRun(ctx, run, automation.Mode)
	if err != nil {
		return RunAdmission{}, err
	}
	logger := e.logger.With("automation_id", automation.ID, "run_id", admission.Run.ID)
	for _, cancelled := range admission.Cancelled {
		e.dropCancelled(ctx, cancelled)
		logger.Info("automation run cancelled to start over", "cancelled_run_id", cancelled)
	}
	if admission.Run.Status == RunSkipped {
		e.executor.countEnded(ctx, RunSkipped)
		logger.Info("automation already running; run skipped", "trigger_kind", run.TriggerKind)
		return admission, nil
	}
	go e.run(ctx, automation, admission.Run)
	return admission, nil
}

func (e *engine) cancel(ctx context.Context, automationID int, runID int) (Run, error) {
	run, err := e.store.CancelRun(ctx, automationID, runID, e.now())
	if err != nil {
		return Run{}, err
	}
	e.dropCancelled(ctx, runID)
	e.logger.Info("automation run cancelled", "automation_id", automationID, "run_id", runID)
	return run, nil
}

// A cancelled run that was mid-step stops at its next step, when the store
// refuses to move it on.
func (e *engine) dropCancelled(ctx context.Context, runID int) {
	e.scheduler.remove(resumeKey(runID))
	e.executor.countEnded(ctx, RunCancelled)
}

func (e *engine) resume(ctx context.Context, runID int) {
	run, err := e.store.ResumeRun(ctx, runID, e.now())
	if errors.Is(err, ErrRunGone) {
		e.logger.Info("waiting automation run was cancelled or deleted", "run_id", runID)
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
