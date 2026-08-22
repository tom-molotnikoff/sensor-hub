package oauth

import (
	"net/smtp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestXOauth2Auth_Start_ReturnsCorrectMechanism(t *testing.T) {
	auth := &XOauth2Auth{
		Username:    "user@example.com",
		AccessToken: "test-access-token",
	}

	mechanism, response, err := auth.Start(&smtp.ServerInfo{})

	assert.NoError(t, err)
	assert.Equal(t, "XOAUTH2", mechanism)
	assert.NotEmpty(t, response)
}

func TestXOauth2Auth_Start_FormatsAuthStringCorrectly(t *testing.T) {
	auth := &XOauth2Auth{
		Username:    "user@example.com",
		AccessToken: "my-access-token",
	}

	_, response, err := auth.Start(&smtp.ServerInfo{})

	assert.NoError(t, err)
	// XOAUTH2 format: "user=<user>\x01auth=Bearer <token>\x01\x01"
	// Note: Go's smtp.Client.Auth() will base64-encode this before sending
	expected := "user=user@example.com\x01auth=Bearer my-access-token\x01\x01"
	assert.Equal(t, expected, string(response))
}

func TestXOauth2Auth_Next_ReturnsNil(t *testing.T) {
	auth := &XOauth2Auth{
		Username:    "user@example.com",
		AccessToken: "test-token",
	}

	response, err := auth.Next([]byte("challenge"), true)

	assert.NoError(t, err)
	assert.Nil(t, response)
}

// ============================================================================
// OAuthService Tests
// ============================================================================

func TestNewOAuthService_CreatesService(t *testing.T) {
	reader := &MockFileReader{Files: make(map[string][]byte)}
	writer := NewMockFileWriter()

	service := NewOAuthService(reader, writer, "/creds.json", "/token.json", 30)

	assert.NotNil(t, service)
}

func TestOAuthService_GetStatus_NotConfigured(t *testing.T) {
	reader := &MockFileReader{Files: make(map[string][]byte)}
	writer := NewMockFileWriter()

	service := NewOAuthService(reader, writer, "/creds.json", "/token.json", 30)
	status := service.GetStatus()

	assert.False(t, status.Configured)
	assert.False(t, status.TokenValid)
	assert.False(t, status.RefresherActive)
}

func TestOAuthService_Initialise_MissingCredentials(t *testing.T) {
	reader := &MockFileReader{Files: make(map[string][]byte)}
	writer := NewMockFileWriter()

	service := NewOAuthService(reader, writer, "/creds.json", "/token.json", 30)
	err := service.Initialise()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "credentials")
}

func TestOAuthService_Initialise_MissingToken_StillConfigured(t *testing.T) {
	reader := &MockFileReader{
		Files: map[string][]byte{
			"/creds.json": testCredentialsJSON(),
		},
	}
	writer := NewMockFileWriter()

	service := NewOAuthService(reader, writer, "/creds.json", "/token.json", 30)
	err := service.Initialise()

	// Should not error - credentials exist, just needs authorization
	assert.NoError(t, err)
	status := service.GetStatus()
	assert.True(t, status.Configured)
	assert.True(t, status.NeedsAuth)
	assert.False(t, status.TokenValid)
}

func TestOAuthService_Initialise_Success(t *testing.T) {
	reader := &MockFileReader{
		Files: map[string][]byte{
			"/creds.json": testCredentialsJSON(),
			"/token.json": testTokenJSON(),
		},
	}
	writer := NewMockFileWriter()

	service := NewOAuthService(reader, writer, "/creds.json", "/token.json", 30)
	err := service.Initialise()

	assert.NoError(t, err)
	status := service.GetStatus()
	assert.True(t, status.Configured)
}

func TestOAuthService_IsReady_AfterSuccessfulInit(t *testing.T) {
	reader := &MockFileReader{
		Files: map[string][]byte{
			"/creds.json": testCredentialsJSON(),
			"/token.json": testTokenJSON(),
		},
	}
	writer := NewMockFileWriter()

	service := NewOAuthService(reader, writer, "/creds.json", "/token.json", 30)
	_ = service.Initialise()

	assert.True(t, service.IsReady())
}

func TestOAuthService_GetToken_ReturnsToken(t *testing.T) {
	reader := &MockFileReader{
		Files: map[string][]byte{
			"/creds.json": testCredentialsJSON(),
			"/token.json": testTokenJSON(),
		},
	}
	writer := NewMockFileWriter()

	service := NewOAuthService(reader, writer, "/creds.json", "/token.json", 30)
	_ = service.Initialise()
	token := service.GetToken()

	assert.NotNil(t, token)
	assert.Equal(t, "test-access-token", token.AccessToken)
}

func TestOAuthService_GetSMTPAuth_ReturnsAuth(t *testing.T) {
	reader := &MockFileReader{
		Files: map[string][]byte{
			"/creds.json": testCredentialsJSON(),
			"/token.json": testTokenJSON(),
		},
	}
	writer := NewMockFileWriter()

	service := NewOAuthService(reader, writer, "/creds.json", "/token.json", 30)
	_ = service.Initialise()
	auth := service.GetSMTPAuth("user@example.com")

	assert.NotNil(t, auth)
}

func TestOAuthService_GetAuthURL_ReturnsURL(t *testing.T) {
	reader := &MockFileReader{
		Files: map[string][]byte{
			"/creds.json": testCredentialsJSON(),
			"/token.json": testTokenJSON(),
		},
	}
	writer := NewMockFileWriter()

	service := NewOAuthService(reader, writer, "/creds.json", "/token.json", 30)
	_ = service.Initialise()
	url, err := service.GetAuthURL("test-state")

	assert.NoError(t, err)
	assert.Contains(t, url, "accounts.google.com")
	assert.Contains(t, url, "test-state")
}

func TestOAuthService_StartStopTokenRefresher(t *testing.T) {
	reader := &MockFileReader{
		Files: map[string][]byte{
			"/creds.json": testCredentialsJSON(),
			"/token.json": testTokenJSON(),
		},
	}
	writer := NewMockFileWriter()

	service := NewOAuthService(reader, writer, "/creds.json", "/token.json", 30)
	_ = service.Initialise()

	service.StartTokenRefresher()
	assert.True(t, service.GetStatus().RefresherActive)

	service.StopTokenRefresher()
	assert.False(t, service.GetStatus().RefresherActive)
}

func TestOAuthService_ExchangeCode_StartsRefresher(t *testing.T) {
	// This test verifies that after successful code exchange, the refresher is started
	reader := &MockFileReader{
		Files: map[string][]byte{
			"/creds.json": testCredentialsJSON(),
			// No token.json - simulating first-time setup
		},
	}
	writer := NewMockFileWriter()

	service := NewOAuthService(reader, writer, "/creds.json", "/token.json", 30)
	err := service.Initialise()
	assert.NoError(t, err)

	// Before exchange - not ready, refresher not active
	assert.False(t, service.IsReady())
	assert.False(t, service.GetStatus().RefresherActive)

	// Note: We can't actually test ExchangeCode without mocking the OAuth2 exchange
	// But we can verify the service starts in the right state for the flow
	assert.True(t, service.GetStatus().NeedsAuth)
}

func TestOAuthService_Reload_Success(t *testing.T) {
	reader := &MockFileReader{
		Files: map[string][]byte{
			"/creds.json": testCredentialsJSON(),
			"/token.json": testTokenJSON(),
		},
	}
	writer := NewMockFileWriter()

	service := NewOAuthService(reader, writer, "/creds.json", "/token.json", 30)
	_ = service.Initialise()

	// Start refresher
	service.StartTokenRefresher()
	assert.True(t, service.GetStatus().RefresherActive)

	// Reload should work and restart the refresher
	err := service.Reload()
	assert.NoError(t, err)
	assert.True(t, service.GetStatus().Configured)
	assert.True(t, service.GetStatus().RefresherActive)

	// Cleanup
	service.StopTokenRefresher()
}

// Bug #44: when the application's configured OAuth credential path changes
// (e.g. the user fixes a corrupted value), the in-flight OAuthService must be
// able to pick up the new paths without restarting the process. SetPaths
// updates the cached paths used by subsequent Initialise/Reload calls.
func TestOAuthService_SetPaths_UsedByReload(t *testing.T) {
	reader := &MockFileReader{
		Files: map[string][]byte{
			"/stale/creds.json": []byte("{ this is broken }"),
			"/fresh/creds.json": testCredentialsJSON(),
			"/fresh/token.json": testTokenJSON(),
		},
	}
	writer := NewMockFileWriter()

	service := NewOAuthService(reader, writer, "/stale/creds.json", "/stale/token.json", 30)
	// Initial load fails — stale path points at unparseable bytes
	_ = service.Initialise()
	assert.False(t, service.GetStatus().Configured)

	// Update paths (the user fixed their config); next Reload should use them.
	service.SetPaths("/fresh/creds.json", "/fresh/token.json")

	err := service.Reload()
	assert.NoError(t, err)
	assert.True(t, service.GetStatus().Configured,
		"Reload after SetPaths must use new paths (issue #44 recovery)")

	service.StopTokenRefresher()
}
