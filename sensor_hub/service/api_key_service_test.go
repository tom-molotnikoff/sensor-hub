package service

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupApiKeyService() (*ApiKeyService, *MockApiKeyRepository, *MockUserRepository, *MockRoleRepository) {
	apiKeyRepo := new(MockApiKeyRepository)
	userRepo := new(MockUserRepository)
	roleRepo := new(MockRoleRepository)
	svc := NewApiKeyService(apiKeyRepo, userRepo, roleRepo, slog.Default())
	return svc, apiKeyRepo, userRepo, roleRepo
}

// ============================================================================
// CreateApiKey tests
// ============================================================================

func TestApiKeyService_CreateApiKey_Success(t *testing.T) {
	svc, apiKeyRepo, _, _ := setupApiKeyService()

	apiKeyRepo.On("CreateApiKey", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), 1, (*time.Time)(nil)).
		Return(int64(1), nil)

	fullKey, err := svc.CreateApiKey(context.Background(), "test-key", 1, nil)

	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(fullKey, "shk_"), "key should start with shk_ prefix")
	assert.Len(t, fullKey, 68, "shk_ (4) + 64 hex chars = 68")
	apiKeyRepo.AssertExpectations(t)
}

func TestApiKeyService_CreateApiKey_WithExpiry(t *testing.T) {
	svc, apiKeyRepo, _, _ := setupApiKeyService()

	expiry := time.Now().Add(24 * time.Hour)
	apiKeyRepo.On("CreateApiKey", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), 1, &expiry).
		Return(int64(2), nil)

	fullKey, err := svc.CreateApiKey(context.Background(), "test-key", 1, &expiry)

	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(fullKey, "shk_"))
	apiKeyRepo.AssertExpectations(t)
}

func TestApiKeyService_CreateApiKey_UniqueKeys(t *testing.T) {
	svc, apiKeyRepo, _, _ := setupApiKeyService()

	apiKeyRepo.On("CreateApiKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, 1, (*time.Time)(nil)).
		Return(int64(1), nil)

	key1, _ := svc.CreateApiKey(context.Background(), "key-1", 1, nil)
	key2, _ := svc.CreateApiKey(context.Background(), "key-2", 1, nil)

	assert.NotEqual(t, key1, key2, "generated keys should be unique")
}

// ============================================================================
// ValidateApiKey tests
// ============================================================================

func TestApiKeyService_ValidateApiKey_Success(t *testing.T) {
	svc, apiKeyRepo, userRepo, roleRepo := setupApiKeyService()

	apiKey := &database.ApiKey{Id: 1, Name: "test", UserId: 5}
	user := &gen.User{Id: 5, Username: "testuser"}

	apiKeyRepo.On("GetApiKeyByHash", mock.Anything, mock.AnythingOfType("string")).Return(apiKey, nil)
	userRepo.On("GetUserById", mock.Anything, 5).Return(user, nil)
	roleRepo.On("GetPermissionsForUser", mock.Anything, 5).Return([]string{"view_sensors", "manage_sensors"}, nil)
	apiKeyRepo.On("UpdateLastUsed", mock.Anything, 1).Return(nil)

	result, err := svc.ValidateApiKey(context.Background(), "shk_abc123")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "testuser", result.Username)
	assert.Contains(t, result.Permissions, "view_sensors")
	assert.Contains(t, result.Permissions, "manage_sensors")
	apiKeyRepo.AssertCalled(t, "GetApiKeyByHash", mock.Anything, mock.AnythingOfType("string"))
	userRepo.AssertCalled(t, "GetUserById", mock.Anything, 5)
}

func TestApiKeyService_ValidateApiKey_NotFound(t *testing.T) {
	svc, apiKeyRepo, _, _ := setupApiKeyService()

	apiKeyRepo.On("GetApiKeyByHash", mock.Anything, mock.AnythingOfType("string")).Return(nil, nil)

	result, err := svc.ValidateApiKey(context.Background(), "shk_invalid")

	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestApiKeyService_ValidateApiKey_UserNotFound(t *testing.T) {
	svc, apiKeyRepo, userRepo, _ := setupApiKeyService()

	apiKey := &database.ApiKey{Id: 1, Name: "test", UserId: 999}
	apiKeyRepo.On("GetApiKeyByHash", mock.Anything, mock.AnythingOfType("string")).Return(apiKey, nil)
	userRepo.On("GetUserById", mock.Anything, 999).Return(nil, nil)

	result, err := svc.ValidateApiKey(context.Background(), "shk_abc123")

	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestApiKeyService_ValidateApiKey_DisabledUser(t *testing.T) {
	svc, apiKeyRepo, userRepo, _ := setupApiKeyService()

	apiKey := &database.ApiKey{Id: 1, Name: "test", UserId: 5}
	apiKeyRepo.On("GetApiKeyByHash", mock.Anything, mock.AnythingOfType("string")).Return(apiKey, nil)
	userRepo.On("GetUserById", mock.Anything, 5).Return(&gen.User{Id: 5, Username: "gone", Disabled: true}, nil)

	result, err := svc.ValidateApiKey(context.Background(), "shk_abc123")

	assert.NoError(t, err)
	assert.Nil(t, result, "a disabled user's key does not authenticate")
	apiKeyRepo.AssertNotCalled(t, "UpdateLastUsed", mock.Anything, mock.Anything)
}

// ============================================================================
// Revoke, delete and update expiry: owner scope
// ============================================================================

func TestApiKeyService_KeyWritesAreScopedToTheCallerWithoutManageUsers(t *testing.T) {
	svc, apiKeyRepo, _, _ := setupApiKeyService()
	caller := &gen.User{Id: 5, Permissions: []string{"manage_api_keys"}}
	expiry := time.Now().Add(time.Hour)
	owner := 5

	apiKeyRepo.On("RevokeApiKey", mock.Anything, 1, &owner).Return(nil)
	apiKeyRepo.On("DeleteApiKey", mock.Anything, 1, &owner).Return(nil)
	apiKeyRepo.On("UpdateApiKeyExpiry", mock.Anything, 1, &owner, &expiry).Return(nil)

	assert.NoError(t, svc.RevokeApiKey(context.Background(), 1, caller))
	assert.NoError(t, svc.DeleteApiKey(context.Background(), 1, caller))
	assert.NoError(t, svc.UpdateApiKeyExpiry(context.Background(), 1, caller, &expiry))
	apiKeyRepo.AssertExpectations(t)
}

func TestApiKeyService_KeyWritesReachAnyKeyWithManageUsers(t *testing.T) {
	svc, apiKeyRepo, _, _ := setupApiKeyService()
	caller := &gen.User{Id: 1, Permissions: []string{"manage_api_keys", "manage_users"}}
	expiry := time.Now().Add(time.Hour)
	var anyOwner *int

	apiKeyRepo.On("RevokeApiKey", mock.Anything, 9, anyOwner).Return(nil)
	apiKeyRepo.On("DeleteApiKey", mock.Anything, 9, anyOwner).Return(nil)
	apiKeyRepo.On("UpdateApiKeyExpiry", mock.Anything, 9, anyOwner, &expiry).Return(nil)

	assert.NoError(t, svc.RevokeApiKey(context.Background(), 9, caller))
	assert.NoError(t, svc.DeleteApiKey(context.Background(), 9, caller))
	assert.NoError(t, svc.UpdateApiKeyExpiry(context.Background(), 9, caller, &expiry))
	apiKeyRepo.AssertExpectations(t)
}

// ============================================================================
// ListApiKeysForUser tests
// ============================================================================

func TestApiKeyService_ListApiKeysForUser_Success(t *testing.T) {
	svc, apiKeyRepo, _, _ := setupApiKeyService()

	keys := []database.ApiKey{
		{Id: 1, Name: "key-1", KeyPrefix: "shk_aaaa"},
		{Id: 2, Name: "key-2", KeyPrefix: "shk_bbbb"},
	}
	apiKeyRepo.On("ListApiKeysForUser", mock.Anything, 5).Return(keys, nil)

	result, err := svc.ListApiKeysForUser(context.Background(), 5)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	apiKeyRepo.AssertExpectations(t)
}
