package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadingsBetweenCommand_SendsMeasurementType(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		require.Equal(t, "/api/readings/between", r.URL.Path)
		assert.Equal(t, "contact", r.URL.Query().Get("type"))
		assert.Equal(t, "count", r.URL.Query().Get("aggregation_function"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(gen.AggregatedReadingsResponse{
			AggregationInterval: gen.AggregatedReadingsResponseAggregationIntervalPT1H,
			AggregationFunction: gen.AggregatedReadingsResponseAggregationFunctionCount,
			Readings:            []gen.Reading{},
		}))
	}))
	defer server.Close()

	_, _, err := executeRootCommand(t, "--server", server.URL, "readings", "between",
		"--sensor", "front-door", "--type", "contact",
		"--start", "2026-09-01", "--end", "2026-09-08",
		"--aggregation-function", "count")
	require.NoError(t, err)
	assert.True(t, called)
}
