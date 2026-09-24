//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadings_BetweenDates(t *testing.T) {
	ensureSensorsRegistered(t)
	client.CollectAll()

	now := time.Now().UTC()
	from := now.Add(-1 * time.Hour).Format("2006-01-02")
	to := now.Add(24 * time.Hour).Format("2006-01-02")

	readings, status := client.GetReadingsBetween(from, to, "")
	require.Equal(t, http.StatusOK, status)
	require.NotEmpty(t, readings)
	for _, r := range readings {
		parsed, err := time.Parse(time.RFC3339, r.Time)
		require.NoError(t, err, "reading time %q is not RFC3339", r.Time)
		assert.Equal(t, time.UTC, parsed.Location())
	}
}

func TestReadings_FilterBySensor(t *testing.T) {
	ensureSensorsRegistered(t)
	client.CollectAll()

	now := time.Now().UTC()
	from := now.Add(-1 * time.Hour).Format("2006-01-02")
	to := now.Add(24 * time.Hour).Format("2006-01-02")

	readings, status := client.GetReadingsBetween(from, to, "Mock Sensor 1")
	require.Equal(t, http.StatusOK, status)

	for _, r := range readings {
		assert.Equal(t, "Mock Sensor 1", r.SensorName)
	}
}

func TestReadings_FilterBySensorCaseInsensitive(t *testing.T) {
	ensureSensorsRegistered(t)
	client.CollectAll()

	now := time.Now().UTC()
	from := now.Add(-1 * time.Hour).Format("2006-01-02")
	to := now.Add(24 * time.Hour).Format("2006-01-02")

	readings, status := client.GetReadingsBetween(from, to, "mock sensor 1")
	require.Equal(t, http.StatusOK, status)
	assert.NotEmpty(t, readings)
}

func TestReadings_NoResults(t *testing.T) {
	resp, body, err := client.GetRaw("/api/readings/between?start=2020-01-01&end=2020-01-02", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &raw))
	assert.JSONEq(t, "[]", string(raw["readings"]))
}

func TestReadings_ISODatetimeRange(t *testing.T) {
	ensureSensorsRegistered(t)
	client.CollectAll()

	now := time.Now().UTC()
	from := now.Add(-1 * time.Hour).Format(time.RFC3339)
	to := now.Add(1 * time.Hour).Format(time.RFC3339)

	readings, status := client.GetReadingsBetween(from, to, "")
	require.Equal(t, http.StatusOK, status)
	assert.NotEmpty(t, readings)
}

func TestReadings_DatetimeNarrowerThanDate(t *testing.T) {
	ensureSensorsRegistered(t)
	client.CollectAll()

	// Use a range far in the past — should return nothing
	readings, status := client.GetReadingsBetween("2020-06-15T10:00:00Z", "2020-06-15T11:00:00Z", "")
	require.Equal(t, http.StatusOK, status)
	assert.Empty(t, readings)
}

// ============================================================================
// Auto-aggregation tests
// ============================================================================

func TestReadings_AggregationMetadata_ShortRange(t *testing.T) {
	ensureSensorsRegistered(t)
	client.CollectAll()

	now := time.Now().UTC()
	from := now.Add(-5 * time.Minute).Format("2006-01-02 15:04:05")
	to := now.Add(5 * time.Minute).Format("2006-01-02 15:04:05")

	resp, status := client.GetReadingsBetweenAggregated(from, to, "", "", "", "")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "raw", string(resp.AggregationInterval), "short range should return raw readings")
	assert.Equal(t, "none", string(resp.AggregationFunction))
}

func TestReadings_AggregationMetadata_LongRange(t *testing.T) {
	ensureSensorsRegistered(t)
	client.CollectAll()

	now := time.Now().UTC()
	from := now.Add(-8 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	to := now.Add(1 * time.Hour).Format("2006-01-02 15:04:05")

	resp, status := client.GetReadingsBetweenAggregated(from, to, "", "", "", "")
	require.Equal(t, http.StatusOK, status)
	// 8-day span exceeds P7D tier (168h), falls into P30D tier → PT1H interval
	assert.Equal(t, "PT1H", string(resp.AggregationInterval))
	assert.NotEqual(t, "none", string(resp.AggregationFunction))
}

func TestReadings_AggregationOverride_Interval(t *testing.T) {
	ensureSensorsRegistered(t)
	client.CollectAll()

	now := time.Now().UTC()
	from := now.Add(-10 * time.Minute).Format("2006-01-02 15:04:05")
	to := now.Add(10 * time.Minute).Format("2006-01-02 15:04:05")

	resp, status := client.GetReadingsBetweenAggregated(from, to, "", "temperature", "PT5M", "")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "PT5M", string(resp.AggregationInterval))
}

func TestReadings_AggregationOverride_Function(t *testing.T) {
	ensureSensorsRegistered(t)
	client.CollectAll()

	now := time.Now().UTC()
	from := now.Add(-2 * time.Hour).Format("2006-01-02 15:04:05")
	to := now.Add(1 * time.Hour).Format("2006-01-02 15:04:05")

	// "avg" is supported for temperature — override explicitly
	resp, status := client.GetReadingsBetweenAggregated(from, to, "", "temperature", "PT1H", "avg")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "avg", string(resp.AggregationFunction))
}

func TestReadings_AggregationOverride_UnsupportedFunction(t *testing.T) {
	ensureSensorsRegistered(t)
	client.CollectAll()

	now := time.Now().UTC()
	from := now.Add(-2 * time.Hour).Format("2006-01-02 15:04:05")
	to := now.Add(1 * time.Hour).Format("2006-01-02 15:04:05")

	// "last" is not supported for temperature (only avg is)
	_, status := client.GetReadingsBetweenAggregated(from, to, "", "temperature", "PT1H", "last")
	require.Equal(t, http.StatusBadRequest, status, "unsupported aggregation function should return 400")
}

func TestReadings_AggregationOverride_FunctionWithoutType(t *testing.T) {
	ensureSensorsRegistered(t)

	now := time.Now().UTC()
	from := now.Add(-2 * time.Hour).Format("2006-01-02 15:04:05")
	to := now.Add(1 * time.Hour).Format("2006-01-02 15:04:05")

	_, status := client.GetReadingsBetweenAggregated(from, to, "", "", "PT1H", "count")
	require.Equal(t, http.StatusBadRequest, status)
}

func TestReadings_AggregatedResponse_HasReadings(t *testing.T) {
	ensureSensorsRegistered(t)
	client.CollectAll()

	now := time.Now().UTC()
	from := now.Add(-2 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	to := now.Add(1 * time.Hour).Format("2006-01-02 15:04:05")

	resp, status := client.GetReadingsBetweenAggregated(from, to, "", "", "", "")
	require.Equal(t, http.StatusOK, status)
	assert.NotEmpty(t, resp.Readings, "aggregated response should still contain readings")
	assert.NotEqual(t, "raw", string(resp.AggregationInterval), "2-day range should trigger aggregation")
}
