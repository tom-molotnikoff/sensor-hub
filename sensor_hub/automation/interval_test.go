package automation_test

import (
	"context"
	"testing"
	"time"

	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (f *fixture) nextFireAt(t *testing.T, automationID int) time.Time {
	t.Helper()
	view, err := f.service.Get(context.Background(), automationID)
	require.NoError(t, err)
	require.NotNil(t, view.NextFireAt)
	return *view.NextFireAt
}

func TestInterval_FiresEveryIntervalCountedFromTheSave(t *testing.T) {
	f := newFixture(t)
	saved := time.Now().UTC()
	created := f.create(t, []gen.AutomationTrigger{intervalTrigger(30 * 60)}, setStep(f.lampID, "state", "ON"))
	require.NotNil(t, created.NextFireAt)
	first := *created.NextFireAt
	assert.WithinDuration(t, saved.Add(30*time.Minute), first, time.Second)

	f.service.Fire(*created.Triggers[0].Id, first)
	f.commands.await(t, 0).outcome <- "acknowledged"
	run := f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)

	assert.Equal(t, gen.AutomationRunTriggerKindInterval, run.TriggerKind)
	assert.WithinDuration(t, first.Add(30*time.Minute), f.nextFireAt(t, created.Id), 0)
}

func TestInterval_SwitchingOnStartsCountingFromThen(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	created := f.create(t, []gen.AutomationTrigger{intervalTrigger(30 * 60)}, setStep(f.lampID, "state", "ON"))
	f.service.Fire(*created.Triggers[0].Id, *created.NextFireAt)
	f.commands.await(t, 0).outcome <- "acknowledged"
	f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)

	_, err := f.service.SetEnabled(ctx, created.Id, false)
	require.NoError(t, err)
	enabled := time.Now().UTC()
	on, err := f.service.SetEnabled(ctx, created.Id, true)
	require.NoError(t, err)

	require.NotNil(t, on.NextFireAt)
	assert.WithinDuration(t, enabled.Add(30*time.Minute), *on.NextFireAt, time.Second)
}

func TestNextFireAt_IsTheEarliestOfTheScheduleAndIntervalTriggers(t *testing.T) {
	f := newFixture(t)
	now := time.Now().UTC()

	sooner := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday()), intervalTrigger(30 * 60)}, setStep(f.lampID, "state", "ON"))
	later := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday()), intervalTrigger(24 * 60 * 60)}, setStep(f.lampID, "state", "ON"))

	require.NotNil(t, sooner.NextFireAt)
	assert.WithinDuration(t, now.Add(30*time.Minute), *sooner.NextFireAt, time.Second)
	require.NotNil(t, later.NextFireAt)
	assert.WithinDuration(t, now.Add(12*time.Hour), *later.NextFireAt, time.Minute)
}

func TestRestart_AnIntervalKeepsItsPhase(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{intervalTrigger(30 * 60)}, setStep(f.lampID, "state", "ON"))

	f.stop()
	due := time.Now().UTC().Add(7 * time.Minute).Truncate(time.Second)
	f.exec(t, "UPDATE automation_triggers SET next_due_at = ? WHERE id = ?", due, *created.Triggers[0].Id)
	f.start(t)

	assert.WithinDuration(t, due, f.nextFireAt(t, created.Id), 0)
}

func TestStartup_AnIntervalMissedSeveralTimesStartsOneRun(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{intervalTrigger(60)}, setStep(f.lampID, "state", "ON"))

	f.stop()
	due := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	f.exec(t, "UPDATE automation_triggers SET next_due_at = ? WHERE id = ?", due, *created.Triggers[0].Id)
	f.start(t)

	f.commands.await(t, 0).outcome <- "acknowledged"
	f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)
	f.neverMoreCommandsThan(t, 1, "the missed times each started a run")
	runs, err := f.service.Runs(context.Background(), created.Id)
	require.NoError(t, err)
	assert.Len(t, runs, 1)
	assert.WithinDuration(t, due.Add(121*time.Minute), f.nextFireAt(t, created.Id), 0, "the interval keeps its phase")
}

func TestStartup_AnIntervalMissedSeveralTimesPastTheGraceWindowIsRecordedAsMissedOnce(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{intervalTrigger(60 * 60)}, setStep(f.lampID, "state", "ON"))

	f.stop()
	due := time.Now().UTC().Add(-210 * time.Minute).Truncate(time.Second)
	f.exec(t, "UPDATE automation_triggers SET next_due_at = ? WHERE id = ?", due, *created.Triggers[0].Id)
	f.start(t)

	run := f.latestRun(t, created.Id, gen.AutomationRunStatusMissed)
	require.NotNil(t, run.DueAt)
	assert.True(t, due.Add(3*time.Hour).Equal(*run.DueAt), "due %s, want the latest missed time %s", run.DueAt, due.Add(3*time.Hour))
	runs, err := f.service.Runs(context.Background(), created.Id)
	require.NoError(t, err)
	assert.Len(t, runs, 1)
	f.neverMoreCommandsThan(t, 0, "a missed interval ran")
}
