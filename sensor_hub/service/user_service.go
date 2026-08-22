package service

import (
	"context"
	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/notifications"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type UserServiceInterface interface {
	CreateUser(ctx context.Context, user gen.User, plainPassword string) (int, error)
	ListUsers(ctx context.Context) ([]gen.User, error)
	GetUserById(ctx context.Context, id int) (*gen.User, error)
	ChangePassword(ctx context.Context, userId int, newPassword string, keepToken string) error
	DeleteUser(ctx context.Context, userId int) error
	SetMustChangeFlag(ctx context.Context, userId int, mustChange bool) error
	SetUserRoles(ctx context.Context, userId int, roles []string) error
}

type UserService struct {
	userRepo database.UserRepository
	notifSvc NotificationServiceInterface
	logger   *slog.Logger
}

func NewUserService(u database.UserRepository, n NotificationServiceInterface, logger *slog.Logger) *UserService {
	return &UserService{userRepo: u, notifSvc: n, logger: logger.With("component", "user_service")}
}

func (s *UserService) notifyUserEvent(action, username string, metadata map[string]interface{}) {
	if s.notifSvc == nil {
		return
	}
	notif := notifications.Notification{
		Category: notifications.CategoryUserManagement,
		Severity: notifications.SeverityInfo,
		Title:    fmt.Sprintf("User %s", action),
		Message:  fmt.Sprintf("User '%s' was %s", username, action),
		Metadata: metadata,
	}
	go s.notifSvc.CreateNotification(context.Background(), notif, "view_notifications_user_mgmt")
}

func (s *UserService) CreateUser(ctx context.Context, user gen.User, plainPassword string) (int, error) {
	if plainPassword == "" {
		return 0, fmt.Errorf("password cannot be empty")
	}
	cost := 12
	if cfg := appProps.AppConfig(); cfg != nil && cfg.AuthBcryptCost > 0 {
		cost = cfg.AuthBcryptCost
	}
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(plainPassword), cost)
	if err != nil {
		return 0, err
	}
	user.MustChangePassword = true
	user.CreatedAt = time.Now()
	id, err := s.userRepo.CreateUser(ctx, user, string(hashBytes))
	if err != nil {
		return 0, err
	}
	for _, r := range user.Roles {
		err = s.userRepo.AssignRoleToUser(ctx, id, r)
		if err != nil {
			return 0, fmt.Errorf("failed to assign role %s to user: %w", r, err)
		}
	}
	s.notifyUserEvent("added", user.Username, map[string]interface{}{"user_id": id})
	return id, nil
}

func (s *UserService) ListUsers(ctx context.Context) ([]gen.User, error) {
	return s.userRepo.ListUsers(ctx)
}

func (s *UserService) GetUserById(ctx context.Context, id int) (*gen.User, error) {
	return s.userRepo.GetUserById(ctx, id)
}

func (s *UserService) ChangePassword(ctx context.Context, userId int, newPassword string, keepToken string) error {
	if newPassword == "" {
		return fmt.Errorf("password cannot be empty")
	}
	cost := 12
	if cfg := appProps.AppConfig(); cfg != nil && cfg.AuthBcryptCost > 0 {
		cost = cfg.AuthBcryptCost
	}
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(newPassword), cost)
	if err != nil {
		return err
	}

	if err := s.userRepo.UpdatePassword(ctx, userId, string(hashBytes), false); err != nil {
		return err
	}
	if keepToken != "" {
		return s.userRepo.DeleteSessionsForUserExcept(ctx, userId, keepToken)
	}
	return s.userRepo.DeleteSessionsForUser(ctx, userId)
}

func (s *UserService) DeleteUser(ctx context.Context, userId int) error {
	user, _ := s.userRepo.GetUserById(ctx, userId)
	err := s.userRepo.DeleteUserById(ctx, userId)
	if err != nil {
		return err
	}
	if user != nil {
		s.notifyUserEvent("removed", user.Username, map[string]interface{}{"user_id": userId})
	}
	return nil
}

func (s *UserService) SetMustChangeFlag(ctx context.Context, userId int, mustChange bool) error {
	return s.userRepo.SetMustChangeFlag(ctx, userId, mustChange)
}

func (s *UserService) SetUserRoles(ctx context.Context, userId int, roles []string) error {
	err := s.userRepo.SetRolesForUser(ctx, userId, roles)
	if err != nil {
		return err
	}
	user, _ := s.userRepo.GetUserById(ctx, userId)
	if user != nil {
		s.notifyUserEvent("role changed", user.Username, map[string]interface{}{
			"user_id": userId,
			"roles":   roles,
		})
	}
	return nil
}
