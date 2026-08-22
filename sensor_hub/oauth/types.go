package oauth

import (
	"net/smtp"
	"time"

	"golang.org/x/oauth2"
)

// OAuthStatus represents the current state of OAuth authentication
type OAuthStatus struct {
	Configured      bool      `json:"configured"`  // true if credentials.json exists and is valid
	NeedsAuth       bool      `json:"needs_auth"`  // true if configured but no token (needs authorization)
	TokenValid      bool      `json:"token_valid"` // true if token exists and is not expired
	TokenExpiry     time.Time `json:"token_expiry,omitempty"`
	RefresherActive bool      `json:"refresher_active"`
	LastRefreshAt   time.Time `json:"last_refresh_at,omitempty"`
	LastError       string    `json:"last_error,omitempty"`
}

// OAuthServiceInterface defines the contract for OAuth operations
type OAuthServiceInterface interface {
	// Initialise loads credentials and token from configured paths
	Initialise() error

	// GetStatus returns the current OAuth status
	GetStatus() OAuthStatus

	// IsReady returns true if OAuth is configured and token is valid
	IsReady() bool

	// GetToken returns the current OAuth token (may be nil)
	GetToken() *oauth2.Token

	// GetSMTPAuth returns an smtp.Auth for use with Gmail SMTP
	GetSMTPAuth(username string) smtp.Auth

	// StartTokenRefresher starts the background token refresh goroutine
	StartTokenRefresher()

	// StopTokenRefresher stops the background token refresh goroutine
	StopTokenRefresher()

	// GetAuthURL returns the Google OAuth consent URL for re-authorization
	GetAuthURL(state string) (string, error)

	// ExchangeCode exchanges an authorization code for a token and persists it
	ExchangeCode(code string) error

	// Reload re-reads credentials and token from disk
	Reload() error
}

// FileReader abstracts file reading for testability
type FileReader interface {
	ReadFile(path string) ([]byte, error)
}

// FileWriter abstracts file writing for testability
type FileWriter interface {
	WriteFile(path string, data []byte, perm uint32) error
}
