package service

import (
	appProps "example/sensorHub/application_properties"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const (
	minAdminPasswordLength = 8
	minAdminBcryptCost     = 10
)

func HashFirstAdminPassword(password string) (string, error) {
	if len(password) < minAdminPasswordLength {
		return "", fmt.Errorf("password must be at least %d characters", minAdminPasswordLength)
	}
	cost := minAdminBcryptCost
	if cfg := appProps.AppConfig(); cfg != nil && cfg.AuthBcryptCost > cost {
		cost = cfg.AuthBcryptCost
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}
