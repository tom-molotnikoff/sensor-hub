package fixtures

import (
	"context"
	"fmt"

	gen "example/sensorHub/gen"
	"example/sensorHub/service"
)

type User struct {
	Username string
	Password string
	Role     string
}

func CreateUser(ctx context.Context, users service.UserServiceInterface, user User) (int, error) {
	id, err := users.CreateUser(ctx, gen.User{Username: user.Username, Roles: []string{user.Role}}, user.Password)
	if err != nil {
		return 0, fmt.Errorf("failed to create user %s: %w", user.Username, err)
	}
	if err := users.SetMustChangeFlag(ctx, id, false); err != nil {
		return 0, fmt.Errorf("failed to clear the password change for %s: %w", user.Username, err)
	}
	return id, nil
}
