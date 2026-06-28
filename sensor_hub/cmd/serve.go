package cmd

import (
	"context"
	"database/sql"
	"example/sensorHub/actuation"
	"example/sensorHub/alerting"
	"example/sensorHub/api"
	"example/sensorHub/api/middleware"
	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	_ "example/sensorHub/drivers" // register sensor drivers
	mqttBrokerPkg "example/sensorHub/mqtt"
	"example/sensorHub/oauth"
	"example/sensorHub/service"
	"example/sensorHub/smtp"
	"example/sensorHub/telemetry"
	"example/sensorHub/ws"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var configDir string
var logFile string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Sensor Hub server",
	Long:  "Starts the HTTP API server, sensor discovery, periodic collection, and serves the embedded UI.",
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().StringVar(&configDir, "config-dir", "configuration", "Path to configuration directory")
	serveCmd.Flags().StringVar(&logFile, "log-file", "", "Path to log file (default: stdout)")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	err := appProps.InitialiseConfig(configDir)
	if err != nil {
		return fmt.Errorf("failed to initialise application configuration: %w", err)
	}

	appProps.WatchConfigFiles(ctx)

	logLevel := telemetry.ParseLogLevel(appProps.AppConfig.LogLevel)

	tel, err := telemetry.Init(context.Background(), telemetry.Config{
		ServiceName: "sensor-hub",
		Version:     Version,
		LogLevel:    logLevel,
		LogFilePath: logFile,
	})
	if err != nil {
		return fmt.Errorf("failed to initialise telemetry: %w", err)
	}
	defer tel.Shutdown()

	logger := tel.Logger

	// Start embedded MQTT broker if enabled
	embeddedBroker := mqttBrokerPkg.NewEmbeddedBroker(mqttBrokerPkg.BrokerConfig{
		TCPAddress: fmt.Sprintf(":%d", appProps.AppConfig.MQTTBrokerPort),
	}, logger)

	if appProps.AppConfig.MQTTBrokerEnabled {
		if err := embeddedBroker.Start(); err != nil {
			return fmt.Errorf("failed to start embedded MQTT broker: %w", err)
		}
		defer func() {
			if err := embeddedBroker.Stop(); err != nil {
				logger.Error("error stopping embedded MQTT broker", "error", err)
			}
		}()
	}

	db, err := database.InitialiseDatabase(logger)
	if err != nil {
		return fmt.Errorf("failed to initialise database: %w", err)
	}

	defer func(db *sql.DB) {
		if err := db.Close(); err != nil {
			logger.Error("error closing database", "error", err)
		}
	}(db)

	sensorRepo := database.NewSensorRepository(db, logger)
	readingsRepo := database.NewReadingsRepository(db, logger)
	mtRepo := database.NewMeasurementTypeRepository(db, logger)
	alertRepo := database.NewAlertRepository(db, logger)
	notificationRepo := database.NewNotificationRepository(db, logger)

	userRepo := database.NewUserRepository(db, logger)
	sessionRepo := database.NewSessionRepository(db, logger)
	failedRepo := database.NewFailedLoginRepository(db, logger)
	roleRepo := database.NewRoleRepository(db, logger)

	smtpNotifier := smtp.NewSMTPNotifier(logger)
	wsBroadcaster := ws.NewNotificationBroadcaster(logger)
	notificationService := service.NewNotificationService(notificationRepo, wsBroadcaster, logger)
	notificationService.SetEmailNotifier(smtpNotifier)
	thresholdProcessor := alerting.NewThresholdAlertProcessor(alertRepo, &notifRepoAdapter{notificationRepo}, wsBroadcaster, smtpNotifier, logger)
	sensorService := service.NewSensorService(sensorRepo, readingsRepo, mtRepo, thresholdProcessor, notificationService, logger)

	aggregationTiers, err := service.ParseAggregationTiers(appProps.AppConfig.ReadingsAggregationTiers)
	if err != nil {
		return fmt.Errorf("failed to parse aggregation tiers: %w", err)
	}
	if aggregationTiers == nil {
		aggregationTiers = service.DefaultAggregationTiers
	}
	maintenanceRepo := database.NewMaintenanceRepository(db)

	readingsService := service.NewReadingsService(readingsRepo, mtRepo, aggregationTiers, appProps.AppConfig.ReadingsAggregationEnabled, logger)

	// Seed the in-memory current-readings store so a freshly started server serves
	// correct toggle/sensor state on connect without a database query.
	if latest, err := readingsService.ServiceGetLatest(ctx); err != nil {
		logger.Warn("failed to seed current-readings store", "error", err)
	} else {
		ws.SeedReadings(latest)
	}
	propertiesService := service.NewPropertiesService(logger)
	cleanupService := service.NewCleanupService(sensorRepo, readingsRepo, failedRepo, notificationRepo, alertRepo, maintenanceRepo, logger)

	userService := service.NewUserService(userRepo, notificationService, logger)
	authService := service.NewAuthService(userRepo, sessionRepo, failedRepo, roleRepo, logger)
	roleService := service.NewRoleService(roleRepo, logger)
	alertManagementService := service.NewAlertManagementService(alertRepo, logger)

	apiKeyRepo := database.NewApiKeyRepository(db, logger)
	apiKeyService := service.NewApiKeyService(apiKeyRepo, userRepo, roleRepo, logger)

	dashboardRepo := database.NewDashboardRepository(db, logger)
	dashboardService := service.NewDashboardService(dashboardRepo, logger)

	mqttBrokerRepo := database.NewMQTTBrokerRepository(db, logger)
	mqttSubRepo := database.NewMQTTSubscriptionRepository(db, logger)
	commandHistoryRepo := database.NewSensorCommandHistoryRepository(db, logger)
	mqttService := service.NewMQTTService(mqttBrokerRepo, mqttSubRepo, logger)

	connManager := mqttBrokerPkg.NewConnectionManager(sensorService, mqttSubRepo, mqttBrokerRepo, logger)
	mqttService.SetSubscriptionNotifier(connManager)
	commandTracker := actuation.NewCommandTracker(commandHistoryRepo, ws.NewCommandStatusBroadcaster(logger), logger)
	commandService := service.NewCommandService(sensorRepo, mqttSubRepo, commandHistoryRepo, connManager, commandTracker, logger)
	sensorService.SetReadingsObserver(commandTracker)
	if err := commandTracker.RecoverPending(ctx); err != nil {
		return fmt.Errorf("failed to recover pending commands: %w", err)
	}

	middleware.InitAuthMiddleware(authService)
	middleware.InitPermissionMiddleware(roleRepo)
	middleware.InitApiKeyMiddleware(apiKeyService)

	initialAdmin := os.Getenv("SENSOR_HUB_INITIAL_ADMIN")
	if initialAdmin != "" {
		var username, password string
		for i, c := range initialAdmin {
			if c == ':' {
				username = initialAdmin[:i]
				password = initialAdmin[i+1:]
				break
			}
		}
		if username != "" && password != "" {
			err = authService.CreateInitialAdminIfNone(context.Background(), username, password)
			if err != nil {
				return fmt.Errorf("failed to create initial admin user: %w", err)
			}
			logger.Info("initial admin user ready", "username", username)
		}
	}

	err = sensorService.ServiceDiscoverSensors(context.Background())
	if err != nil {
		return fmt.Errorf("failed to discover sensors: %w", err)
	}

	// Start MQTT connection manager (connects to all enabled brokers)
	if err := connManager.Start(ctx); err != nil {
		logger.Error("failed to start MQTT connection manager", "error", err)
	}
	defer connManager.Stop()

	err = oauth.InitialiseOauth()
	if err != nil {
		logger.Warn("failed to initialise OAuth", "error", err)
	}

	oauthAdapter := service.NewOAuthServiceAdapter(oauth.GetService())

	server := api.NewServer(
		sensorService,
		commandService,
		readingsService,
		authService,
		userService,
		roleService,
		alertManagementService,
		notificationService,
		apiKeyService,
		dashboardService,
		propertiesService,
		mqttService,
		oauthAdapter,
		connManager,
	)

	sensorService.ServiceStartPeriodicSensorCollection(ctx)

	cleanupService.StartPeriodicCleanup(ctx)

	return api.InitialiseAndListen(ctx, logger, tel.PrometheusHandler, server)
}
