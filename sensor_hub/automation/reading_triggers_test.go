package automation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"example/sensorHub/automation"
	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readingTrigger(sensorID int, measurementType string, operator gen.AutomationTriggerOperator) gen.AutomationTrigger {
	return gen.AutomationTrigger{Type: gen.AutomationTriggerTypeReading, SensorId: &sensorID, MeasurementType: &measurementType, Operator: &operator}
}

func below(sensorID int, threshold, margin float64) gen.AutomationTrigger {
	trigger := readingTrigger(sensorID, "temperature", gen.AutomationTriggerOperatorFallsBelow)
	trigger.Threshold, trigger.RearmMargin = &threshold, &margin
	return trigger
}

func becomes(sensorID int, value string) gen.AutomationTrigger {
	trigger := readingTrigger(sensorID, "contact", gen.AutomationTriggerOperatorBecomes)
	trigger.Value = &value
	return trigger
}

func held(trigger gen.AutomationTrigger, seconds int) gen.AutomationTrigger {
	trigger.HoldSeconds = &seconds
	return trigger
}

func (f *fixture) temperature(values ...float64) {
	readings := make([]gen.Reading, 0, len(values))
	for _, value := range values {
		readings = append(readings, gen.Reading{MeasurementType: "temperature", NumericValue: &value})
	}
	f.readings.Consume(context.Background(), f.climate, readings)
}

func (f *fixture) runCount(t *testing.T, automationID int) int {
	t.Helper()
	runs, err := f.service.Runs(context.Background(), automationID)
	require.NoError(t, err)
	return len(runs)
}

func (f *fixture) neverMoreRunsThan(t *testing.T, automationID int, limit int, wait time.Duration, message string) {
	t.Helper()
	for deadline := time.Now().Add(wait); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if f.runCount(t, automationID) > limit {
			assert.Fail(t, message)
			return
		}
	}
}

func TestReadingTrigger_ACrossingStartsARunWithinASecond(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{below(f.climate.Id, 16, 0.2)}, setStep(f.lampID, "state", "ON"))

	f.temperature(16.2)
	consumed := time.Now()
	f.temperature(15.9)

	sent := f.commands.await(t, 0)
	assert.WithinDuration(t, consumed, sent.at, time.Second)
	sent.outcome <- "acknowledged"
	run := f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)
	assert.Equal(t, gen.AutomationRunTriggerKindReading, run.TriggerKind)
	assert.Equal(t, created.Triggers[0].Id, run.TriggerId)
}

func TestReadingTrigger_AHeldConditionFiresWhenTheHoldEndsWithoutAnotherReading(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{held(below(f.climate.Id, 16, 0.2), 1)}, setStep(f.lampID, "state", "ON"))

	f.temperature(15.9)
	consumed := time.Now()

	sent := f.commands.await(t, 0)
	assert.WithinDuration(t, consumed.Add(time.Second), sent.at, 500*time.Millisecond)
	sent.outcome <- "acknowledged"
	f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)
}

func TestReadingTrigger_AnAutomationThatIsOffIgnoresReadingsAndSwitchingItOnStartsAfresh(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	created := f.create(t, []gen.AutomationTrigger{below(f.climate.Id, 16, 0.2)}, setStep(f.lampID, "state", "ON"))
	f.temperature(15.9)
	f.commands.await(t, 0).outcome <- "acknowledged"
	f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)

	_, err := f.service.SetEnabled(ctx, created.Id, false)
	require.NoError(t, err)
	f.temperature(16.5, 15.5)
	f.neverMoreRunsThan(t, created.Id, 1, 200*time.Millisecond, "an automation that is off started a run")

	_, err = f.service.SetEnabled(ctx, created.Id, true)
	require.NoError(t, err)
	f.temperature(15.4)
	f.commands.await(t, 1).outcome <- "acknowledged"
	require.Eventually(t, func() bool { return f.runCount(t, created.Id) == 2 }, 5*time.Second, 10*time.Millisecond)
}

func TestReadingTrigger_AnAutomationWithOnlyReadingTriggersHasNoNextFireTime(t *testing.T) {
	f := newFixture(t)

	created := f.create(t, []gen.AutomationTrigger{below(f.climate.Id, 16, 0.2)}, setStep(f.lampID, "state", "ON"))

	assert.Equal(t, gen.AutomationStatusArmed, created.Status)
	assert.Nil(t, created.NextFireAt)
}

func TestReadingTrigger_IsReturnedAsSaved(t *testing.T) {
	f := newFixture(t)

	created := f.create(t, []gen.AutomationTrigger{held(below(f.climate.Id, 16, 0.2), 300), becomes(f.door.Id, "false")}, setStep(f.lampID, "state", "ON"))

	numeric, binary := created.Triggers[0], created.Triggers[1]
	assert.Equal(t, gen.AutomationTriggerTypeReading, numeric.Type)
	assert.Equal(t, f.climate.Id, *numeric.SensorId)
	assert.Equal(t, "temperature", *numeric.MeasurementType)
	assert.Equal(t, gen.AutomationTriggerOperatorFallsBelow, *numeric.Operator)
	assert.Equal(t, 16.0, *numeric.Threshold)
	assert.Equal(t, 0.2, *numeric.RearmMargin)
	assert.Equal(t, 300, *numeric.HoldSeconds)
	assert.Nil(t, numeric.Value)
	assert.Equal(t, "false", *binary.Value)
	assert.Equal(t, 0, *binary.HoldSeconds)
	assert.Nil(t, binary.RearmMargin)
}

func TestReadingTrigger_IsRejectedWhenInvalid(t *testing.T) {
	f := newFixture(t)
	noMargin := below(f.climate.Id, 16, 0)
	noMargin.RearmMargin = nil
	binaryWithMargin := becomes(f.door.Id, "false")
	margin := 0.2
	binaryWithMargin.RearmMargin = &margin
	becomesOnNumeric := readingTrigger(f.climate.Id, "temperature", gen.AutomationTriggerOperatorBecomes)
	becomesOnNumeric.Value = &[]string{"20"}[0]
	belowOnBinary := below(f.door.Id, 16, 0.2)
	belowOnBinary.MeasurementType = &[]string{"contact"}[0]
	notReported := below(f.climate.Id, 16, 0.2)
	notReported.MeasurementType = &[]string{"pressure"}[0]

	for name, testCase := range map[string]struct {
		trigger gen.AutomationTrigger
		field   string
	}{
		"numeric without a margin":    {noMargin, "triggers[0].rearm_margin"},
		"negative margin":             {below(f.climate.Id, 16, -0.1), "triggers[0].rearm_margin"},
		"binary with a margin":        {binaryWithMargin, "triggers[0].rearm_margin"},
		"becomes on a numeric series": {becomesOnNumeric, "triggers[0].operator"},
		"falls below on a binary one": {belowOnBinary, "triggers[0].operator"},
		"type the sensor never sends": {notReported, "triggers[0].measurement_type"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := f.service.Create(context.Background(), gen.AutomationInput{
				Name: "Lounge heat on", Triggers: []gen.AutomationTrigger{testCase.trigger}, Steps: []gen.AutomationStep{setStep(f.lampID, "state", "ON")},
			})
			var invalid *automation.ValidationError
			require.True(t, errors.As(err, &invalid), "got %v", err)
			assert.Contains(t, invalid.Message, testCase.field)
		})
	}
}
