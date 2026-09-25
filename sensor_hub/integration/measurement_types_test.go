//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	gen "example/sensorHub/gen"
	"example/sensorHub/testharness"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMeasurementTypes_ListAll(t *testing.T) {
	ensureSensorsRegistered(t)
	client.CollectAll()

	raw, status := client.GetAllMeasurementTypes()
	require.Equal(t, http.StatusOK, status)

	var mts []gen.MeasurementType
	require.NoError(t, json.Unmarshal(raw, &mts))
	assert.NotEmpty(t, mts, "should return at least one measurement type")

	// Every measurement type should include a default aggregation function
	// and at least one supported aggregation function
	for _, mt := range mts {
		assert.NotEmpty(t, mt.DefaultAggregationFunction,
			"measurement type %q should have a default aggregation function", mt.Name)
		assert.NotEmpty(t, mt.SupportedAggregationFunctions,
			"measurement type %q should have at least one supported aggregation function", mt.Name)
		assert.Contains(t, mt.SupportedAggregationFunctions, mt.DefaultAggregationFunction,
			"measurement type %q default function should be in its supported list", mt.Name)
	}
}

func TestMeasurementTypes_WithReadings(t *testing.T) {
	ensureSensorsRegistered(t)
	client.CollectAll()

	raw, status := client.GetMeasurementTypesWithReadings()
	require.Equal(t, http.StatusOK, status)

	var mts []gen.MeasurementType
	require.NoError(t, json.Unmarshal(raw, &mts))
	assert.NotEmpty(t, mts, "should return types that have readings")

	// Every returned type should also be in the full list
	allRaw, _ := client.GetAllMeasurementTypes()
	var allMts []gen.MeasurementType
	require.NoError(t, json.Unmarshal(allRaw, &allMts))

	allNames := make(map[string]bool)
	for _, mt := range allMts {
		allNames[mt.Name] = true
	}
	for _, mt := range mts {
		assert.True(t, allNames[mt.Name], "with-readings type %q should exist in full list", mt.Name)
	}
}

func TestMeasurementTypes_ForSensor(t *testing.T) {
	ensureSensorsRegistered(t)

	sensors, status := client.GetAllSensors()
	require.Equal(t, http.StatusOK, status)
	require.NotEmpty(t, sensors)

	raw, status := client.GetMeasurementTypesForSensor(sensors[0].Id)
	require.Equal(t, http.StatusOK, status)

	var mts []gen.MeasurementType
	require.NoError(t, json.Unmarshal(raw, &mts))
	assert.NotEmpty(t, mts, "sensor should support at least one measurement type")
}

func TestMeasurementTypes_ForSensor_NonExistent(t *testing.T) {
	raw, status := client.GetMeasurementTypesForSensor(99999)
	assert.Equal(t, http.StatusOK, status)
	var mts []gen.MeasurementType
	require.NoError(t, json.Unmarshal(raw, &mts))
	assert.Empty(t, mts, "non-existent sensor should return empty list")
}

func TestMeasurementTypes_Unauthenticated(t *testing.T) {
	unauthed := testharness.NewClient(t, env.ServerURL)
	_, status := unauthed.GetAllMeasurementTypes()
	assert.Equal(t, http.StatusUnauthorized, status)
}

func TestMeasurementTypes_MinAndMaxSupportedForNumericTypesOnly(t *testing.T) {
	numeric := map[string]bool{
		"temperature": true, "humidity": true, "pressure": true, "power": true, "battery": true,
		"voltage": true, "luminance": true, "link_quality": true, "illuminance": true, "energy": true,
		"current": true, "co2": true, "voc": true, "formaldehyde": true, "pm25": true,
		"soil_moisture": true, "energy_today": true, "energy_month": true, "energy_yesterday": true,
	}

	raw, status := client.GetAllMeasurementTypes()
	require.Equal(t, http.StatusOK, status)
	var mts []gen.MeasurementType
	require.NoError(t, json.Unmarshal(raw, &mts))

	seen := 0
	for _, mt := range mts {
		if numeric[mt.Name] {
			seen++
			assert.Contains(t, mt.SupportedAggregationFunctions, "min", mt.Name)
			assert.Contains(t, mt.SupportedAggregationFunctions, "max", mt.Name)
			assert.Equal(t, "avg", mt.DefaultAggregationFunction, mt.Name)
			continue
		}
		assert.NotContains(t, mt.SupportedAggregationFunctions, "min", mt.Name)
		assert.NotContains(t, mt.SupportedAggregationFunctions, "max", mt.Name)
	}
	assert.Equal(t, len(numeric), seen)
}

func TestMeasurementTypes_IncreaseSupportedForEnergyCountersOnly(t *testing.T) {
	counters := map[string]bool{"energy": true, "energy_today": true, "energy_month": true}

	raw, status := client.GetAllMeasurementTypes()
	require.Equal(t, http.StatusOK, status)
	var mts []gen.MeasurementType
	require.NoError(t, json.Unmarshal(raw, &mts))

	seen := 0
	for _, mt := range mts {
		if counters[mt.Name] {
			seen++
			assert.Contains(t, mt.SupportedAggregationFunctions, "increase", mt.Name)
			assert.Equal(t, "avg", mt.DefaultAggregationFunction, mt.Name)
			continue
		}
		assert.NotContains(t, mt.SupportedAggregationFunctions, "increase", mt.Name)
	}
	assert.Equal(t, len(counters), seen)
}
