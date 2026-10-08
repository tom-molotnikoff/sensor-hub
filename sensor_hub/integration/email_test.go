//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	gen "example/sensorHub/gen"
	"example/sensorHub/testharness"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// emailUser is signed in as an admin with an email address, so a test email
// has somewhere to go.
func emailUser(t *testing.T, username, address string, roles ...string) *testharness.Client {
	t.Helper()
	const password = "emailuserpass123"
	_, status := client.CreateUser(gen.CreateUserRequest{Username: username, Password: password, Email: ptrStr(address), Roles: &roles})
	require.Contains(t, []int{http.StatusCreated, http.StatusConflict}, status)
	user := testharness.NewClient(t, env.ServerURL)
	require.Equal(t, http.StatusOK, user.Login(username, password))
	require.Equal(t, http.StatusOK, user.ChangePassword(password))
	return user
}

func emailSettings(t *testing.T, c *testharness.Client) (gen.EmailSettings, string) {
	t.Helper()
	resp, status := c.GetEmailSettings()
	require.Equal(t, http.StatusOK, status, "body: %s", resp)
	var settings gen.EmailSettings
	require.NoError(t, json.Unmarshal(resp, &settings))
	return settings, string(resp)
}

func TestEmail_SavesTheSettingsAndNeverReturnsThePassword(t *testing.T) {
	const password = "write-only-smtp-password-91f3"
	settings := env.SMTPSettings(password)
	settings.Security = "starttls"

	resp, status := client.UpdateEmailSettings(settings)
	require.Equal(t, http.StatusOK, status, "body: %s", resp)
	assert.NotContains(t, string(resp), password)

	saved, raw := emailSettings(t, client)
	assert.NotContains(t, raw, password)
	assert.NotContains(t, raw, `"password":`)
	assert.Equal(t, gen.EmailSettingsPasswordStatusSet, *saved.PasswordStatus)
	assert.Equal(t, settings.Host, saved.Host)
	assert.Equal(t, settings.Port, saved.Port)
	assert.Equal(t, gen.EmailSettingsSecurity("starttls"), saved.Security)
	assert.Equal(t, settings.Username, saved.Username)
	assert.Equal(t, settings.FromAddress, saved.FromAddress)

	// Leaving the password out keeps it; an empty one clears it.
	settings.Password = nil
	_, status = client.UpdateEmailSettings(settings)
	require.Equal(t, http.StatusOK, status)
	saved, _ = emailSettings(t, client)
	assert.Equal(t, gen.EmailSettingsPasswordStatusSet, *saved.PasswordStatus)
	settings.Password = ptrStr("")
	_, status = client.UpdateEmailSettings(settings)
	require.Equal(t, http.StatusOK, status)
	saved, _ = emailSettings(t, client)
	assert.Equal(t, gen.EmailSettingsPasswordStatusUnset, *saved.PasswordStatus)

	settings.FromAddress = "not an address"
	_, status = client.UpdateEmailSettings(settings)
	assert.Equal(t, http.StatusBadRequest, status)
}

func TestEmail_TestSendReachesTheCallerAndRecordsTheSend(t *testing.T) {
	sender := emailUser(t, "email-test-sender", "email-test-sender@example.com", "admin")
	_, status := sender.UpdateEmailSettings(env.SMTPSettings(testharness.SMTPPassword))
	require.Equal(t, http.StatusOK, status)
	env.SMTP.Reset()

	resp, status := sender.SendTestEmail()

	require.Equal(t, http.StatusOK, status, "body: %s", resp)
	messages := env.SMTP.Messages()
	require.Len(t, messages, 1)
	assert.Equal(t, []string{"email-test-sender@example.com"}, messages[0].To)
	assert.Contains(t, messages[0].Data, "Subject: Sensor Hub test email")
	saved, _ := emailSettings(t, sender)
	assert.NotNil(t, saved.LastSentAt)
	assert.Nil(t, saved.LastError)
}

func TestEmail_TestSendThatTheServerRefusesAnswers502AndRecordsTheError(t *testing.T) {
	sender := emailUser(t, "email-refused-sender", "email-refused-sender@example.com", "admin")
	_, status := sender.UpdateEmailSettings(env.SMTPSettings(testharness.SMTPPassword))
	require.Equal(t, http.StatusOK, status)
	_, status = sender.SendTestEmail()
	require.Equal(t, http.StatusOK, status)
	before, _ := emailSettings(t, sender)
	require.NotNil(t, before.LastSentAt)
	_, status = sender.UpdateEmailSettings(env.SMTPSettings("not-the-smtp-password"))
	require.Equal(t, http.StatusOK, status)
	env.SMTP.Reset()

	resp, status := sender.SendTestEmail()

	require.Equal(t, http.StatusBadGateway, status, "body: %s", resp)
	var body gen.ErrorResponse
	require.NoError(t, json.Unmarshal(resp, &body))
	assert.Contains(t, body.Message, "535 5.7.8 Authentication credentials invalid")
	assert.Empty(t, env.SMTP.Messages())
	after, _ := emailSettings(t, sender)
	require.NotNil(t, after.LastError)
	assert.True(t, strings.Contains(*after.LastError, "535"), *after.LastError)
	assert.Equal(t, before.LastSentAt, after.LastSentAt, "a failure keeps when the last send succeeded")
}

func TestEmail_TestSendNeedsTheCallersEmailAddress(t *testing.T) {
	_, status := client.UpdateEmailSettings(env.SMTPSettings(testharness.SMTPPassword))
	require.Equal(t, http.StatusOK, status)

	// The harness admin has no email address.
	resp, status := client.SendTestEmail()

	assert.Equal(t, http.StatusBadRequest, status, "body: %s", resp)
}

func TestEmail_SettingsNeedManageEmail(t *testing.T) {
	viewer := emailUser(t, "email-viewer", "email-viewer@example.com", "viewer")

	_, status := viewer.GetEmailSettings()
	assert.Equal(t, http.StatusForbidden, status)
	_, status = viewer.UpdateEmailSettings(env.SMTPSettings("x"))
	assert.Equal(t, http.StatusForbidden, status)
	_, status = viewer.SendTestEmail()
	assert.Equal(t, http.StatusForbidden, status)
}
