package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/smtp"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// OAuthService manages OAuth tokens and provides SMTP authentication
type OAuthService struct {
	reader              FileReader
	writer              FileWriter
	credentialsPath     string
	tokenPath           string
	refreshIntervalMins int

	mu              sync.RWMutex
	config          *oauth2.Config
	token           *oauth2.Token
	configured      bool
	lastRefreshAt   time.Time
	lastError       string
	refresherActive bool
	stopChan        chan struct{}
}

// SetPaths updates the credential/token file paths the service reads on the
// next Initialise or Reload. Used to recover from a stale path captured at
// startup when the application configuration is later corrected (issue #44).
func (s *OAuthService) SetPaths(credentialsPath, tokenPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.credentialsPath = credentialsPath
	s.tokenPath = tokenPath
}

// paths returns the currently configured credential and token paths under the
// service's read lock so SetPaths is safe against concurrent Initialise.
func (s *OAuthService) paths() (string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.credentialsPath, s.tokenPath
}

// tokenPathLocked returns the current token path; safe for concurrent SetPaths.
func (s *OAuthService) tokenPathLocked() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tokenPath
}

// NewOAuthService creates a new OAuthService
func NewOAuthService(reader FileReader, writer FileWriter, credentialsPath, tokenPath string, refreshIntervalMins int) *OAuthService {
	return &OAuthService{
		reader:              reader,
		writer:              writer,
		credentialsPath:     credentialsPath,
		tokenPath:           tokenPath,
		refreshIntervalMins: refreshIntervalMins,
	}
}

// Initialise loads credentials and optionally token from configured paths
// If credentials.json exists but token.json doesn't, OAuth is configured but needs authorization
func (s *OAuthService) Initialise() error {
	credPath, tokenPath := s.paths()
	credBytes, err := s.reader.ReadFile(credPath)
	if err != nil {
		s.mu.Lock()
		s.lastError = fmt.Sprintf("unable to read credentials: %v", err)
		s.mu.Unlock()
		return fmt.Errorf("unable to read credentials.json: %w", err)
	}

	config, err := google.ConfigFromJSON(credBytes, "https://mail.google.com/")
	if err != nil {
		s.mu.Lock()
		s.lastError = fmt.Sprintf("unable to parse credentials: %v", err)
		s.mu.Unlock()
		return fmt.Errorf("unable to parse credentials.json: %w", err)
	}

	// Force out-of-band redirect URI for manual code entry (works with non-TLD hosts)
	config.RedirectURL = "urn:ietf:wg:oauth:2.0:oob"

	s.mu.Lock()
	s.config = config
	s.configured = true
	s.mu.Unlock()

	// Try to load token - if it doesn't exist, that's okay (needs authorization)
	tokenBytes, err := s.reader.ReadFile(tokenPath)
	if err != nil {
		// Token doesn't exist yet - user needs to authorize
		s.mu.Lock()
		s.lastError = ""
		s.mu.Unlock()
		return nil
	}

	var token oauth2.Token
	if err := json.Unmarshal(tokenBytes, &token); err != nil {
		s.mu.Lock()
		s.lastError = fmt.Sprintf("unable to parse token: %v", err)
		s.mu.Unlock()
		// Token file exists but is invalid - still configured, just needs re-auth
		return nil
	}

	s.mu.Lock()
	s.token = &token
	s.lastError = ""
	s.mu.Unlock()

	return nil
}

// GetStatus returns the current OAuth status
func (s *OAuthService) GetStatus() OAuthStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := OAuthStatus{
		Configured:      s.configured,
		NeedsAuth:       s.configured && s.token == nil,
		RefresherActive: s.refresherActive,
		LastRefreshAt:   s.lastRefreshAt.UTC(),
		LastError:       s.lastError,
	}

	if s.token != nil {
		status.TokenValid = s.token.Valid()
		status.TokenExpiry = s.token.Expiry.UTC()
	}

	return status
}

// IsReady returns true if OAuth is configured and token is valid
func (s *OAuthService) IsReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.configured && s.token != nil
}

// GetToken returns the current OAuth token
func (s *OAuthService) GetToken() *oauth2.Token {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.token
}

// GetSMTPAuth returns an smtp.Auth for use with Gmail SMTP
func (s *OAuthService) GetSMTPAuth(username string) smtp.Auth {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.token == nil {
		return nil
	}

	return &XOauth2Auth{
		Username:    username,
		AccessToken: s.token.AccessToken,
	}
}

// StartTokenRefresher starts the background token refresh goroutine
func (s *OAuthService) StartTokenRefresher() {
	s.mu.Lock()
	if s.refresherActive {
		s.mu.Unlock()
		return
	}
	s.stopChan = make(chan struct{})
	s.refresherActive = true
	s.mu.Unlock()

	go s.refreshLoop()
}

// StopTokenRefresher stops the background token refresh goroutine
func (s *OAuthService) StopTokenRefresher() {
	s.mu.Lock()
	if !s.refresherActive {
		s.mu.Unlock()
		return
	}
	close(s.stopChan)
	s.refresherActive = false
	s.mu.Unlock()
}

func (s *OAuthService) refreshLoop() {
	ticker := time.NewTicker(time.Duration(s.refreshIntervalMins) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.refreshToken()
		}
	}
}

func (s *OAuthService) refreshToken() {
	s.mu.RLock()
	config := s.config
	token := s.token
	s.mu.RUnlock()

	if config == nil || token == nil {
		return
	}

	tokenSource := config.TokenSource(context.Background(), token)
	newToken, err := tokenSource.Token()
	if err != nil {
		s.mu.Lock()
		s.lastError = fmt.Sprintf("token refresh failed: %v", err)
		s.mu.Unlock()
		return
	}

	s.mu.Lock()
	s.token = newToken
	s.lastRefreshAt = time.Now()
	s.lastError = ""
	s.mu.Unlock()

	// Update legacy globals for smtp package compatibility
	UpdateLegacyGlobals()

	// Persist token
	tokenBytes, err := json.Marshal(newToken)
	if err != nil {
		s.mu.Lock()
		s.lastError = fmt.Sprintf("unable to marshal token: %v", err)
		s.mu.Unlock()
		return
	}

	if err := s.writer.WriteFile(s.tokenPathLocked(), tokenBytes, 0600); err != nil {
		s.mu.Lock()
		s.lastError = fmt.Sprintf("unable to write token: %v", err)
		s.mu.Unlock()
	}
}

// GetAuthURL returns the Google OAuth consent URL for re-authorization
func (s *OAuthService) GetAuthURL(state string) (string, error) {
	s.mu.RLock()
	config := s.config
	s.mu.RUnlock()

	if config == nil {
		return "", fmt.Errorf("OAuth not configured")
	}

	return config.AuthCodeURL(state, oauth2.AccessTypeOffline), nil
}

// ExchangeCode exchanges an authorization code for a token and persists it
func (s *OAuthService) ExchangeCode(code string) error {
	s.mu.RLock()
	config := s.config
	s.mu.RUnlock()

	if config == nil {
		return fmt.Errorf("OAuth not configured")
	}

	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		s.mu.Lock()
		s.lastError = fmt.Sprintf("code exchange failed: %v", err)
		s.mu.Unlock()
		return fmt.Errorf("unable to exchange code: %w", err)
	}

	s.mu.Lock()
	s.token = token
	s.lastError = ""
	s.mu.Unlock()

	// Persist token
	tokenBytes, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("unable to marshal token: %w", err)
	}

	if err := s.writer.WriteFile(s.tokenPathLocked(), tokenBytes, 0600); err != nil {
		return fmt.Errorf("unable to write token: %w", err)
	}

	// Update legacy globals for smtp package compatibility
	UpdateLegacyGlobals()

	// Start the token refresher if not already running
	s.StartTokenRefresher()

	return nil
}

// Reload re-reads credentials and token from disk without restarting the service
func (s *OAuthService) Reload() error {
	// Stop refresher if running
	wasActive := s.GetStatus().RefresherActive
	if wasActive {
		s.StopTokenRefresher()
	}

	// Reset state
	s.mu.Lock()
	s.config = nil
	s.token = nil
	s.configured = false
	s.lastError = ""
	s.mu.Unlock()

	// Re-initialize
	err := s.Initialise()

	// Update legacy globals for smtp package compatibility
	UpdateLegacyGlobals()

	// Restart refresher if it was active and we have a token
	if wasActive && s.IsReady() {
		s.StartTokenRefresher()
	}

	return err
}
