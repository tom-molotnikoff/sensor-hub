package oauth

import (
	"fmt"
	"net/smtp"
	"path/filepath"

	appProps "example/sensorHub/application_properties"

	"golang.org/x/oauth2"
)

// Global OAuth service instance
var oauthService *OAuthService

// Legacy compatibility - these are kept for backward compatibility with smtp package
var OauthToken *oauth2.Token
var OauthSet = false

// UpdateLegacyGlobals updates the legacy global variables from the current service state
// This must be called after token changes (ExchangeCode, Reload) to keep smtp package working
func UpdateLegacyGlobals() {
	if oauthService != nil {
		OauthToken = oauthService.GetToken()
		OauthSet = oauthService.IsReady()
	}
}

// XOauth2Auth implements smtp.Auth for XOAUTH2 authentication
type XOauth2Auth struct {
	Username    string
	AccessToken string
}

// Start initiates the XOAUTH2 authentication
// Note: Go's smtp.Client.Auth() will base64-encode this response,
// so we return raw bytes here, not base64-encoded
func (a *XOauth2Auth) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	// XOAUTH2 format: "user=<user>\x01auth=Bearer <token>\x01\x01"
	authString := fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", a.Username, a.AccessToken)
	return "XOAUTH2", []byte(authString), nil
}

// Next handles additional authentication challenges (not used in XOAUTH2)
func (a *XOauth2Auth) Next(_ []byte, _ bool) ([]byte, error) {
	return nil, nil
}

// GetService returns the global OAuth service instance
func GetService() *OAuthService {
	return oauthService
}

// InitialiseOauth initializes the global OAuth service using application config
func InitialiseOauth() error {
	cfg := appProps.AppConfig()
	if cfg == nil {
		return fmt.Errorf("application config not initialized")
	}

	credPath := cfg.ResolvedOAuthCredentialsPath()
	if credPath == "" {
		credPath = filepath.Join(appProps.GetConfigDir(), "credentials.json")
	}
	tokenPath := cfg.ResolvedOAuthTokenPath()
	if tokenPath == "" {
		tokenPath = filepath.Join(appProps.GetConfigDir(), "token.json")
	}
	refreshInterval := cfg.OAuthTokenRefreshIntervalMinutes
	if refreshInterval <= 0 {
		refreshInterval = 30
	}

	oauthService = NewOAuthService(
		&OSFileReader{},
		&OSFileWriter{},
		credPath,
		tokenPath,
		refreshInterval,
	)

	if err := oauthService.Initialise(); err != nil {
		return err
	}

	// Update legacy globals for backward compatibility
	OauthToken = oauthService.GetToken()
	OauthSet = oauthService.IsReady()

	// Start background refresher
	oauthService.StartTokenRefresher()

	return nil
}

// StopOauth stops the token refresher
func StopOauth() {
	if oauthService != nil {
		oauthService.StopTokenRefresher()
	}
}
