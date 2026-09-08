package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ============================================================================
// Test helpers
// ============================================================================

func setupAuthService() (*AuthService, *MockUserRepository, *MockSessionRepository, *MockFailedLoginRepository) {
	userRepo := new(MockUserRepository)
	sessionRepo := new(MockSessionRepository)
	failedRepo := new(MockFailedLoginRepository)

	service := NewAuthService(userRepo, sessionRepo, failedRepo, slog.Default())
	return service, userRepo, sessionRepo, failedRepo
}

func setupTestConfig() func() {
	origConfig := appProps.AppConfig()
	appProps.SetAppConfig(&appProps.ApplicationConfiguration{
		AuthBcryptCost:                4, // Low cost for fast tests
		AuthSessionTTLMinutes:         60,
		AuthLoginBackoffWindowMinutes: 15,
		AuthLoginBackoffThreshold:     5,
		AuthLoginBackoffBaseSeconds:   2,
		AuthLoginBackoffMaxSeconds:    300,
	})
	return func() { appProps.SetAppConfig(origConfig) }
}

func resetBlockers() {
	ipBlocker = newSimpleBlocker()
	userBlocker = newSimpleBlocker()
}

// ============================================================================
// Login tests
// ============================================================================

func TestAuthService_Login_Success(t *testing.T) {
	defer setupTestConfig()()
	resetBlockers()

	service, userRepo, sessionRepo, failedRepo := setupAuthService()

	// bcrypt hash of "password123" with cost 4
	passwordHash := "$2a$04$8/TZfgezGK2PM2Eoni4P6O/nUDjGtd4rLPMHqQ7g4n3DATqIDPRxq"
	user := &gen.User{Id: 1, Username: "testuser", Disabled: false, MustChangePassword: false}

	failedRepo.On("CountRecentFailedAttemptsByUsername", mock.Anything, "testuser", mock.Anything).Return(0, nil)
	failedRepo.On("CountRecentFailedAttemptsByIP", mock.Anything, "192.168.1.1", mock.Anything).Return(0, nil)
	userRepo.On("GetUserByUsername", mock.Anything, "testuser").Return(user, passwordHash, nil)
	sessionRepo.On("CreateSession", mock.Anything, 1, mock.Anything, mock.Anything, "192.168.1.1", "TestAgent").Return("csrf-token", nil)
	failedRepo.On("DeleteRecentFailedAttemptsByIP", mock.Anything, "192.168.1.1", mock.Anything).Return(nil)

	token, csrf, mustChange, err := service.Login(context.Background(), "testuser", "password123", "192.168.1.1", "TestAgent")

	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, "csrf-token", csrf)
	assert.False(t, mustChange)
	userRepo.AssertExpectations(t)
	sessionRepo.AssertExpectations(t)
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	defer setupTestConfig()()
	resetBlockers()

	service, userRepo, _, failedRepo := setupAuthService()

	failedRepo.On("CountRecentFailedAttemptsByUsername", mock.Anything, "unknown", mock.Anything).Return(0, nil)
	failedRepo.On("CountRecentFailedAttemptsByIP", mock.Anything, "192.168.1.1", mock.Anything).Return(0, nil)
	userRepo.On("GetUserByUsername", mock.Anything, "unknown").Return(nil, "", nil)
	failedRepo.On("RecordFailedAttempt", mock.Anything, "unknown", (*int)(nil), "192.168.1.1", "no_such_user").Return(nil)

	token, csrf, _, err := service.Login(context.Background(), "unknown", "password", "192.168.1.1", "TestAgent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid credentials")
	assert.Empty(t, token)
	assert.Empty(t, csrf)
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	defer setupTestConfig()()
	resetBlockers()

	service, userRepo, _, failedRepo := setupAuthService()

	passwordHash := "$2a$04$8/TZfgezGK2PM2Eoni4P6O/nUDjGtd4rLPMHqQ7g4n3DATqIDPRxq"
	user := &gen.User{Id: 1, Username: "testuser", Disabled: false}
	userId := 1

	failedRepo.On("CountRecentFailedAttemptsByUsername", mock.Anything, "testuser", mock.Anything).Return(0, nil)
	failedRepo.On("CountRecentFailedAttemptsByIP", mock.Anything, "192.168.1.1", mock.Anything).Return(0, nil)
	userRepo.On("GetUserByUsername", mock.Anything, "testuser").Return(user, passwordHash, nil)
	failedRepo.On("RecordFailedAttempt", mock.Anything, "testuser", &userId, "192.168.1.1", "bad_password").Return(nil)

	token, _, _, err := service.Login(context.Background(), "testuser", "wrongpassword", "192.168.1.1", "TestAgent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid credentials")
	assert.Empty(t, token)
}

func TestAuthService_Login_DisabledAccount(t *testing.T) {
	defer setupTestConfig()()
	resetBlockers()

	service, userRepo, _, failedRepo := setupAuthService()

	passwordHash := "$2a$04$8/TZfgezGK2PM2Eoni4P6O/nUDjGtd4rLPMHqQ7g4n3DATqIDPRxq"
	user := &gen.User{Id: 1, Username: "testuser", Disabled: true}

	failedRepo.On("CountRecentFailedAttemptsByUsername", mock.Anything, "testuser", mock.Anything).Return(0, nil)
	failedRepo.On("CountRecentFailedAttemptsByIP", mock.Anything, "192.168.1.1", mock.Anything).Return(0, nil)
	userRepo.On("GetUserByUsername", mock.Anything, "testuser").Return(user, passwordHash, nil)

	token, _, _, err := service.Login(context.Background(), "testuser", "password123", "192.168.1.1", "TestAgent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "account disabled")
	assert.Empty(t, token)
}

func TestAuthService_Login_MustChangePassword(t *testing.T) {
	defer setupTestConfig()()
	resetBlockers()

	service, userRepo, sessionRepo, failedRepo := setupAuthService()

	passwordHash := "$2a$04$8/TZfgezGK2PM2Eoni4P6O/nUDjGtd4rLPMHqQ7g4n3DATqIDPRxq"
	user := &gen.User{Id: 1, Username: "testuser", Disabled: false, MustChangePassword: true}

	failedRepo.On("CountRecentFailedAttemptsByUsername", mock.Anything, "testuser", mock.Anything).Return(0, nil)
	failedRepo.On("CountRecentFailedAttemptsByIP", mock.Anything, "192.168.1.1", mock.Anything).Return(0, nil)
	userRepo.On("GetUserByUsername", mock.Anything, "testuser").Return(user, passwordHash, nil)
	sessionRepo.On("CreateSession", mock.Anything, 1, mock.Anything, mock.Anything, "192.168.1.1", "TestAgent").Return("csrf-token", nil)
	failedRepo.On("DeleteRecentFailedAttemptsByIP", mock.Anything, "192.168.1.1", mock.Anything).Return(nil)

	_, _, mustChange, err := service.Login(context.Background(), "testuser", "password123", "192.168.1.1", "TestAgent")

	assert.NoError(t, err)
	assert.True(t, mustChange)
}

func TestAuthService_Login_TooManyAttempts(t *testing.T) {
	defer setupTestConfig()()
	resetBlockers()

	service, _, _, failedRepo := setupAuthService()

	failedRepo.On("CountRecentFailedAttemptsByUsername", mock.Anything, "testuser", mock.Anything).Return(10, nil)
	failedRepo.On("CountRecentFailedAttemptsByIP", mock.Anything, "192.168.1.1", mock.Anything).Return(10, nil)

	_, _, _, err := service.Login(context.Background(), "testuser", "password", "192.168.1.1", "TestAgent")

	assert.Error(t, err)
	var tooManyErr *TooManyAttemptsError
	assert.True(t, errors.As(err, &tooManyErr))
	assert.Greater(t, tooManyErr.RetryAfterSeconds, 0)
}

func TestAuthService_Login_DBError(t *testing.T) {
	defer setupTestConfig()()
	resetBlockers()

	service, userRepo, _, failedRepo := setupAuthService()

	failedRepo.On("CountRecentFailedAttemptsByUsername", mock.Anything, "testuser", mock.Anything).Return(0, nil)
	failedRepo.On("CountRecentFailedAttemptsByIP", mock.Anything, "192.168.1.1", mock.Anything).Return(0, nil)
	userRepo.On("GetUserByUsername", mock.Anything, "testuser").Return(nil, "", errors.New("database error"))

	_, _, _, err := service.Login(context.Background(), "testuser", "password", "192.168.1.1", "TestAgent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
}

// ============================================================================
// ValidateSession tests
// ============================================================================

func TestAuthService_ValidateSession_Success(t *testing.T) {
	service, _, sessionRepo, _ := setupAuthService()

	user := &gen.User{Id: 1, Username: "testuser", Permissions: []string{"read", "write"}}
	sessionRepo.On("GetAuthenticatedUserByToken", mock.Anything, "valid-token").Return(user, time.Now(), nil)

	result, err := service.ValidateSession(context.Background(), "valid-token")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "testuser", result.Username)
	assert.Contains(t, result.Permissions, "read")
	sessionRepo.AssertNotCalled(t, "TouchSession", mock.Anything, mock.Anything)
	sessionRepo.AssertExpectations(t)
}

func TestAuthService_ValidateSession_TouchesStaleSession(t *testing.T) {
	service, _, sessionRepo, _ := setupAuthService()

	user := &gen.User{Id: 1, Username: "testuser"}
	sessionRepo.On("GetAuthenticatedUserByToken", mock.Anything, "valid-token").Return(user, time.Now().Add(-61*time.Second), nil)
	sessionRepo.On("TouchSession", mock.Anything, "valid-token").Return(nil)

	result, err := service.ValidateSession(context.Background(), "valid-token")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	sessionRepo.AssertExpectations(t)
}

func TestAuthService_ValidateSession_DoesNotTouchFreshSession(t *testing.T) {
	service, _, sessionRepo, _ := setupAuthService()

	user := &gen.User{Id: 1, Username: "testuser"}
	sessionRepo.On("GetAuthenticatedUserByToken", mock.Anything, "valid-token").Return(user, time.Now().Add(-59*time.Second), nil)

	result, err := service.ValidateSession(context.Background(), "valid-token")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	sessionRepo.AssertNotCalled(t, "TouchSession", mock.Anything, mock.Anything)
	sessionRepo.AssertExpectations(t)
}

func TestAuthService_ValidateSession_TouchFailureStillAuthenticates(t *testing.T) {
	service, _, sessionRepo, _ := setupAuthService()

	user := &gen.User{Id: 1, Username: "testuser"}
	sessionRepo.On("GetAuthenticatedUserByToken", mock.Anything, "valid-token").Return(user, time.Time{}, nil)
	sessionRepo.On("TouchSession", mock.Anything, "valid-token").Return(errors.New("database error"))

	result, err := service.ValidateSession(context.Background(), "valid-token")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	sessionRepo.AssertExpectations(t)
}

func TestAuthService_ValidateSession_InvalidToken(t *testing.T) {
	service, _, sessionRepo, _ := setupAuthService()

	sessionRepo.On("GetAuthenticatedUserByToken", mock.Anything, "invalid-token").Return(nil, time.Time{}, nil)

	result, err := service.ValidateSession(context.Background(), "invalid-token")

	assert.NoError(t, err)
	assert.Nil(t, result)
	sessionRepo.AssertNotCalled(t, "TouchSession", mock.Anything, mock.Anything)
}

func TestAuthService_ValidateSession_DBError(t *testing.T) {
	service, _, sessionRepo, _ := setupAuthService()

	sessionRepo.On("GetAuthenticatedUserByToken", mock.Anything, "token").Return(nil, time.Time{}, errors.New("database error"))

	result, err := service.ValidateSession(context.Background(), "token")

	assert.Error(t, err)
	assert.Nil(t, result)
}

// ============================================================================
// Logout tests
// ============================================================================

func TestAuthService_Logout_Success(t *testing.T) {
	service, _, sessionRepo, _ := setupAuthService()

	sessionRepo.On("DeleteSessionByToken", mock.Anything, "token").Return(nil)

	err := service.Logout(context.Background(), "token")

	assert.NoError(t, err)
	sessionRepo.AssertExpectations(t)
}

func TestAuthService_Logout_Error(t *testing.T) {
	service, _, sessionRepo, _ := setupAuthService()

	sessionRepo.On("DeleteSessionByToken", mock.Anything, "token").Return(errors.New("database error"))

	err := service.Logout(context.Background(), "token")

	assert.Error(t, err)
}

// ============================================================================
// ChangePassword tests
// ============================================================================

func TestAuthService_ChangePassword_Success(t *testing.T) {
	defer setupTestConfig()()

	service, userRepo, _, _ := setupAuthService()

	userRepo.On("UpdatePassword", mock.Anything, 1, mock.Anything, false).Return(nil)

	err := service.ChangePassword(context.Background(), 1, "newpassword")

	assert.NoError(t, err)
	userRepo.AssertExpectations(t)
}

func TestAuthService_ChangePassword_DBError(t *testing.T) {
	defer setupTestConfig()()

	service, userRepo, _, _ := setupAuthService()

	userRepo.On("UpdatePassword", mock.Anything, 1, mock.Anything, false).Return(errors.New("database error"))

	err := service.ChangePassword(context.Background(), 1, "newpassword")

	assert.Error(t, err)
}

// ============================================================================
// CreateInitialAdminIfNone tests
// ============================================================================

func TestAuthService_CreateInitialAdminIfNone_CreatesAdmin(t *testing.T) {
	defer setupTestConfig()()

	service, userRepo, _, _ := setupAuthService()

	userRepo.On("ListUsers", mock.Anything).Return([]gen.User{}, nil)
	userRepo.On("CreateUser", mock.Anything, mock.Anything, mock.Anything).Return(1, nil)
	userRepo.On("AssignRoleToUser", mock.Anything, 1, RoleAdmin).Return(nil)

	err := service.CreateInitialAdminIfNone(context.Background(), "admin", "password")

	assert.NoError(t, err)
	userRepo.AssertExpectations(t)
}

func TestAuthService_CreateInitialAdminIfNone_SkipsIfUsersExist(t *testing.T) {
	service, userRepo, _, _ := setupAuthService()

	userRepo.On("ListUsers", mock.Anything).Return([]gen.User{{Id: 1, Username: "existing"}}, nil)

	err := service.CreateInitialAdminIfNone(context.Background(), "admin", "password")

	assert.NoError(t, err)
	userRepo.AssertNotCalled(t, "CreateUser")
}

func TestAuthService_CreateInitialAdminIfNone_ListUsersError(t *testing.T) {
	service, userRepo, _, _ := setupAuthService()

	userRepo.On("ListUsers", mock.Anything).Return([]gen.User{}, errors.New("database error"))

	err := service.CreateInitialAdminIfNone(context.Background(), "admin", "password")

	assert.Error(t, err)
}

// ============================================================================
// ListSessionsForUser tests
// ============================================================================

func TestAuthService_ListSessionsForUser_Success(t *testing.T) {
	service, _, sessionRepo, _ := setupAuthService()

	sessions := []database.SessionInfo{
		{Id: 1, CreatedAt: time.Now()},
		{Id: 2, CreatedAt: time.Now()},
	}
	sessionRepo.On("ListSessionsForUser", mock.Anything, 1).Return(sessions, nil)

	result, err := service.ListSessionsForUser(context.Background(), 1)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestAuthService_ListSessionsForUser_Error(t *testing.T) {
	service, _, sessionRepo, _ := setupAuthService()

	sessionRepo.On("ListSessionsForUser", mock.Anything, 1).Return([]database.SessionInfo{}, errors.New("database error"))

	_, err := service.ListSessionsForUser(context.Background(), 1)

	assert.Error(t, err)
}

// ============================================================================
// RevokeSessionById tests
// ============================================================================

func TestAuthService_RevokeSessionById_Success(t *testing.T) {
	service, _, sessionRepo, _ := setupAuthService()

	sessionRepo.On("InsertSessionAudit", mock.Anything, int64(1), (*int)(nil), "revoked", (*string)(nil)).Return(nil)
	sessionRepo.On("RevokeSessionById", mock.Anything, int64(1)).Return(nil)

	err := service.RevokeSessionById(context.Background(), 1)

	assert.NoError(t, err)
	sessionRepo.AssertExpectations(t)
}

func TestAuthService_RevokeSessionByIdWithActor_Success(t *testing.T) {
	service, _, sessionRepo, _ := setupAuthService()

	actorId := 2
	reason := "security concern"
	sessionRepo.On("InsertSessionAudit", mock.Anything, int64(1), &actorId, "revoked", &reason).Return(nil)
	sessionRepo.On("RevokeSessionById", mock.Anything, int64(1)).Return(nil)

	err := service.RevokeSessionByIdWithActor(context.Background(), 1, &actorId, &reason)

	assert.NoError(t, err)
	sessionRepo.AssertExpectations(t)
}

// ============================================================================
// GetCSRFForToken tests
// ============================================================================

func TestAuthService_GetCSRFForToken_Success(t *testing.T) {
	service, _, sessionRepo, _ := setupAuthService()

	sessionRepo.On("GetCSRFForToken", mock.Anything, "token").Return("csrf-value", nil)

	csrf, err := service.GetCSRFForToken(context.Background(), "token")

	assert.NoError(t, err)
	assert.Equal(t, "csrf-value", csrf)
}

func TestAuthService_GetCSRFForToken_Error(t *testing.T) {
	service, _, sessionRepo, _ := setupAuthService()

	sessionRepo.On("GetCSRFForToken", mock.Anything, "token").Return("", errors.New("not found"))

	_, err := service.GetCSRFForToken(context.Background(), "token")

	assert.Error(t, err)
}

// ============================================================================
// GetSessionIdForToken tests
// ============================================================================

func TestAuthService_GetSessionIdForToken_Success(t *testing.T) {
	service, _, sessionRepo, _ := setupAuthService()

	sessionRepo.On("GetSessionIdByToken", mock.Anything, "token").Return(int64(123), nil)

	sessionId, err := service.GetSessionIdForToken(context.Background(), "token")

	assert.NoError(t, err)
	assert.Equal(t, int64(123), sessionId)
}

func TestAuthService_GetSessionIdForToken_Error(t *testing.T) {
	service, _, sessionRepo, _ := setupAuthService()

	sessionRepo.On("GetSessionIdByToken", mock.Anything, "token").Return(int64(0), errors.New("not found"))

	_, err := service.GetSessionIdForToken(context.Background(), "token")

	assert.Error(t, err)
}
