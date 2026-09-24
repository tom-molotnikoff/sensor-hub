//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlerts_CreateAndGetRule(t *testing.T) {
	ensureSensorsRegistered(t)

	sensor, _ := client.GetSensorByName("Mock Sensor 1")
	require.True(t, sensor.Id > 0)

	rule := gen.AlertRule{
		SensorId:          sensor.Id,
		MeasurementTypeId: 1, // temperature
		AlertType:         "numeric_range",
		HighThreshold:     30.0,
		LowThreshold:      10.0,
		RateLimitSeconds:  6,
		Enabled:           true,
	}

	_, status := client.CreateAlertRule(rule)
	require.Equal(t, http.StatusCreated, status)

	resp, status := client.GetAlertRulesBySensorID(sensor.Id)
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(resp), "numeric_range")

	var raw []map[string]any
	require.NoError(t, json.Unmarshal(resp, &raw))
	require.NotEmpty(t, raw)
	assert.Contains(t, raw[0], "sensor_id")
	assert.Contains(t, raw[0], "rate_limit_seconds")
	assert.NotContains(t, raw[0], "SensorID")
}

// TestAlerts_EditRuleWithMutableFieldsOnly is the regression test for #51:
// the UI's edit dialog only sends mutable fields (alert type, thresholds,
// trigger status, rate limit, enabled). The server must look up the existing
// rule and preserve the immutable SensorID and MeasurementTypeID so the PUT
// succeeds instead of failing validation with 400 Bad Request.
func TestAlerts_EditRuleWithMutableFieldsOnly(t *testing.T) {
	ensureSensorsRegistered(t)

	sensor, _ := client.GetSensorByName("Mock Sensor 1")
	require.True(t, sensor.Id > 0)

	// Find an existing rule for this sensor (created by an earlier test) or
	// create one. The (sensor_id, measurement_type_id, alert_type) tuple is
	// unique, so we don't unconditionally re-create.
	rulesRaw, status := client.GetAlertRulesBySensorID(sensor.Id)
	require.Equal(t, http.StatusOK, status)
	var rules []gen.AlertRule
	require.NoError(t, json.Unmarshal(rulesRaw, &rules))

	if len(rules) == 0 {
		_, status := client.CreateAlertRule(gen.AlertRule{
			SensorId:          sensor.Id,
			MeasurementTypeId: 1,
			AlertType:         "numeric_range",
			HighThreshold:     30.0,
			LowThreshold:      10.0,
			RateLimitSeconds:  6,
			Enabled:           true,
		})
		require.Equal(t, http.StatusCreated, status)
		rulesRaw, status = client.GetAlertRulesBySensorID(sensor.Id)
		require.Equal(t, http.StatusOK, status)
		require.NoError(t, json.Unmarshal(rulesRaw, &rules))
	}
	require.NotEmpty(t, rules)
	ruleID := rules[0].Id

	// Body matches what EditAlertDialog.tsx sends: only mutable fields, no
	// SensorID or MeasurementTypeID. Pre-fix this returned 400.
	body := map[string]any{
		"alert_type":         "numeric_range",
		"high_threshold":     35.0,
		"low_threshold":      12.0,
		"rate_limit_seconds": 60,
		"enabled":            false,
	}
	_, status = client.UpdateAlertRuleWithBody(ruleID, body)
	assert.Equal(t, http.StatusOK, status)

	// Verify the update actually applied and immutable fields are intact.
	rulesRaw, status = client.GetAlertRulesBySensorID(sensor.Id)
	require.Equal(t, http.StatusOK, status)
	require.NoError(t, json.Unmarshal(rulesRaw, &rules))
	var updated *gen.AlertRule
	for i := range rules {
		if rules[i].Id == ruleID {
			updated = &rules[i]
			break
		}
	}
	require.NotNil(t, updated)
	assert.Equal(t, sensor.Id, updated.SensorId)
	assert.Equal(t, 1, updated.MeasurementTypeId)
	assert.Equal(t, 35.0, updated.HighThreshold)
	assert.Equal(t, 12.0, updated.LowThreshold)
	assert.Equal(t, 60, updated.RateLimitSeconds)
	assert.False(t, updated.Enabled)
}

func TestAlerts_CollectionTriggersAlertCheck(t *testing.T) {
	ensureSensorsRegistered(t)

	// Collect readings — exercises the alert evaluation code path.
	// Mock sensors return 18-22°C, so a threshold of 30 won't fire,
	// but this validates the NullSQLiteTime fix doesn't cause scan errors.
	_, status := client.CollectAll()
	assert.Equal(t, http.StatusOK, status)

	sensor, _ := client.GetSensorByName("Mock Sensor 1")
	resp, status := client.GetAlertHistory(sensor.Id)
	require.Equal(t, http.StatusOK, status)
	// History may be empty (no alerts fired), but the endpoint should work
	assert.NotNil(t, resp)
}
