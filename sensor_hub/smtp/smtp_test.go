package smtp

import (
	"log/slog"
	"testing"

	appProps "example/sensorHub/application_properties"
	"example/sensorHub/oauth"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

func TestNewSMTPNotifier(t *testing.T) {
	notifier := NewSMTPNotifier(slog.Default())
	assert.NotNil(t, notifier)
}

func TestSMTPNotifier_SendNotification_OAuthNotSet(t *testing.T) {
	originalOauthSet := oauth.OauthSet
	defer func() { oauth.OauthSet = originalOauthSet }()

	oauth.OauthSet = false

	notifier := NewSMTPNotifier(slog.Default())
	err := notifier.SendNotification("recipient@example.com", "Test Title", "Test message", "threshold_alert")

	assert.NoError(t, err)
}

func TestSMTPNotifier_SendNotification_WithOAuth(t *testing.T) {
	originalOauthSet := oauth.OauthSet
	originalToken := oauth.OauthToken
	originalConfig := appProps.AppConfig()
	defer func() {
		oauth.OauthSet = originalOauthSet
		oauth.OauthToken = originalToken
		appProps.SetAppConfig(originalConfig)
	}()

	oauth.OauthSet = true
	oauth.OauthToken = &oauth2.Token{
		AccessToken: "test-token",
	}
	appProps.SetAppConfig(&appProps.ApplicationConfiguration{
		SMTPUser: "test@example.com",
	})

	notifier := NewSMTPNotifier(slog.Default())
	err := notifier.SendNotification("recipient@example.com", "Alert: TestSensor", "value 35.00 is above threshold", "threshold_alert")

	// Will fail because we're not actually connected to SMTP
	assert.Error(t, err)
}
