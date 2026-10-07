package automation_test

import (
	"context"
	"testing"

	"example/sensorHub/automation"
	gen "example/sensorHub/gen"
	"example/sensorHub/notifications"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (f *fixture) dropCapability(property string) {
	f.sensors.change(f.lampID, func(sensor *gen.Sensor) {
		var kept []gen.Capability
		for _, capability := range *sensor.Capabilities {
			if capability.Property != property {
				kept = append(kept, capability)
			}
		}
		sensor.Capabilities = &kept
	})
}

func (f *fixture) restoreLamp() {
	f.sensors.change(f.lampID, func(sensor *gen.Sensor) { *sensor = lamp(f.lampID) })
}

func (f *fixture) get(t *testing.T, id int) gen.Automation {
	t.Helper()
	view, err := f.service.Get(context.Background(), id)
	require.NoError(t, err)
	return view
}

func TestBroken_ALostPropertyBreaksTheAutomationWhichNotifiesOnceAndStartsNoRuns(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())},
		setStep(f.lampID, "state", "ON"),
		setStep(f.lampID, "brightness", "150"),
	)

	f.dropCapability("brightness")
	f.service.SensorChanged(ctx, f.lampID)
	f.service.SensorChanged(ctx, f.lampID)

	broken := f.get(t, created.Id)
	assert.Equal(t, gen.AutomationStatusBroken, broken.Status)
	assert.Equal(t, ptr("step 2: hallway-lamp no longer has brightness"), broken.StatusReason)
	assert.Nil(t, broken.NextFireAt)
	sent := f.notifier.all()
	require.Len(t, sent, 1)
	assert.Equal(t, "manage_automations", sent[0].permission)
	assert.Equal(t, notifications.CategoryAutomationFailure, sent[0].notification.Category)
	assert.Contains(t, sent[0].notification.Title, "Evening lights")
	assert.Contains(t, sent[0].notification.Message, "step 2: hallway-lamp no longer has brightness")

	f.fire(created)
	_, err := f.service.RunNow(ctx, created.Id, 1)
	assert.ErrorIs(t, err, automation.ErrBroken)
	f.neverMoreCommandsThan(t, 0, "a broken automation sent a command")
	runs, err := f.service.Runs(ctx, created.Id)
	require.NoError(t, err)
	assert.Empty(t, runs, "a broken automation recorded a run")
	assert.Len(t, f.notifier.all(), 1)
}

func TestBroken_LeavesQuietlyWhenTheCapabilityComesBackAndALaterBreakNotifiesAgain(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, setStep(f.lampID, "effect", "blink"))
	f.dropCapability("effect")
	f.service.SensorChanged(ctx, f.lampID)
	require.Equal(t, gen.AutomationStatusBroken, f.get(t, created.Id).Status)

	f.restoreLamp()
	f.service.SensorChanged(ctx, f.lampID)

	fixed := f.get(t, created.Id)
	assert.Equal(t, gen.AutomationStatusArmed, fixed.Status)
	assert.Nil(t, fixed.StatusReason)
	assert.NotNil(t, fixed.NextFireAt)
	assert.Len(t, f.notifier.all(), 1, "leaving Broken notified")
	f.fire(created)
	f.commands.await(t, 0).outcome <- "acknowledged"
	f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)

	f.dropCapability("effect")
	f.service.SensorChanged(ctx, f.lampID)
	assert.Len(t, f.notifier.all(), 2, "a later break did not notify")
}

func TestBroken_SavingTheAutomationValidLeavesBrokenQuietly(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, setStep(f.lampID, "brightness", "150"))
	f.dropCapability("brightness")
	f.service.SensorChanged(ctx, f.lampID)

	saved, err := f.service.Update(ctx, created.Id, gen.AutomationInput{
		Name:     "Evening lights",
		Triggers: []gen.AutomationTrigger{scheduleTrigger(laterToday())},
		Steps:    []gen.AutomationStep{setStep(f.lampID, "state", "ON")},
	})

	require.NoError(t, err)
	assert.Equal(t, gen.AutomationStatusArmed, saved.Status)
	assert.NotNil(t, saved.NextFireAt)
	assert.Len(t, f.notifier.all(), 1)
}

func TestBroken_StartupRechecksEveryAutomationAndARestartIsNotANewBreak(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, setStep(f.lampID, "state", "ON"))
	f.sensors.change(f.lampID, func(sensor *gen.Sensor) { sensor.Capabilities = &[]gen.Capability{} })

	f.restart(t)

	broken := f.get(t, created.Id)
	assert.Equal(t, gen.AutomationStatusBroken, broken.Status)
	assert.Equal(t, ptr("step 1: hallway-lamp is no longer controllable"), broken.StatusReason)
	assert.Len(t, f.notifier.all(), 1)

	f.restart(t)

	assert.Equal(t, gen.AutomationStatusBroken, f.get(t, created.Id).Status)
	assert.Len(t, f.notifier.all(), 1, "a restart counted as a new break")
}
