package middleware

import (
	"context"
	db "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"time"

	"github.com/stretchr/testify/mock"
)

type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) Login(ctx context.Context, username, password, ip, userAgent string) (string, string, bool, error) {
	args := m.Called(ctx, username, password, ip, userAgent)
	return args.String(0), args.String(1), args.Bool(2), args.Error(3)
}

func (m *MockAuthService) ValidateSession(ctx context.Context, rawToken string) (*gen.User, error) {
	args := m.Called(ctx, rawToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.User), args.Error(1)
}

func (m *MockAuthService) Logout(ctx context.Context, rawToken string) error {
	args := m.Called(ctx, rawToken)
	return args.Error(0)
}

func (m *MockAuthService) ChangePassword(ctx context.Context, userId int, newPassword string) error {
	args := m.Called(ctx, userId, newPassword)
	return args.Error(0)
}

func (m *MockAuthService) ListSessionsForUser(ctx context.Context, userId int) ([]db.SessionInfo, error) {
	args := m.Called(ctx, userId)
	return args.Get(0).([]db.SessionInfo), args.Error(1)
}

func (m *MockAuthService) RevokeSessionById(ctx context.Context, sessionId int64) error {
	args := m.Called(ctx, sessionId)
	return args.Error(0)
}

func (m *MockAuthService) RevokeSessionByIdWithActor(ctx context.Context, sessionId int64, revokedByUserId *int, reason *string) error {
	args := m.Called(ctx, sessionId, revokedByUserId, reason)
	return args.Error(0)
}

func (m *MockAuthService) GetCSRFForToken(ctx context.Context, rawToken string) (string, error) {
	args := m.Called(ctx, rawToken)
	return args.String(0), args.Error(1)
}

func (m *MockAuthService) GetSessionIdForToken(ctx context.Context, rawToken string) (int64, error) {
	args := m.Called(ctx, rawToken)
	return args.Get(0).(int64), args.Error(1)
}

// MockApiKeyService mocks key validation; the key-management methods are not
// reached by the middleware.
type MockApiKeyService struct {
	mock.Mock
}

func (m *MockApiKeyService) ValidateApiKey(ctx context.Context, rawKey string) (*gen.User, error) {
	args := m.Called(ctx, rawKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.User), args.Error(1)
}

func (m *MockApiKeyService) CreateApiKey(context.Context, string, int, *time.Time) (string, error) {
	panic("not used by the middleware")
}

func (m *MockApiKeyService) ListApiKeysForUser(context.Context, int) ([]db.ApiKey, error) {
	panic("not used by the middleware")
}

func (m *MockApiKeyService) UpdateApiKeyExpiry(context.Context, int, *gen.User, *time.Time) error {
	panic("not used by the middleware")
}

func (m *MockApiKeyService) RevokeApiKey(context.Context, int, *gen.User) error {
	panic("not used by the middleware")
}

func (m *MockApiKeyService) DeleteApiKey(context.Context, int, *gen.User) error {
	panic("not used by the middleware")
}
