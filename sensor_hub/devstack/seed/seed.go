package main

import (
	"context"
	"log/slog"

	database "example/sensorHub/db"
	"example/sensorHub/service"
	"example/sensorHub/testharness/fixtures"
)

const adminAPIKeyName = "devstack seed"

var devUsers = []fixtures.User{
	{Username: "admin", Password: "adminpassword", Role: service.RoleAdmin},
	{Username: "user", Password: "userpassword", Role: service.RoleUser},
	{Username: "viewer", Password: "viewerpassword", Role: service.RoleViewer},
}

type stepError struct {
	step string
	err  error
}

func (e *stepError) Error() string {
	return e.step + ": " + e.err.Error()
}

func (e *stepError) Unwrap() error {
	return e.err
}

type seeder struct {
	db      *database.Handles
	users   service.UserServiceInterface
	apiKeys service.ApiKeyServiceInterface
	logger  *slog.Logger
}

func seed(ctx context.Context, db *database.Handles, logger *slog.Logger) (string, error) {
	userRepo := database.NewUserRepository(db, logger)
	s := &seeder{
		db:      db,
		users:   service.NewUserService(userRepo, nil, logger),
		apiKeys: service.NewApiKeyService(database.NewApiKeyRepository(db, logger), userRepo, database.NewRoleRepository(db, logger), logger),
		logger:  logger,
	}

	marker, err := loadMarker(ctx, db.Writer)
	if err != nil {
		return "", &stepError{step: "read the marker", err: err}
	}
	if marker.seeded {
		logger.Info("entities were seeded before, leaving them as they are")
		return marker.adminAPIKey, nil
	}
	return s.createEntities(ctx)
}

func (s *seeder) createEntities(ctx context.Context) (string, error) {
	userIDs, err := s.createUsers(ctx)
	if err != nil {
		return "", &stepError{step: "create the users", err: err}
	}
	apiKey, err := s.createAdminAPIKey(ctx, userIDs["admin"])
	if err != nil {
		return "", &stepError{step: "create the admin API key", err: err}
	}
	if err := writeMarker(ctx, s.db.Writer, apiKey); err != nil {
		return "", &stepError{step: "write the marker", err: err}
	}
	return apiKey, nil
}

func (s *seeder) createUsers(ctx context.Context) (map[string]int, error) {
	existing, err := s.users.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]int, len(existing)+len(devUsers))
	for _, user := range existing {
		ids[user.Username] = user.Id
	}
	for _, user := range devUsers {
		if _, ok := ids[user.Username]; ok {
			s.logger.Info("user exists, skipping", "username", user.Username)
			continue
		}
		id, err := fixtures.CreateUser(ctx, s.users, user)
		if err != nil {
			return nil, err
		}
		ids[user.Username] = id
		s.logger.Info("created user", "username", user.Username, "role", user.Role)
	}
	return ids, nil
}

func (s *seeder) createAdminAPIKey(ctx context.Context, adminID int) (string, error) {
	keys, err := s.apiKeys.ListApiKeysForUser(ctx, adminID)
	if err != nil {
		return "", err
	}
	for _, key := range keys {
		if key.Name != adminAPIKeyName {
			continue
		}
		if err := s.apiKeys.DeleteApiKey(ctx, key.Id, adminID); err != nil {
			return "", err
		}
		s.logger.Info("deleted an API key left by an unfinished run", "key_prefix", key.KeyPrefix)
	}
	apiKey, err := s.apiKeys.CreateApiKey(ctx, adminAPIKeyName, adminID, nil)
	if err != nil {
		return "", err
	}
	s.logger.Info("created the admin API key", "name", adminAPIKeyName)
	return apiKey, nil
}
