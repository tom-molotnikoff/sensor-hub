package automation

import (
	"fmt"
	"time"

	gen "example/sensorHub/gen"
)

var dayOrder = []struct {
	day  time.Weekday
	name gen.AutomationTriggerDays
}{
	{time.Monday, gen.AutomationDayMon},
	{time.Tuesday, gen.AutomationDayTue},
	{time.Wednesday, gen.AutomationDayWed},
	{time.Thursday, gen.AutomationDayThu},
	{time.Friday, gen.AutomationDayFri},
	{time.Saturday, gen.AutomationDaySat},
	{time.Sunday, gen.AutomationDaySun},
}

func automationView(automation Automation, state RunState, nextFireAt *time.Time, zone string) gen.Automation {
	view := gen.Automation{
		Id:            automation.ID,
		Name:          automation.Name,
		Enabled:       automation.Enabled,
		Triggers:      make([]gen.AutomationTrigger, 0, len(automation.Triggers)),
		Steps:         stepViews(automation.Steps),
		Status:        gen.AutomationStatusArmed,
		LastRunFailed: state.LastRunFailed,
		HubTimezone:   zone,
		CreatedAt:     automation.CreatedAt.UTC(),
		UpdatedAt:     automation.UpdatedAt.UTC(),
	}
	switch {
	case !automation.Enabled:
		view.Status = gen.AutomationStatusOff
	case state.Running:
		view.Status = gen.AutomationStatusRunning
	}
	if nextFireAt != nil {
		next := nextFireAt.UTC()
		view.NextFireAt = &next
	}
	for _, trigger := range automation.Triggers {
		view.Triggers = append(view.Triggers, triggerView(trigger))
	}
	return view
}

func triggerView(trigger Trigger) gen.AutomationTrigger {
	id := trigger.ID
	view := gen.AutomationTrigger{Id: &id, Type: gen.AutomationTriggerType(trigger.Kind)}
	if schedule := trigger.Schedule; schedule != nil {
		at := fmt.Sprintf("%02d:%02d", schedule.MinuteOfDay/60, schedule.MinuteOfDay%60)
		days := make([]gen.AutomationTriggerDays, 0, 7)
		for _, entry := range dayOrder {
			if schedule.Days.Has(entry.day) {
				days = append(days, entry.name)
			}
		}
		view.At = &at
		view.Days = &days
	}
	if trigger.Kind == TriggerInterval {
		seconds := int(trigger.Interval / time.Second)
		view.Seconds = &seconds
	}
	return view
}

func stepViews(steps []Step) []gen.AutomationStep {
	views := make([]gen.AutomationStep, 0, len(steps))
	for _, step := range steps {
		view := gen.AutomationStep{Type: gen.AutomationStepType(step.Kind)}
		switch step.Kind {
		case StepSet:
			sensorID, property, value := step.SensorID, step.Property, step.Value
			view.SensorId, view.Property, view.Value = &sensorID, &property, &value
		case StepWait:
			seconds := step.Seconds
			view.Seconds = &seconds
		}
		views = append(views, view)
	}
	return views
}

func runView(run Run) gen.AutomationRun {
	view := gen.AutomationRun{
		Id:           run.ID,
		AutomationId: run.AutomationID,
		TriggerId:    run.TriggerID,
		TriggerKind:  gen.AutomationRunTriggerKind(run.TriggerKind),
		Status:       gen.AutomationRunStatus(run.Status),
		CurrentStep:  run.CurrentStep,
		Steps:        stepViews(run.Steps),
		StepOutcomes: make([]gen.AutomationRunStep, 0, len(run.StepOutcomes)),
		StartedAt:    run.StartedAt.UTC(),
		Error:        run.Error,
	}
	view.FinishedAt = utc(run.FinishedAt)
	view.ResumeAt = utc(run.ResumeAt)
	view.DueAt = utc(run.DueAt)
	if run.PastGrace != nil {
		seconds := int(run.PastGrace.Seconds())
		view.PastGraceSeconds = &seconds
	}
	for _, step := range run.StepOutcomes {
		outcome := gen.AutomationRunStep{
			Position:  step.Position,
			Kind:      gen.AutomationRunStepKind(step.Kind),
			Outcome:   gen.AutomationRunStepOutcome(step.Outcome),
			CommandId: step.CommandID,
			StartedAt: step.StartedAt.UTC(),
		}
		outcome.FinishedAt = utc(step.FinishedAt)
		view.StepOutcomes = append(view.StepOutcomes, outcome)
	}
	return view
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	converted := t.UTC()
	return &converted
}
