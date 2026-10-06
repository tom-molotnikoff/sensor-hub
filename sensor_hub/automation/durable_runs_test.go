package automation_test

import (
	"context"
	"testing"
	"time"

	"example/sensorHub/automation"
	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lampTimer(f *fixture, wait int) []gen.AutomationStep {
	return []gen.AutomationStep{
		setStep(f.lampID, "state", "ON"),
		setStep(f.lampID, "brightness", "150"),
		waitStep(wait),
		setStep(f.lampID, "state", "OFF"),
	}
}

func (f *fixture) startWaiting(t *testing.T, created gen.Automation) gen.AutomationRun {
	t.Helper()
	f.fire(created)
	f.commands.await(t, 0).outcome <- "acknowledged"
	f.commands.await(t, 1).outcome <- "acknowledged"
	return f.latestRun(t, created.Id, gen.AutomationRunStatusWaiting)
}

func (f *fixture) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	_, err := f.db.Writer.Exec(query, args...)
	require.NoError(t, err)
}

func TestWait_SendsTheNextStepOnceTheWaitIsOver(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, lampTimer(f, 1)...)

	waiting := f.startWaiting(t, created)

	assert.Equal(t, 3, waiting.CurrentStep)
	require.NotNil(t, waiting.ResumeAt)
	require.Len(t, waiting.StepOutcomes, 3)
	waitStarted := waiting.StepOutcomes[2].StartedAt
	assert.WithinDuration(t, waitStarted.Add(time.Second), *waiting.ResumeAt, time.Millisecond)
	view, err := f.service.Get(context.Background(), created.Id)
	require.NoError(t, err)
	assert.Equal(t, gen.AutomationStatusRunning, view.Status)

	off := f.commands.await(t, 2)
	assert.Equal(t, "OFF", off.value)
	assert.False(t, off.at.Before(waitStarted.Add(time.Second)), "the step after the wait went out %s after the wait started", off.at.Sub(waitStarted))
	off.outcome <- "acknowledged"

	run := f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)
	require.Len(t, run.StepOutcomes, 4)
	assert.Equal(t, gen.AutomationRunStepKindWait, run.StepOutcomes[2].Kind)
	assert.Equal(t, gen.AutomationRunStepOutcomeSucceeded, run.StepOutcomes[2].Outcome)
	assert.Nil(t, run.ResumeAt)
}

func TestRestart_AWaitingRunCarriesOnAtItsResumeTime(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, lampTimer(f, 4*60*60)...)
	waiting := f.startWaiting(t, created)

	f.stop()
	resumeAt := time.Now().UTC().Add(time.Second)
	f.exec(t, "UPDATE automation_runs SET resume_at = ? WHERE id = ?", resumeAt, waiting.Id)
	f.start(t)

	off := f.commands.await(t, 2)
	assert.Equal(t, "OFF", off.value)
	assert.False(t, off.at.Before(resumeAt), "the run carried on %s before its resume time", resumeAt.Sub(off.at))
	off.outcome <- "acknowledged"
	f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)
}

func TestRestart_AWaitingRunWhoseResumeTimePassedResumesOnStartup(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, lampTimer(f, 4*60*60)...)
	waiting := f.startWaiting(t, created)

	f.stop()
	f.exec(t, "UPDATE automation_runs SET resume_at = ? WHERE id = ?", time.Now().UTC().Add(-3*24*time.Hour), waiting.Id)
	f.start(t)

	f.commands.await(t, 2).outcome <- "acknowledged"
	f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)
}

func TestRestart_AnInterruptedSetStepTakesTheOutcomeOfItsRecordedCommand(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())},
		setStep(f.lampID, "state", "ON"),
		setStep(f.lampID, "brightness", "150"),
	)
	f.fire(created)
	interrupted := f.commands.await(t, 0)

	f.restart(t)
	f.commands.awaitedOutcome(t, interrupted.id) <- "acknowledged"

	f.commands.await(t, 1).outcome <- "acknowledged"
	run := f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)
	assert.Equal(t, 2, f.commandsInHistory(t), "the interrupted step was sent twice")
	require.Len(t, run.StepOutcomes, 2)
	assert.Equal(t, &interrupted.id, run.StepOutcomes[0].CommandId)
	assert.Equal(t, gen.AutomationRunStepOutcomeSucceeded, run.StepOutcomes[0].Outcome)
}

func TestRestart_AnInterruptedSetStepWithNoRecordedCommandIsSent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, setStep(f.lampID, "state", "ON"))
	saved, err := f.store.GetAutomation(ctx, created.Id)
	require.NoError(t, err)
	runID, err := f.store.CreateRun(ctx, automation.Run{
		AutomationID: created.Id,
		TriggerKind:  automation.TriggerSchedule,
		Status:       automation.RunRunning,
		Steps:        saved.Steps,
		StartedAt:    time.Now().UTC(),
	})
	require.NoError(t, err)
	_, err = f.store.StartRunStep(ctx, runID, 1, automation.StepSet, time.Now().UTC())
	require.NoError(t, err)

	f.restart(t)

	sent := f.commands.await(t, 0)
	assert.Equal(t, runID, sent.runID)
	sent.outcome <- "acknowledged"
	run := f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)
	require.Len(t, run.StepOutcomes, 1)
	assert.Equal(t, &sent.id, run.StepOutcomes[0].CommandId)
}

func TestStartup_ATriggerThatCameDueWithinTheGraceWindowStartsARun(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, setStep(f.lampID, "state", "ON"))

	f.stop()
	f.exec(t, "UPDATE automation_triggers SET next_due_at = ? WHERE id = ?", time.Now().UTC().Add(-9*time.Minute), *created.Triggers[0].Id)
	f.start(t)

	f.commands.await(t, 0).outcome <- "acknowledged"
	run := f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)
	assert.Equal(t, created.Triggers[0].Id, run.TriggerId)
}

func TestStartup_ATriggerLaterThanTheGraceWindowIsRecordedAsMissed(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, setStep(f.lampID, "state", "ON"))

	f.stop()
	due := time.Now().UTC().Add(-130 * time.Minute).Truncate(time.Second)
	f.exec(t, "UPDATE automation_triggers SET next_due_at = ? WHERE id = ?", due, *created.Triggers[0].Id)
	f.start(t)

	run := f.latestRun(t, created.Id, gen.AutomationRunStatusMissed)
	assert.Equal(t, created.Triggers[0].Id, run.TriggerId)
	require.NotNil(t, run.DueAt)
	assert.True(t, due.Equal(*run.DueAt), "due %s, want %s", run.DueAt, due)
	require.NotNil(t, run.PastGraceSeconds)
	assert.InDelta(t, 120*60, *run.PastGraceSeconds, 5)
	assert.Empty(t, run.StepOutcomes)
	runs, err := f.service.Runs(context.Background(), created.Id)
	require.NoError(t, err)
	assert.Len(t, runs, 1)
	assert.Never(t, func() bool { return f.commands.count() > 0 }, 200*time.Millisecond, 10*time.Millisecond, "a missed trigger ran")
}
