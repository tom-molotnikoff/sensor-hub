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

func (f *fixture) createInMode(t *testing.T, mode gen.AutomationMode, steps ...gen.AutomationStep) gen.Automation {
	t.Helper()
	created, err := f.service.Create(context.Background(), gen.AutomationInput{
		Name: "Evening lights", Mode: &mode, Triggers: []gen.AutomationTrigger{scheduleTrigger(laterToday())}, Steps: steps,
	})
	require.NoError(t, err)
	return created
}

func (f *fixture) run(t *testing.T, automationID int, runID int) gen.AutomationRun {
	t.Helper()
	runs, err := f.service.Runs(context.Background(), automationID)
	require.NoError(t, err)
	for _, run := range runs {
		if run.Id == runID {
			return run
		}
	}
	require.FailNow(t, "no such run", "run %d", runID)
	return gen.AutomationRun{}
}

func (f *fixture) user(t *testing.T, username string) int {
	t.Helper()
	result, err := f.db.Writer.Exec("INSERT INTO users (username, password_hash) VALUES (?, 'x')", username)
	require.NoError(t, err)
	id, err := result.LastInsertId()
	require.NoError(t, err)
	return int(id)
}

// noCommandsBeyond checks until a moment rather than for a fixed spell, so it
// can outlast a resume time.
func (f *fixture) noCommandsBeyond(t *testing.T, limit int, until time.Time, message string) {
	t.Helper()
	for ; time.Now().Before(until); time.Sleep(10 * time.Millisecond) {
		if f.commandsInHistory(t) > limit {
			assert.Fail(t, message)
			return
		}
	}
}

func TestMode_DefaultsToSingleAndAnUpdateWithoutOneKeepsIt(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, setStep(f.lampID, "state", "ON"))
	assert.Equal(t, gen.AutomationModeSingle, created.Mode)

	restart := gen.AutomationModeRestart
	input := gen.AutomationInput{Name: "Evening lights", Mode: &restart, Triggers: []gen.AutomationTrigger{scheduleTrigger(laterToday())},
		Steps: []gen.AutomationStep{setStep(f.lampID, "state", "ON")}}
	updated, err := f.service.Update(ctx, created.Id, input)
	require.NoError(t, err)
	assert.Equal(t, gen.AutomationModeRestart, updated.Mode)

	input.Mode = nil
	kept, err := f.service.Update(ctx, created.Id, input)
	require.NoError(t, err)
	assert.Equal(t, gen.AutomationModeRestart, kept.Mode)

	input.Mode = ptr(gen.AutomationMode("queued"))
	_, err = f.service.Update(ctx, created.Id, input)
	var invalid *automation.ValidationError
	require.ErrorAs(t, err, &invalid)
	assert.Contains(t, invalid.Message, "mode")
}

func TestMode_SingleRecordsASkippedRunAndTheActiveRunCarriesOn(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, lampTimer(f, 1)...)
	waiting := f.startWaiting(t, created)

	f.fire(created)

	skipped := f.latestRun(t, created.Id, gen.AutomationRunStatusSkipped)
	assert.NotEqual(t, waiting.Id, skipped.Id)
	assert.Empty(t, skipped.StepOutcomes)
	assert.NotNil(t, skipped.FinishedAt)
	f.commands.await(t, 2).outcome <- "acknowledged"
	require.Eventually(t, func() bool {
		return f.run(t, created.Id, waiting.Id).Status == gen.AutomationRunStatusSucceeded
	}, 5*time.Second, 10*time.Millisecond, "the active run did not carry on")
	assert.Equal(t, 3, f.commands.count())
}

func TestMode_RestartCancelsTheActiveRunAndStartsOverFromStepOne(t *testing.T) {
	f := newFixture(t)
	created := f.createInMode(t, gen.AutomationModeRestart, lampTimer(f, 4*60*60)...)
	waiting := f.startWaiting(t, created)

	f.fire(created)

	first := f.commands.await(t, 2)
	assert.Equal(t, "state", first.property)
	assert.Equal(t, "ON", first.value)
	assert.NotEqual(t, waiting.Id, first.runID)
	cancelled := f.run(t, created.Id, waiting.Id)
	assert.Equal(t, gen.AutomationRunStatusCancelled, cancelled.Status)
	assert.NotNil(t, cancelled.FinishedAt)
	assert.Nil(t, cancelled.ResumeAt)
	require.Len(t, cancelled.StepOutcomes, 3)
	assert.Equal(t, gen.AutomationRunStepOutcomeCancelled, cancelled.StepOutcomes[2].Outcome)
	assert.False(t, f.service.ResumeScheduled(waiting.Id))
}

func TestStartup_ATriggerCaughtUpInRestartModeCancelsTheRunThatWouldResume(t *testing.T) {
	f := newFixture(t)
	created := f.createInMode(t, gen.AutomationModeRestart, lampTimer(f, 4*60*60)...)
	waiting := f.startWaiting(t, created)

	f.stop()
	f.exec(t, "UPDATE automation_runs SET resume_at = ? WHERE id = ?", time.Now().UTC().Add(-time.Minute), waiting.Id)
	f.exec(t, "UPDATE automation_triggers SET next_due_at = ? WHERE id = ?", time.Now().UTC().Add(-time.Minute), *created.Triggers[0].Id)
	f.start(t)

	restarted := f.commands.await(t, 2)
	assert.Equal(t, "ON", restarted.value, "the cancelled run resumed instead of a new one starting")
	assert.NotEqual(t, waiting.Id, restarted.runID)
	assert.Equal(t, gen.AutomationRunStatusCancelled, f.run(t, created.Id, waiting.Id).Status)
}

func TestEdit_AnActiveRunFinishesWithTheStepsItStartedWith(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, lampTimer(f, 1)...)
	waiting := f.startWaiting(t, created)

	edited, err := f.service.Update(ctx, created.Id, gen.AutomationInput{
		Name: "Evening lights", Triggers: []gen.AutomationTrigger{scheduleTrigger(laterToday())},
		Steps: []gen.AutomationStep{setStep(f.lampID, "brightness", "50")},
	})
	require.NoError(t, err)

	off := f.commands.await(t, 2)
	assert.Equal(t, "OFF", off.value)
	off.outcome <- "acknowledged"
	require.Eventually(t, func() bool {
		return f.run(t, created.Id, waiting.Id).Status == gen.AutomationRunStatusSucceeded
	}, 5*time.Second, 10*time.Millisecond)

	f.fire(edited)
	next := f.commands.await(t, 3)
	assert.Equal(t, "brightness", next.property)
	assert.Equal(t, "50", next.value)
}

func TestSwitchOff_TheActiveRunFinishesAndNoNewTriggerFires(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, lampTimer(f, 1)...)
	waiting := f.startWaiting(t, created)

	_, err := f.service.SetEnabled(context.Background(), created.Id, false)
	require.NoError(t, err)
	f.fire(created)

	f.commands.await(t, 2).outcome <- "acknowledged"
	run := f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)
	assert.Equal(t, waiting.Id, run.Id, "a trigger started or skipped a run while the automation was off")
	f.neverMoreCommandsThan(t, 3, "a trigger fired while the automation was off")
}

func TestCancel_AWaitingRunEndsAndItsResumeTimeIsDropped(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, lampTimer(f, 1)...)
	waiting := f.startWaiting(t, created)
	require.True(t, f.service.ResumeScheduled(waiting.Id))

	cancelled, err := f.service.CancelRun(context.Background(), created.Id, waiting.Id)

	require.NoError(t, err)
	assert.Equal(t, gen.AutomationRunStatusCancelled, cancelled.Status)
	assert.Nil(t, cancelled.ResumeAt)
	assert.False(t, f.service.ResumeScheduled(waiting.Id))
	f.noCommandsBeyond(t, 2, waiting.ResumeAt.Add(500*time.Millisecond), "the cancelled run carried on after its wait")
	assert.Empty(t, f.notifier.all())
}

func TestCancel_ACommandInFlightCarriesOnButNoFurtherStepRuns(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())},
		setStep(f.lampID, "state", "ON"),
		setStep(f.lampID, "brightness", "150"),
	)
	f.fire(created)
	inFlight := f.commands.await(t, 0)

	_, err := f.service.CancelRun(context.Background(), created.Id, inFlight.runID)
	require.NoError(t, err)
	inFlight.outcome <- "timed_out"

	f.neverMoreCommandsThan(t, 1, "a step ran after the run was cancelled")
	run := f.run(t, created.Id, inFlight.runID)
	assert.Equal(t, gen.AutomationRunStatusCancelled, run.Status)
	require.Len(t, run.StepOutcomes, 1)
	assert.Equal(t, gen.AutomationRunStepOutcomeCancelled, run.StepOutcomes[0].Outcome)
	assert.Eventually(t, func() bool {
		return f.run(t, created.Id, inFlight.runID).StepOutcomes[0].CommandId != nil
	}, 5*time.Second, 10*time.Millisecond, "the cancelled step lost the command it sent")
	assert.Empty(t, f.notifier.all(), "a cancelled run notified a failure")
}

func TestCancel_RefusesARunThatHasEndedOrIsNotTheAutomations(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, setStep(f.lampID, "state", "ON"))
	f.fire(created)
	f.commands.await(t, 0).outcome <- "acknowledged"
	finished := f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)

	_, err := f.service.CancelRun(ctx, created.Id, finished.Id)
	assert.ErrorIs(t, err, automation.ErrRunNotActive)
	_, err = f.service.CancelRun(ctx, created.Id+1, finished.Id)
	assert.ErrorIs(t, err, automation.ErrRunNotFound)
}

func TestRunNow_StartsAManualRunAsTheUserEvenWhenOff(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	userID := f.user(t, "alice")
	created, err := f.service.Create(ctx, gen.AutomationInput{Name: "Evening lights", Enabled: ptr(false),
		Triggers: []gen.AutomationTrigger{scheduleTrigger(laterToday())}, Steps: []gen.AutomationStep{setStep(f.lampID, "state", "ON")}})
	require.NoError(t, err)

	started, err := f.service.RunNow(ctx, created.Id, userID)

	require.NoError(t, err)
	assert.Equal(t, gen.AutomationRunStatusRunning, started.Status)
	assert.Equal(t, gen.AutomationRunTriggerKindManual, started.TriggerKind)
	assert.Nil(t, started.TriggerId)
	assert.Equal(t, &gen.CommandHistoryUser{Id: userID, Username: "alice"}, started.InitiatedBy)
	sent := f.commands.await(t, 0)
	assert.Equal(t, started.Id, sent.runID)
	sent.outcome <- "acknowledged"
	f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)
}

func TestRunNow_ObeysTheMode(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	userID := f.user(t, "alice")
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, lampTimer(f, 4*60*60)...)
	f.startWaiting(t, created)

	skipped, err := f.service.RunNow(ctx, created.Id, userID)

	require.NoError(t, err)
	assert.Equal(t, gen.AutomationRunStatusSkipped, skipped.Status)
	assert.Equal(t, gen.AutomationRunTriggerKindManual, skipped.TriggerKind)
	_, err = f.service.RunNow(ctx, created.Id+1, userID)
	assert.ErrorIs(t, err, automation.ErrNotFound)
}

func TestDelete_IsRefusedUntilTheActiveRunIsCancelled(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, lampTimer(f, 4*60*60)...)
	waiting := f.startWaiting(t, created)

	assert.ErrorIs(t, f.service.Delete(ctx, created.Id), automation.ErrActiveRun)
	_, err := f.service.Get(ctx, created.Id)
	require.NoError(t, err, "a refused delete deleted the automation")

	_, err = f.service.CancelRun(ctx, created.Id, waiting.Id)
	require.NoError(t, err)
	assert.NoError(t, f.service.Delete(ctx, created.Id))
}
