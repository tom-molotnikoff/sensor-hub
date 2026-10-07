package automation

import (
	"slices"
	"testing"

	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// series turns changes between consecutive readings into readings from 20.
func series(changes ...float64) []float64 {
	values := []float64{20}
	for _, change := range changes {
		values = append(values, values[len(values)-1]+change)
	}
	return values
}

func repeated(change float64, times int) []float64 {
	return slices.Repeat([]float64{change}, times)
}

func TestSuggestMargin_RoundsTheNinetyFifthPercentileChangeUpToAWholeStep(t *testing.T) {
	suggestion := suggestMargin(series(append(repeated(0.2, 9), repeated(-0.5, 20)...)...))

	require.NotNil(t, suggestion.SuggestedMargin)
	assert.InDelta(t, 0.5, *suggestion.P95Change, 1e-12)
	assert.InDelta(t, 0.2, *suggestion.Step, 1e-12)
	assert.InDelta(t, 0.6, *suggestion.SuggestedMargin, 1e-12)
}

func TestSuggestMargin_IsAtLeastOneStep(t *testing.T) {
	suggestion := suggestMargin(series(append(repeated(0, 28), 0.1)...))

	assert.Zero(t, *suggestion.P95Change)
	assert.InDelta(t, 0.1, *suggestion.SuggestedMargin, 1e-12)
}

func TestSuggestMargin_UsesTheLatestReadingsInThreeTiers(t *testing.T) {
	for _, tc := range []struct {
		readings   int
		sample     int
		confidence gen.MarginSuggestionConfidence
	}{
		{1500, 1000, gen.MarginConfidenceHigh},
		{1000, 1000, gen.MarginConfidenceHigh},
		{999, 100, gen.MarginConfidenceMedium},
		{100, 100, gen.MarginConfidenceMedium},
		{99, 30, gen.MarginConfidenceLow},
		{30, 30, gen.MarginConfidenceLow},
	} {
		// Only readings beyond the sample jump, so a sample that reaches them shows it.
		changes := append(repeated(0.1, tc.sample-1), repeated(5, tc.readings-tc.sample)...)
		suggestion := suggestMargin(series(changes...))

		assert.Equal(t, tc.sample, suggestion.SampleCount, "%d readings", tc.readings)
		assert.Equal(t, tc.confidence, suggestion.Confidence, "%d readings", tc.readings)
		assert.InDelta(t, 0.1, *suggestion.SuggestedMargin, 1e-12, "%d readings", tc.readings)
	}
}

func TestSuggestMargin_HasNoSuggestionFromFewerThanThirtyReadings(t *testing.T) {
	suggestion := suggestMargin(series(repeated(0.1, 28)...))

	assert.Equal(t, gen.MarginConfidenceNone, suggestion.Confidence)
	assert.Equal(t, 29, suggestion.SampleCount)
	assert.Nil(t, suggestion.SuggestedMargin)
}

func TestSuggestMargin_IsNoLargerThanAThirdOfADegreeForALivingRoomThermometer(t *testing.T) {
	for seed := uint64(1); seed <= 4; seed++ {
		week := generatedWeek(seed)
		latest := make([]float64, 0, len(week))
		for i := len(week) - 1; i >= 0; i-- {
			latest = append(latest, week[i].value)
		}

		suggestion := suggestMargin(latest)

		require.Equal(t, gen.MarginConfidenceHigh, suggestion.Confidence)
		assert.LessOrEqual(t, *suggestion.SuggestedMargin, 0.3, "seed %d", seed)
	}
}
