package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	database "example/sensorHub/db"
	"example/sensorHub/service"
	"example/sensorHub/testharness/fixtures"
)

const (
	adminUsername   = "admin"
	adminAPIKeyName = "devstack seed"
)

var devUsers = []fixtures.User{
	{Username: adminUsername, Password: "adminpassword", Role: service.RoleAdmin},
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
	db        *database.Handles
	users     service.UserServiceInterface
	apiKeys   service.ApiKeyServiceInterface
	sensors   service.SensorServiceInterface
	mqtt      service.MQTTServiceInterface
	httpMocks []httpMock
	logger    *slog.Logger
}

func seed(ctx context.Context, db *database.Handles, logger *slog.Logger, httpMocks []httpMock) (string, error) {
	userRepo := database.NewUserRepository(db, logger)
	sensorRepo := database.NewSensorRepository(db, logger)
	measurementTypes := database.NewMeasurementTypeRepository(db, logger)
	readingsRepo := database.NewReadingsRepository(db, sensorRepo, measurementTypes, logger)
	s := &seeder{
		db:        db,
		users:     service.NewUserService(userRepo, nil, logger),
		apiKeys:   service.NewApiKeyService(database.NewApiKeyRepository(db, logger), userRepo, database.NewRoleRepository(db, logger), logger),
		sensors:   service.NewSensorService(sensorRepo, readingsRepo, measurementTypes, nil, nil, nil, logger),
		mqtt:      service.NewMQTTService(database.NewMQTTBrokerRepository(db, logger), database.NewMQTTSubscriptionRepository(db, logger), logger),
		httpMocks: httpMocks,
		logger:    logger,
	}

	marker, err := loadMarker(ctx, db.Writer)
	if err != nil {
		return "", &stepError{step: "read the marker", err: err}
	}
	if marker.seeded {
		logger.Info("entities were seeded before, leaving them as they are")
		active, err := s.adminKeyIsActive(ctx, marker.adminAPIKey)
		if err != nil {
			return "", &stepError{step: "check the admin API key", err: err}
		}
		if !active {
			logger.Warn("the seeded admin API key was revoked, expired or deleted, run down -v for a fresh one")
			return "", nil
		}
		return marker.adminAPIKey, nil
	}
	return s.createEntities(ctx)
}

func (s *seeder) adminKeyIsActive(ctx context.Context, apiKey string) (bool, error) {
	users, err := s.users.ListUsers(ctx)
	if err != nil {
		return false, err
	}
	for _, user := range users {
		if user.Username != adminUsername {
			continue
		}
		keys, err := s.apiKeys.ListApiKeysForUser(ctx, user.Id)
		if err != nil {
			return false, err
		}
		for _, key := range keys {
			expired := key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now())
			if strings.HasPrefix(apiKey, key.KeyPrefix) && !key.Revoked && !expired {
				return true, nil
			}
		}
	}
	return false, nil
}

func (s *seeder) createEntities(ctx context.Context) (string, error) {
	userIDs, err := s.createUsers(ctx)
	if err != nil {
		return "", &stepError{step: "create the users", err: err}
	}
	apiKey, err := s.createAdminAPIKey(ctx, userIDs[adminUsername])
	if err != nil {
		return "", &stepError{step: "create the admin API key", err: err}
	}
	if err := s.createMQTTSubscription(ctx); err != nil {
		return "", &stepError{step: "create the MQTT subscription", err: err}
	}
	if err := s.createSensors(ctx); err != nil {
		return "", &stepError{step: "create the sensors", err: err}
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
		if id, ok := ids[user.Username]; ok {
			if err := s.finishUser(ctx, id, user); err != nil {
				return nil, err
			}
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

func (s *seeder) finishUser(ctx context.Context, id int, user fixtures.User) error {
	if err := s.users.SetUserRoles(ctx, id, []string{user.Role}); err != nil {
		return fmt.Errorf("failed to set the role of %s: %w", user.Username, err)
	}
	if err := s.users.SetMustChangeFlag(ctx, id, false); err != nil {
		return fmt.Errorf("failed to clear the password change for %s: %w", user.Username, err)
	}
	s.logger.Info("user exists, made sure of its role and password state", "username", user.Username, "role", user.Role)
	return nil
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
