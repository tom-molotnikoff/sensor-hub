package service

import (
	"context"
	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const (
	minAdminPasswordLength = 8
	minAdminBcryptCost     = 10
)

func CreateFirstAdmin(ctx context.Context, users database.UserRepository, username, email, password string, mustChangePassword bool) error {
	if len(password) < minAdminPasswordLength {
		return fmt.Errorf("password must be at least %d characters", minAdminPasswordLength)
	}
	cost := minAdminBcryptCost
	if cfg := appProps.AppConfig(); cfg != nil && cfg.AuthBcryptCost > cost {
		cost = cfg.AuthBcryptCost
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return err
	}
	user := gen.User{Username: username, Email: email, MustChangePassword: mustChangePassword}
	_, err = users.CreateFirstAdmin(ctx, user, string(hash))
	return err
}
