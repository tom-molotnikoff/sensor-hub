package automation_test

import (
	"testing"

	appProps "example/sensorHub/application_properties"
	gen "example/sensorHub/gen"
	"example/sensorHub/notifications"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (f *fixture) scheduledRun(t *testing.T) (gen.Automation, gen.AutomationRun) {
	t.Helper()
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, setStep(f.lampID, "state", "ON"))
	f.fire(created)
	f.commands.await(t, f.commands.count()).outcome <- "acknowledged"
	return created, f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)
}

func (f *fixture) maxCauseChain(t *testing.T, limit string) {
	t.Helper()
	original := appProps.AppConfig()
	t.Cleanup(func() { appProps.SetAppConfig(original) })
	application, smtp, db := appProps.BuildDefaults()
	application["automation.loop.max.chain"] = limit
	require.NoError(t, appProps.ReloadConfig(application, smtp, db))
}

func TestLoopGuard_ARunStartedByAReadingRecordsTheRunWhoseCommandItAcknowledged(t *testing.T) {
	f := newFixture(t)
	cause, causeRun := f.scheduledRun(t)
	created := f.create(t, []gen.AutomationTrigger{below(f.climate.Id, 16, 0.2)}, setStep(f.lampID, "state", "ON"))

	f.temperatureCausedBy(&causeRun.Id, 15.9)
	f.commands.await(t, 1).outcome <- "acknowledged"
	caused := f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)
	f.temperature(16.2, 15.9)
	f.commands.await(t, 2).outcome <- "acknowledged"
	uncaused := f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)

	assert.Equal(t, &gen.AutomationCauseRun{Id: causeRun.Id, AutomationId: cause.Id, AutomationName: "Evening lights"}, caused.CauseRun)
	require.NotEqual(t, caused.Id, uncaused.Id)
	assert.Nil(t, uncaused.CauseRun)
}

func TestLoopGuard_AHeldTriggerKeepsTheCauseOfTheReadingThatStartedTheHold(t *testing.T) {
	f := newFixture(t)
	_, causeRun := f.scheduledRun(t)
	created := f.create(t, []gen.AutomationTrigger{held(below(f.climate.Id, 16, 0.2), 1)}, setStep(f.lampID, "state", "ON"))

	f.temperatureCausedBy(&causeRun.Id, 15.9)
	f.commands.await(t, 1).outcome <- "acknowledged"

	run := f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)
	require.NotNil(t, run.CauseRun)
	assert.Equal(t, causeRun.Id, run.CauseRun.Id)
}

func TestLoopGuard_RefusesARunWhoseCauseChainIsAlreadyAtTheLimit(t *testing.T) {
	f := newFixture(t)
	f.maxCauseChain(t, "2")
	_, first := f.scheduledRun(t)
	created := f.create(t, []gen.AutomationTrigger{below(f.climate.Id, 16, 0.2)}, setStep(f.lampID, "state", "ON"))
	f.temperatureCausedBy(&first.Id, 15.9)
	f.commands.await(t, 1).outcome <- "acknowledged"
	second := f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)

	f.temperatureCausedBy(&second.Id, 16.2, 15.9)

	refused := f.latestRun(t, created.Id, gen.AutomationRunStatusFailed)
	require.NotNil(t, refused.CauseRun)
	assert.Equal(t, second.Id, refused.CauseRun.Id)
	require.NotNil(t, refused.Error)
	assert.Regexp(t, `^loop guard: 2 automation runs in a row .*automation\.loop\.max\.chain`, *refused.Error)
	assert.Empty(t, refused.StepOutcomes)
	f.neverMoreCommandsThan(t, 2, "the refused run sent a command")

	sent := f.notifier.all()
	require.Len(t, sent, 1)
	assert.Equal(t, notifications.CategoryAutomationFailure, sent[0].notification.Category)
	assert.Equal(t, "manage_automations", sent[0].permission)
	assert.Contains(t, sent[0].notification.Message, "loop guard")
}
