package database

import (
	"context"

	gen "example/sensorHub/gen"
)

type UserRepository interface {
	CreateUser(ctx context.Context, user gen.User, passwordHash string) (int, error)
	CreateFirstAdmin(ctx context.Context, user gen.User, passwordHash string) (int, error)
	GetUserByUsername(ctx context.Context, username string) (*gen.User, string, error) // returns user and passwordHash
	GetUserById(ctx context.Context, id int) (*gen.User, error)
	ListUsers(ctx context.Context) ([]gen.User, error)
	UpdatePassword(ctx context.Context, userId int, passwordHash string, mustChange bool) error
	SetDisabled(ctx context.Context, userId int, disabled bool) error
	AssignRoleToUser(ctx context.Context, userId int, roleName string) error
	GetRolesForUser(ctx context.Context, userId int) ([]string, error)
	DeleteSessionsForUser(ctx context.Context, userId int) error
	DeleteSessionsForUserExcept(ctx context.Context, userId int, keepToken string) error
	DeleteUserById(ctx context.Context, userId int) error
	SetMustChangeFlag(ctx context.Context, userId int, mustChange bool) error
	SetRolesForUser(ctx context.Context, userId int, roles []string) error
}
