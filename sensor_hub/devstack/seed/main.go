package main

import (
	"context"
	"errors"
	"log/slog"
	"os"

	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
)

const configDir = "configuration"

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	apiKey, err := run(context.Background(), logger)
	if err != nil {
		failed := &stepError{step: "unknown", err: err}
		errors.As(err, &failed)
		logger.Error("seed failed", "step", failed.step, "cause", failed.err)
		os.Exit(1)
	}
	if apiKey == "" {
		logger.Info("seed complete")
		return
	}
	logger.Info("seed complete", "admin_api_key", apiKey)
}

func run(ctx context.Context, logger *slog.Logger) (string, error) {
	if err := appProps.InitialiseConfig(configDir); err != nil {
		return "", &stepError{step: "load the configuration", err: err}
	}
	db, err := database.Open(appProps.AppConfig(), logger)
	if err != nil {
		return "", &stepError{step: "open and migrate the database", err: err}
	}
	defer db.Close()
	return seed(ctx, db, logger, composeHTTPMocks, historyWindow)
}
