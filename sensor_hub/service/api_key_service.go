package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"fmt"
	"log/slog"
	"time"
)

type ApiKeyServiceInterface interface {
	CreateApiKey(ctx context.Context, name string, userId int, expiresAt *time.Time) (fullKey string, err error)
	ListApiKeysForUser(ctx context.Context, userId int) ([]database.ApiKey, error)
	// UpdateApiKeyExpiry, RevokeApiKey and DeleteApiKey act on the caller's
	// own key, or on any key when the caller holds manage_users. They return
	// database.ErrApiKeyNotFound when the key does not exist or belongs to
	// another user the caller may not manage.
	UpdateApiKeyExpiry(ctx context.Context, keyId int, caller *gen.User, expiresAt *time.Time) error
	RevokeApiKey(ctx context.Context, keyId int, caller *gen.User) error
	DeleteApiKey(ctx context.Context, keyId int, caller *gen.User) error
	ValidateApiKey(ctx context.Context, rawKey string) (*gen.User, error)
}

type ApiKeyService struct {
	apiKeyRepo database.ApiKeyRepository
	userRepo   database.UserRepository
	roleRepo   database.RoleRepository
	logger     *slog.Logger
}

func NewApiKeyService(a database.ApiKeyRepository, u database.UserRepository, r database.RoleRepository, logger *slog.Logger) *ApiKeyService {
	return &ApiKeyService{apiKeyRepo: a, userRepo: u, roleRepo: r, logger: logger.With("component", "api_key_service")}
}

const apiKeyPrefix = "shk_"

func (s *ApiKeyService) CreateApiKey(ctx context.Context, name string, userId int, expiresAt *time.Time) (string, error) {
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return "", fmt.Errorf("failed to generate random key: %w", err)
	}

	fullKey := apiKeyPrefix + hex.EncodeToString(rawBytes)
	keyPrefix := fullKey[:12]
	keyHash := hashKey(fullKey)

	_, err := s.apiKeyRepo.CreateApiKey(ctx, name, keyPrefix, keyHash, userId, expiresAt)
	if err != nil {
		return "", fmt.Errorf("failed to store api key: %w", err)
	}

	return fullKey, nil
}

func (s *ApiKeyService) ListApiKeysForUser(ctx context.Context, userId int) ([]database.ApiKey, error) {
	return s.apiKeyRepo.ListApiKeysForUser(ctx, userId)
}

func (s *ApiKeyService) UpdateApiKeyExpiry(ctx context.Context, keyId int, caller *gen.User, expiresAt *time.Time) error {
	return s.apiKeyRepo.UpdateApiKeyExpiry(ctx, keyId, apiKeyOwnerScope(caller), expiresAt)
}

func (s *ApiKeyService) RevokeApiKey(ctx context.Context, keyId int, caller *gen.User) error {
	return s.apiKeyRepo.RevokeApiKey(ctx, keyId, apiKeyOwnerScope(caller))
}

func (s *ApiKeyService) DeleteApiKey(ctx context.Context, keyId int, caller *gen.User) error {
	return s.apiKeyRepo.DeleteApiKey(ctx, keyId, apiKeyOwnerScope(caller))
}

// apiKeyOwnerScope is the owner a key operation is restricted to: none for a
// caller who manages users, otherwise the caller.
func apiKeyOwnerScope(caller *gen.User) *int {
	if hasPermission(caller.Permissions, "manage_users") {
		return nil
	}
	id := caller.Id
	return &id
}

func (s *ApiKeyService) ValidateApiKey(ctx context.Context, rawKey string) (*gen.User, error) {
	keyHash := hashKey(rawKey)

	apiKey, err := s.apiKeyRepo.GetApiKeyByHash(ctx, keyHash)
	if err != nil {
		return nil, fmt.Errorf("failed to look up api key: %w", err)
	}
	if apiKey == nil {
		return nil, nil
	}

	user, err := s.userRepo.GetUserById(ctx, apiKey.UserId)
	if err != nil {
		return nil, fmt.Errorf("failed to look up user for api key: %w", err)
	}
	if user == nil || user.Disabled {
		return nil, nil
	}

	perms, err := s.roleRepo.GetPermissionsForUser(ctx, user.Id)
	if err == nil {
		user.Permissions = perms
	}

	// Fire-and-forget last_used update
	go func() {
		if err := s.apiKeyRepo.UpdateLastUsed(context.Background(), apiKey.Id); err != nil {
			s.logger.Error("failed to update api key last_used_at", "error", err)
		}
	}()

	return user, nil
}

func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}
