//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	gen "example/sensorHub/gen"
	"example/sensorHub/testharness"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestThresholdAlert_EndToEnd verifies the full alert-firing flow:
// rule evaluation → alert_sent_history row → notification row →
// user_notification assignment → WS broadcast → email dispatch.
func TestThresholdAlert_EndToEnd(t *testing.T) {
	ensureSensorsRegistered(t)

	sensor, _ := client.GetSensorByName("Mock Sensor 1")
	require.True(t, sensor.Id > 0, "Mock Sensor 1 must be registered")

	// Create a user with email so the email path can be exercised.
	const alertTestUser = "alert-email-user"
	const alertTestPass = "alertuserpass123"
	const alertTestEmail = "alertuser@example.com"
	alertClient := testharness.NewClient(t, env.ServerURL)
	_, createStatus := client.CreateUser(gen.CreateUserRequest{
		Username: alertTestUser,
		Password: alertTestPass,
		Email:    ptrStr(alertTestEmail),
		Roles:    &[]string{"admin"},
	})
	if createStatus != http.StatusCreated {
		t.Logf("alert-email-user already exists (status %d), continuing", createStatus)
	}

	alertClient.Login(alertTestUser, alertTestPass)
	alertClient.ChangePassword(alertTestPass)

	// Enable email for threshold_alert notifications on this user.
	emailEnabled := true
	require.Equal(t, http.StatusOK,
		alertClient.SetChannelPreference(gen.ChannelPreference{
			Category:     gen.ChannelPreferenceCategoryThresholdAlert,
			EmailEnabled: &emailEnabled,
		}),
	)

	// Ensure a threshold rule that will always fire exists for Mock Sensor 1:
	// mock sensors return 18-22°C, so HighThreshold: 15.0 always triggers.
	// RateLimitSeconds: 0 disables rate-limiting so repeated test runs aren't blocked.
	wantRule := gen.AlertRule{
		SensorID:          sensor.Id,
		MeasurementTypeID: 1, // temperature
		AlertType:         "numeric_range",
		HighThreshold:     15.0,
		LowThreshold:      5.0,
		RateLimitSeconds:  0,
		Enabled:           true,
	}
	_, createRuleStatus := client.CreateAlertRule(wantRule)
	if createRuleStatus != http.StatusCreated {
		// A rule already exists for this sensor/measurement_type — update it so that
		// the thresholds and rate-limit guarantee the alert fires.
		rulesRaw, getRulesStatus := client.GetAlertRulesBySensorID(sensor.Id)
		require.Equal(t, http.StatusOK, getRulesStatus)
		var existingRules []gen.AlertRule
		require.NoError(t, json.Unmarshal(rulesRaw, &existingRules))
		require.NotEmpty(t, existingRules, "expected an existing alert rule")
		_, updateStatus := client.UpdateAlertRuleWithBody(existingRules[0].ID, map[string]any{
			"AlertType":        "numeric_range",
			"HighThreshold":    15.0,
			"LowThreshold":     5.0,
			"RateLimitSeconds": 0,
			"Enabled":          true,
		})
		require.Equal(t, http.StatusOK, updateStatus, "failed to update existing alert rule to fire threshold")
	}

	// Reset captures so earlier test runs don't pollute assertions.
	env.WSCapture.Reset()
	env.EmailCapture.Reset()

	// Trigger collection — this is what fires the alert.
	_, status := client.CollectByName("Mock Sensor 1")
	require.Equal(t, http.StatusOK, status)

	// --- Assert DB persistence ---

	var alertHistoryCount int
	err := env.DB.Reader.QueryRow(
		`SELECT COUNT(*) FROM alert_sent_history WHERE sensor_id = ?`, sensor.Id,
	).Scan(&alertHistoryCount)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, alertHistoryCount, 1, "alert_sent_history should have at least one row")

	var notifCount int
	err = env.DB.Reader.QueryRow(
		`SELECT COUNT(*) FROM notifications WHERE category = 'threshold_alert'`,
	).Scan(&notifCount)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, notifCount, 1, "notifications table should have at least one threshold_alert row")

	var userNotifCount int
	err = env.DB.Reader.QueryRow(
		`SELECT COUNT(*) FROM user_notifications un
		 JOIN notifications n ON un.notification_id = n.id
		 WHERE n.category = 'threshold_alert'`,
	).Scan(&userNotifCount)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, userNotifCount, 1, "user_notifications should have rows for threshold_alert")

	// --- Assert WS broadcast ---
	// WS broadcast is synchronous inside ProcessReading, so no sleep needed.
	assert.NotEmpty(t, env.WSCapture.UserIDs(), "WS broadcast should have fired for at least one user")

	// --- Assert email dispatch ---
	// Email is sent in a background goroutine; allow a short window for it to complete.
	assert.Eventually(t, func() bool {
		return len(env.EmailCapture.Recipients()) > 0
	}, 500*time.Millisecond, 20*time.Millisecond, "email should be dispatched to at least one recipient")
	assert.Contains(t, env.EmailCapture.Recipients(), alertTestEmail)
}
