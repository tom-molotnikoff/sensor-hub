package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	appProps "example/sensorHub/application_properties"
	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ============================================================================
// Test helpers
// ============================================================================

func setupUserService() (*UserService, *MockUserRepository) {
	userRepo := new(MockUserRepository)
	service := NewUserService(userRepo, nil, slog.Default())
	return service, userRepo
}

// ============================================================================
// CreateUser tests
// ============================================================================

func TestUserService_CreateUser_Success(t *testing.T) {
	defer setupTestConfig()()

	service, userRepo := setupUserService()

	user := gen.User{Username: "newuser", Email: "test@test.com", Roles: []string{"user"}}

	userRepo.On("CreateUser", mock.Anything, mock.Anything, mock.Anything).Return(1, nil)
	userRepo.On("AssignRoleToUser", mock.Anything, 1, "user").Return(nil)

	id, err := service.CreateUser(context.Background(), user, "password123")

	assert.NoError(t, err)
	assert.Equal(t, 1, id)
	userRepo.AssertExpectations(t)
}

func TestUserService_CreateUser_EmptyPassword(t *testing.T) {
	service, _ := setupUserService()

	user := gen.User{Username: "newuser"}

	id, err := service.CreateUser(context.Background(), user, "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "password cannot be empty")
	assert.Equal(t, 0, id)
}

func TestUserService_CreateUser_MultipleRoles(t *testing.T) {
	defer setupTestConfig()()

	service, userRepo := setupUserService()

	user := gen.User{Username: "admin", Roles: []string{"admin", "user"}}

	userRepo.On("CreateUser", mock.Anything, mock.Anything, mock.Anything).Return(1, nil)
	userRepo.On("AssignRoleToUser", mock.Anything, 1, "admin").Return(nil)
	userRepo.On("AssignRoleToUser", mock.Anything, 1, "user").Return(nil)

	id, err := service.CreateUser(context.Background(), user, "password123")

	assert.NoError(t, err)
	assert.Equal(t, 1, id)
	userRepo.AssertExpectations(t)
}

func TestUserService_CreateUser_DBError(t *testing.T) {
	defer setupTestConfig()()

	service, userRepo := setupUserService()

	user := gen.User{Username: "newuser"}

	userRepo.On("CreateUser", mock.Anything, mock.Anything, mock.Anything).Return(0, errors.New("database error"))

	id, err := service.CreateUser(context.Background(), user, "password123")

	assert.Error(t, err)
	assert.Equal(t, 0, id)
}

func TestUserService_CreateUser_RoleAssignmentError(t *testing.T) {
	defer setupTestConfig()()

	service, userRepo := setupUserService()

	user := gen.User{Username: "newuser", Roles: []string{"admin"}}

	userRepo.On("CreateUser", mock.Anything, mock.Anything, mock.Anything).Return(1, nil)
	userRepo.On("AssignRoleToUser", mock.Anything, 1, "admin").Return(errors.New("role not found"))

	id, err := service.CreateUser(context.Background(), user, "password123")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to assign role")
	assert.Equal(t, 0, id)
}

func TestUserService_CreateUser_SetsMustChangePassword(t *testing.T) {
	defer setupTestConfig()()

	service, userRepo := setupUserService()

	user := gen.User{Username: "newuser", MustChangePassword: false}

	userRepo.On("CreateUser", mock.Anything, mock.MatchedBy(func(u gen.User) bool {
		return u.MustChangePassword == true
	}), mock.Anything).Return(1, nil)

	id, err := service.CreateUser(context.Background(), user, "password123")

	assert.NoError(t, err)
	assert.Equal(t, 1, id)
	userRepo.AssertExpectations(t)
}

// ============================================================================
// ListUsers tests
// ============================================================================

func TestUserService_ListUsers_Success(t *testing.T) {
	service, userRepo := setupUserService()

	users := []gen.User{
		{Id: 1, Username: "user1"},
		{Id: 2, Username: "user2"},
	}
	userRepo.On("ListUsers", mock.Anything).Return(users, nil)

	result, err := service.ListUsers(context.Background())

	assert.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestUserService_ListUsers_Empty(t *testing.T) {
	service, userRepo := setupUserService()

	userRepo.On("ListUsers", mock.Anything).Return([]gen.User{}, nil)

	result, err := service.ListUsers(context.Background())

	assert.NoError(t, err)
	assert.Len(t, result, 0)
}

func TestUserService_ListUsers_Error(t *testing.T) {
	service, userRepo := setupUserService()

	userRepo.On("ListUsers", mock.Anything).Return([]gen.User{}, errors.New("database error"))

	_, err := service.ListUsers(context.Background())

	assert.Error(t, err)
}

// ============================================================================
// GetUserById tests
// ============================================================================

func TestUserService_GetUserById_Success(t *testing.T) {
	service, userRepo := setupUserService()

	user := &gen.User{Id: 1, Username: "testuser"}
	userRepo.On("GetUserById", mock.Anything, 1).Return(user, nil)

	result, err := service.GetUserById(context.Background(), 1)

	assert.NoError(t, err)
	assert.Equal(t, "testuser", result.Username)
}

func TestUserService_GetUserById_NotFound(t *testing.T) {
	service, userRepo := setupUserService()

	userRepo.On("GetUserById", mock.Anything, 999).Return(nil, nil)

	result, err := service.GetUserById(context.Background(), 999)

	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestUserService_GetUserById_Error(t *testing.T) {
	service, userRepo := setupUserService()

	userRepo.On("GetUserById", mock.Anything, 1).Return(nil, errors.New("database error"))

	_, err := service.GetUserById(context.Background(), 1)

	assert.Error(t, err)
}

// ============================================================================
// ChangePassword tests
// ============================================================================

func TestUserService_ChangePassword_Success(t *testing.T) {
	defer setupTestConfig()()

	service, userRepo := setupUserService()

	userRepo.On("UpdatePassword", mock.Anything, 1, mock.Anything, false).Return(nil)
	userRepo.On("DeleteSessionsForUser", mock.Anything, 1).Return(nil)

	err := service.ChangePassword(context.Background(), 1, "newpassword", "")

	assert.NoError(t, err)
	userRepo.AssertExpectations(t)
}

func TestUserService_ChangePassword_EmptyPassword(t *testing.T) {
	service, _ := setupUserService()

	err := service.ChangePassword(context.Background(), 1, "", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "password cannot be empty")
}

func TestUserService_ChangePassword_WithKeepToken(t *testing.T) {
	defer setupTestConfig()()

	service, userRepo := setupUserService()

	userRepo.On("UpdatePassword", mock.Anything, 1, mock.Anything, false).Return(nil)
	userRepo.On("DeleteSessionsForUserExcept", mock.Anything, 1, "keep-this-token").Return(nil)

	err := service.ChangePassword(context.Background(), 1, "newpassword", "keep-this-token")

	assert.NoError(t, err)
	userRepo.AssertExpectations(t)
	userRepo.AssertNotCalled(t, "DeleteSessionsForUser")
}

func TestUserService_ChangePassword_UpdateError(t *testing.T) {
	defer setupTestConfig()()

	service, userRepo := setupUserService()

	userRepo.On("UpdatePassword", mock.Anything, 1, mock.Anything, false).Return(errors.New("database error"))

	err := service.ChangePassword(context.Background(), 1, "newpassword", "")

	assert.Error(t, err)
}

// ============================================================================
// DeleteUser tests
// ============================================================================

func TestUserService_DeleteUser_Success(t *testing.T) {
	service, userRepo := setupUserService()

	userRepo.On("GetUserById", mock.Anything, 1).Return(&gen.User{Id: 1, Username: "testuser"}, nil)
	userRepo.On("DeleteUserById", mock.Anything, 1).Return(nil)

	err := service.DeleteUser(context.Background(), 1)

	assert.NoError(t, err)
	userRepo.AssertExpectations(t)
}

func TestUserService_DeleteUser_Error(t *testing.T) {
	service, userRepo := setupUserService()

	userRepo.On("GetUserById", mock.Anything, 1).Return(&gen.User{Id: 1, Username: "testuser"}, nil)
	userRepo.On("DeleteUserById", mock.Anything, 1).Return(errors.New("database error"))

	err := service.DeleteUser(context.Background(), 1)

	assert.Error(t, err)
}

// ============================================================================
// SetMustChangeFlag tests
// ============================================================================

func TestUserService_SetMustChangeFlag_True(t *testing.T) {
	service, userRepo := setupUserService()

	userRepo.On("SetMustChangeFlag", mock.Anything, 1, true).Return(nil)

	err := service.SetMustChangeFlag(context.Background(), 1, true)

	assert.NoError(t, err)
	userRepo.AssertExpectations(t)
}

func TestUserService_SetMustChangeFlag_False(t *testing.T) {
	service, userRepo := setupUserService()

	userRepo.On("SetMustChangeFlag", mock.Anything, 1, false).Return(nil)

	err := service.SetMustChangeFlag(context.Background(), 1, false)

	assert.NoError(t, err)
	userRepo.AssertExpectations(t)
}

func TestUserService_SetMustChangeFlag_Error(t *testing.T) {
	service, userRepo := setupUserService()

	userRepo.On("SetMustChangeFlag", mock.Anything, 1, true).Return(errors.New("database error"))

	err := service.SetMustChangeFlag(context.Background(), 1, true)

	assert.Error(t, err)
}

// ============================================================================
// SetUserRoles tests
// ============================================================================

func TestUserService_SetUserRoles_Success(t *testing.T) {
	service, userRepo := setupUserService()

	roles := []string{"admin", "user"}
	userRepo.On("SetRolesForUser", mock.Anything, 1, roles).Return(nil)
	userRepo.On("GetUserById", mock.Anything, 1).Return(&gen.User{Id: 1, Username: "testuser"}, nil)

	err := service.SetUserRoles(context.Background(), 1, roles)

	assert.NoError(t, err)
	userRepo.AssertExpectations(t)
}

func TestUserService_SetUserRoles_EmptyRoles(t *testing.T) {
	service, userRepo := setupUserService()

	userRepo.On("SetRolesForUser", mock.Anything, 1, []string{}).Return(nil)
	userRepo.On("GetUserById", mock.Anything, 1).Return(&gen.User{Id: 1, Username: "testuser"}, nil)

	err := service.SetUserRoles(context.Background(), 1, []string{})

	assert.NoError(t, err)
}

func TestUserService_SetUserRoles_Error(t *testing.T) {
	service, userRepo := setupUserService()

	userRepo.On("SetRolesForUser", mock.Anything, 1, []string{"admin"}).Return(errors.New("database error"))

	err := service.SetUserRoles(context.Background(), 1, []string{"admin"})

	assert.Error(t, err)
}

// ============================================================================
// Edge cases
// ============================================================================

func TestUserService_CreateUser_NilConfig(t *testing.T) {
	// Test with nil AppConfig (should use default bcrypt cost)
	origConfig := appProps.AppConfig()
	appProps.SetAppConfig(nil)
	defer func() { appProps.SetAppConfig(origConfig) }()

	service, userRepo := setupUserService()

	user := gen.User{Username: "newuser"}

	userRepo.On("CreateUser", mock.Anything, mock.Anything, mock.Anything).Return(1, nil)

	id, err := service.CreateUser(context.Background(), user, "password123")

	assert.NoError(t, err)
	assert.Equal(t, 1, id)
}
