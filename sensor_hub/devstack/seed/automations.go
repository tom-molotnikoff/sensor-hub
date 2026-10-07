package main

import (
	"context"
	"fmt"
	"time"

	appProps "example/sensorHub/application_properties"
	"example/sensorHub/automation"
	database "example/sensorHub/db"
	"example/sensorHub/drivers"
)

const (
	// The hub's defaults for actuator.command.timeout_seconds and
	// automation.missed.grace.minutes.
	commandTimeout = 10 * time.Second
	missedGrace    = 10 * time.Minute

	acknowledgedAfter = time.Second
)

type seededAutomation struct {
	name    string
	enabled bool
	mode    automation.Mode
	trigger seededTrigger
	steps   []seededStep
	history func(runHistory, context.Context, automation.Automation) error
}

type seededTrigger struct {
	kind        automation.TriggerKind
	at          int
	days        automation.Weekdays
	interval    time.Duration
	device      string
	measurement string
	operator    automation.Operator
	threshold   float64
	margin      float64
	value       string
	hold        time.Duration
}

type seededStep struct {
	device   string
	property string
	value    string
	wait     time.Duration
}

var (
	everyDay = automation.WeekdaysOf(time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday)
	weekdays = automation.WeekdaysOf(time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday)
)

func at(hour, minute int, days automation.Weekdays) seededTrigger {
	return seededTrigger{kind: automation.TriggerSchedule, at: hour*60 + minute, days: days}
}

func switchOffice(value string) seededStep {
	return seededStep{device: "office-plug", property: "state", value: value}
}

func wait(d time.Duration) seededStep {
	return seededStep{wait: d}
}

var seededAutomations = []seededAutomation{
	{name: "Office plug off at night", enabled: true, mode: automation.ModeSingle,
		trigger: at(23, 30, everyDay),
		steps:   []seededStep{switchOffice("OFF")},
		history: runHistory.nightOff},
	{name: "Office plug on for two hours", enabled: true, mode: automation.ModeSingle,
		trigger: at(18, 0, everyDay),
		steps:   []seededStep{switchOffice("ON"), wait(2 * time.Hour), switchOffice("OFF")},
		history: runHistory.twoHoursOn},
	{name: "Cycle the office plug", enabled: true, mode: automation.ModeSingle,
		trigger: seededTrigger{kind: automation.TriggerInterval, interval: 6 * time.Hour},
		steps:   []seededStep{switchOffice("ON"), wait(15 * time.Minute), switchOffice("OFF")},
		history: runHistory.cycle},
	{name: "Heating on when the bedroom is cold", enabled: true, mode: automation.ModeSingle,
		trigger: seededTrigger{kind: automation.TriggerReading, device: "bedroom-sensor", measurement: "temperature",
			operator: automation.FallsBelow, threshold: 19, margin: 0.3},
		steps:   []seededStep{switchOffice("ON")},
		history: runHistory.heatingOn},
	{name: "Heating off when the bedroom is warm", enabled: true, mode: automation.ModeSingle,
		trigger: seededTrigger{kind: automation.TriggerReading, device: "bedroom-sensor", measurement: "temperature",
			operator: automation.RisesAbove, threshold: 21, margin: 0.3},
		steps:   []seededStep{switchOffice("OFF")},
		history: runHistory.heatingOff},
	// A Zigbee2MQTT contact reads false when open.
	{name: "Office plug on when the front door opens", enabled: true, mode: automation.ModeRestart,
		trigger: seededTrigger{kind: automation.TriggerReading, device: "front-door", measurement: "contact",
			operator: automation.Becomes, value: "false", hold: 30 * time.Second},
		steps:   []seededStep{switchOffice("ON"), wait(10 * time.Minute), switchOffice("OFF")},
		history: runHistory.frontDoorOpened},
	{name: "Office plug on on weekday mornings", enabled: false, mode: automation.ModeSingle,
		trigger: at(7, 0, weekdays),
		steps:   []seededStep{switchOffice("ON")}},
}

func (s *seeder) createAutomations(ctx context.Context, sensorIDs map[string]int, adminID int, now time.Time) error {
	typeIDs, err := s.measurementTypeIDs(ctx)
	if err != nil {
		return err
	}
	existing, err := s.automations.ListAutomations(ctx)
	if err != nil {
		return err
	}
	names := make(map[string]bool, len(existing))
	for _, a := range existing {
		names[a.Name] = true
	}
	history := runHistory{seeder: s, admin: automation.User{ID: adminID, Username: adminUsername}, zone: hubZone(), now: now}
	for _, seeded := range seededAutomations {
		if names[seeded.name] {
			s.logger.Info("automation exists", "name", seeded.name)
			continue
		}
		a, err := s.automations.CreateAutomation(ctx, seeded.automation(sensorIDs, typeIDs))
		if err != nil {
			return fmt.Errorf("automation %s: %w", seeded.name, err)
		}
		s.logger.Info("created automation", "name", seeded.name)
		if seeded.history == nil {
			continue
		}
		if err := seeded.history(history, ctx, a); err != nil {
			return fmt.Errorf("runs of automation %s: %w", seeded.name, err)
		}
	}
	return nil
}

func (seeded seededAutomation) automation(sensorIDs map[string]int, typeIDs map[string]int) automation.Automation {
	t := seeded.trigger
	trigger := automation.Trigger{Kind: t.kind}
	switch t.kind {
	case automation.TriggerSchedule:
		trigger.Schedule = &automation.Schedule{MinuteOfDay: t.at, Days: t.days}
	case automation.TriggerInterval:
		trigger.Interval = t.interval
	case automation.TriggerReading:
		trigger.Reading = &automation.ReadingCondition{
			Series:            automation.Series{SensorID: sensorIDs[t.device], MeasurementType: t.measurement},
			MeasurementTypeID: typeIDs[t.measurement],
			Operator:          t.operator,
			Threshold:         t.threshold,
			Margin:            t.margin,
			Value:             t.value,
			Hold:              t.hold,
		}
	}
	a := automation.Automation{Name: seeded.name, Enabled: seeded.enabled, Mode: seeded.mode, Triggers: []automation.Trigger{trigger}}
	for _, step := range seeded.steps {
		if step.wait > 0 {
			a.Steps = append(a.Steps, automation.Step{Kind: automation.StepWait, Seconds: int(step.wait.Seconds())})
			continue
		}
		a.Steps = append(a.Steps, automation.Step{Kind: automation.StepSet, SensorID: sensorIDs[step.device], Property: step.property, Value: step.value})
	}
	return a
}

func hubZone() *time.Location {
	if cfg := appProps.AppConfig(); cfg != nil {
		return cfg.HubLocation()
	}
	return time.UTC
}

// runHistory records a week of finished runs through the repository calls the
// engine makes, so they have the shape real runs have.
type runHistory struct {
	*seeder
	admin automation.User
	zone  *time.Location
	now   time.Time
}

func (h runHistory) nightOff(ctx context.Context, a automation.Automation) error {
	for day := 7; day >= 1; day-- {
		due := h.daysAgo(day, 23, 30)
		if day == 4 {
			if err := h.missed(ctx, a, due, due.Add(missedGrace+2*time.Hour)); err != nil {
				return err
			}
			continue
		}
		// The last run failing gives the list its "last run failed" flag
		// until the next run at 23:30. Automations on live readings run as
		// soon as the hub starts, which would clear it.
		p := playback{}
		if day == 1 {
			p.failStep = 1
		}
		if err := h.play(ctx, a, due, p); err != nil {
			return err
		}
	}
	return nil
}

func (h runHistory) twoHoursOn(ctx context.Context, a automation.Automation) error {
	for day := 7; day >= 1; day-- {
		started := h.daysAgo(day, 18, 0)
		if day != 2 {
			if err := h.play(ctx, a, started, playback{}); err != nil {
				return err
			}
			continue
		}
		// Run now pressed while the run waits, then Cancel run.
		if err := h.play(ctx, a, started, playback{cancelAt: started.Add(75 * time.Minute)}); err != nil {
			return err
		}
		if err := h.skipped(ctx, a, started.Add(40*time.Minute)); err != nil {
			return err
		}
	}
	return nil
}

func (h runHistory) cycle(ctx context.Context, a automation.Automation) error {
	for n := 8; n >= 1; n-- {
		if err := h.play(ctx, a, h.now.Add(-time.Duration(n)*a.Triggers[0].Interval), playback{}); err != nil {
			return err
		}
	}
	return nil
}

func (h runHistory) heatingOn(ctx context.Context, a automation.Automation) error {
	for _, started := range []time.Time{h.daysAgo(6, 6, 12), h.daysAgo(3, 5, 48), h.daysAgo(1, 6, 31)} {
		if err := h.play(ctx, a, started, playback{}); err != nil {
			return err
		}
	}
	return nil
}

func (h runHistory) heatingOff(ctx context.Context, a automation.Automation) error {
	for _, started := range []time.Time{h.daysAgo(6, 9, 40), h.daysAgo(3, 10, 5), h.daysAgo(2, 9, 12)} {
		if err := h.play(ctx, a, started, playback{}); err != nil {
			return err
		}
	}
	return nil
}

func (h runHistory) frontDoorOpened(ctx context.Context, a automation.Automation) error {
	if err := h.play(ctx, a, h.daysAgo(5, 8, 10), playback{}); err != nil {
		return err
	}
	// The door opened again while the run waited, and restart mode started
	// over.
	first := h.daysAgo(2, 17, 20)
	again := first.Add(4 * time.Minute)
	if err := h.play(ctx, a, first, playback{cancelAt: again}); err != nil {
		return err
	}
	return h.play(ctx, a, again, playback{})
}

func (h runHistory) daysAgo(days, hour, minute int) time.Time {
	local := h.now.In(h.zone)
	return time.Date(local.Year(), local.Month(), local.Day()-days, hour, minute, 0, 0, h.zone).UTC()
}

// A playback stops at failStep, a position counted from 1, with the device not
// acknowledging. A wait still going at cancelAt is cancelled.
type playback struct {
	failStep int
	cancelAt time.Time
}

func (h runHistory) play(ctx context.Context, a automation.Automation, started time.Time, p playback) error {
	trigger := a.Triggers[0]
	runID, err := h.automations.CreateRun(ctx, automation.Run{AutomationID: a.ID, TriggerID: &trigger.ID, TriggerKind: trigger.Kind,
		Status: automation.RunRunning, Steps: a.Steps, StartedAt: started})
	if err != nil {
		return err
	}
	clock := started
	for i, step := range a.Steps {
		position := i + 1
		if step.Kind == automation.StepWait {
			resumeAt := clock.Add(step.Wait())
			if err := h.automations.WaitRun(ctx, runID, position, clock, resumeAt); err != nil {
				return err
			}
			if !p.cancelAt.IsZero() && p.cancelAt.Before(resumeAt) {
				_, err := h.automations.CancelRun(ctx, a.ID, runID, p.cancelAt)
				return err
			}
			if _, err := h.automations.ResumeRun(ctx, runID, resumeAt); err != nil {
				return err
			}
			clock = resumeAt
			continue
		}
		failed := position == p.failStep
		finished, err := h.set(ctx, runID, position, step, clock, failed)
		if err != nil {
			return err
		}
		clock = finished
		if failed {
			sensor, err := h.sensors.ServiceGetSensorById(ctx, step.SensorID)
			if err != nil {
				return err
			}
			message := fmt.Sprintf("step %d (set %s %s to %s) failed: %s did not acknowledge the command within %s",
				position, sensor.Name, step.Property, step.Value, sensor.Name, commandTimeout)
			return h.automations.FinishRun(ctx, runID, automation.RunFailed, &message, clock)
		}
	}
	return h.automations.FinishRun(ctx, runID, automation.RunSucceeded, nil, clock)
}

func (h runHistory) set(ctx context.Context, runID int, position int, step automation.Step, sentAt time.Time, unacknowledged bool) (time.Time, error) {
	stepID, err := h.automations.StartRunStep(ctx, runID, position, automation.StepSet, sentAt)
	if err != nil {
		return time.Time{}, err
	}
	sensor, err := h.sensors.ServiceGetSensorById(ctx, step.SensorID)
	if err != nil {
		return time.Time{}, err
	}
	driver, ok := drivers.GetCommandDriver(sensor.SensorDriver)
	if !ok {
		return time.Time{}, fmt.Errorf("%s has no command driver", sensor.Name)
	}
	topic, payload, err := driver.BuildCommand(*sensor, step.Property, step.Value)
	if err != nil {
		return time.Time{}, err
	}
	commandID, err := h.commands.AddSentCommand(ctx, database.NewCommand{SensorID: sensor.Id, AutomationRunID: &runID,
		Property: step.Property, Value: step.Value, MQTTTopic: topic, MQTTPayload: string(payload),
		TimeoutSeconds: int(commandTimeout.Seconds()), SentAt: sentAt})
	if err != nil {
		return time.Time{}, err
	}
	finished, outcome := sentAt.Add(acknowledgedAfter), automation.StepSucceeded
	if unacknowledged {
		finished, outcome = sentAt.Add(commandTimeout), automation.StepFailed
		_, err = h.commands.MarkTimedOut(ctx, commandID)
	} else {
		_, err = h.commands.MarkAcknowledged(ctx, commandID, step.Value, finished)
	}
	if err != nil {
		return time.Time{}, err
	}
	return finished, h.automations.FinishRunStep(ctx, stepID, outcome, &commandID, finished)
}

// A missed run is recorded when the hub starts again, so it starts and
// finishes then.
func (h runHistory) missed(ctx context.Context, a automation.Automation, due time.Time, recorded time.Time) error {
	trigger := a.Triggers[0]
	pastGrace := recorded.Sub(due) - missedGrace
	_, err := h.automations.CreateRun(ctx, automation.Run{AutomationID: a.ID, TriggerID: &trigger.ID, TriggerKind: trigger.Kind,
		Status: automation.RunMissed, Steps: a.Steps, StartedAt: recorded, FinishedAt: &recorded, DueAt: &due, PastGrace: &pastGrace})
	return err
}

func (h runHistory) skipped(ctx context.Context, a automation.Automation, at time.Time) error {
	_, err := h.automations.CreateRun(ctx, automation.Run{AutomationID: a.ID, TriggerKind: automation.TriggerManual, InitiatedBy: &h.admin,
		Status: automation.RunSkipped, Steps: a.Steps, StartedAt: at, FinishedAt: &at})
	return err
}
