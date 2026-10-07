package automation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"example/sensorHub/automation"
	gen "example/sensorHub/gen"
	"example/sensorHub/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// storeTemperatures stores readings that alternate by swing, a minute apart
// and newer than any stored before.
func (f *fixture) storeTemperatures(t *testing.T, count int, swing float64) {
	t.Helper()
	var newest string
	require.NoError(t, f.db.Reader.QueryRow("SELECT COALESCE(MAX(time), '') FROM readings").Scan(&newest))
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if newest != "" {
		parsed, err := time.Parse(time.RFC3339, utils.NormalizeTimeToRFC3339(newest))
		require.NoError(t, err)
		at = parsed
	}
	temperature := measurementType(t, f.db, "temperature")
	tx, err := f.db.Writer.Begin()
	require.NoError(t, err)
	for i := range count {
		at = at.Add(time.Minute)
		_, err := tx.Exec("INSERT INTO readings (sensor_id, measurement_type_id, numeric_value, time) VALUES (?, ?, ?, ?)",
			f.climate.Id, temperature.Id, 20+float64(i%2)*swing, utils.FormatStorageTime(at))
		require.NoError(t, err)
	}
	require.NoError(t, tx.Commit())
}

func (f *fixture) marginHint(t *testing.T, automationID int) (*float64, *time.Time) {
	t.Helper()
	got, err := f.service.Get(context.Background(), automationID)
	require.NoError(t, err)
	return got.Triggers[0].MarginHint, got.Triggers[0].MarginHintCheckedAt
}

func TestMarginSuggestion_ComesFromTheNewestReadingsOfTheSeries(t *testing.T) {
	f := newFixture(t)
	f.storeTemperatures(t, 1000, 0.4)

	suggestion, err := f.service.MarginSuggestion(context.Background(), f.climate.Id, "temperature")
	require.NoError(t, err)
	assert.Equal(t, gen.MarginConfidenceHigh, suggestion.Confidence)
	assert.InDelta(t, 0.4, *suggestion.SuggestedMargin, 1e-9)

	f.storeTemperatures(t, 1000, 0.1)
	suggestion, err = f.service.MarginSuggestion(context.Background(), f.climate.Id, "temperature")
	require.NoError(t, err)
	assert.InDelta(t, 0.1, *suggestion.SuggestedMargin, 1e-9)
}

func TestMarginSuggestion_IsRejectedForABinaryMeasurementType(t *testing.T) {
	f := newFixture(t)

	_, err := f.service.MarginSuggestion(context.Background(), f.door.Id, "contact")

	var invalid *automation.ValidationError
	require.True(t, errors.As(err, &invalid), "got %v", err)
	assert.Contains(t, invalid.Message, "measurement_type")
}

func TestMarginCheck_HintsALargerMarginWithoutChangingTheSavedOneOrNotifying(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{below(f.climate.Id, 16, 0.2)}, setStep(f.lampID, "state", "ON"))
	f.storeTemperatures(t, 100, 0.4)

	require.NoError(t, f.service.CheckMargins(context.Background()))

	hint, checkedAt := f.marginHint(t, created.Id)
	require.NotNil(t, hint)
	assert.InDelta(t, 0.4, *hint, 1e-9)
	assert.NotNil(t, checkedAt)
	got, err := f.service.Get(context.Background(), created.Id)
	require.NoError(t, err)
	assert.Equal(t, 0.2, *got.Triggers[0].RearmMargin)
	assert.Empty(t, f.notifier.all())
}

func TestMarginCheck_GivesNoHintOnLowConfidence(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{below(f.climate.Id, 16, 0.2)}, setStep(f.lampID, "state", "ON"))
	f.storeTemperatures(t, 99, 0.4)

	require.NoError(t, f.service.CheckMargins(context.Background()))

	hint, _ := f.marginHint(t, created.Id)
	assert.Nil(t, hint)
}

func TestMarginCheck_HintClearsWhenTheTriggerIsSaved(t *testing.T) {
	f := newFixture(t)
	input := gen.AutomationInput{Name: "Heating on", Triggers: []gen.AutomationTrigger{below(f.climate.Id, 16, 0.2)},
		Steps: []gen.AutomationStep{setStep(f.lampID, "state", "ON")}}
	created, err := f.service.Create(context.Background(), input)
	require.NoError(t, err)
	f.storeTemperatures(t, 100, 0.4)
	require.NoError(t, f.service.CheckMargins(context.Background()))

	_, err = f.service.Update(context.Background(), created.Id, input)
	require.NoError(t, err)

	hint, checkedAt := f.marginHint(t, created.Id)
	assert.Nil(t, hint)
	assert.Nil(t, checkedAt)
}

func TestMarginCheck_HintClearsWhenALaterCheckFindsTheMarginEnough(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{below(f.climate.Id, 16, 0.2)}, setStep(f.lampID, "state", "ON"))
	f.storeTemperatures(t, 100, 0.4)
	require.NoError(t, f.service.CheckMargins(context.Background()))

	f.storeTemperatures(t, 100, 0.2)
	require.NoError(t, f.service.CheckMargins(context.Background()))

	hint, checkedAt := f.marginHint(t, created.Id)
	assert.Nil(t, hint)
	assert.Nil(t, checkedAt)
}
