package cmd

import (
	"context"
	"example/sensorHub/actuation"
	"example/sensorHub/alerting"
	"example/sensorHub/api"
	"example/sensorHub/api/middleware"
	appProps "example/sensorHub/application_properties"
	"example/sensorHub/automation"
	database "example/sensorHub/db"
	_ "example/sensorHub/drivers" // register sensor drivers
	mqttBrokerPkg "example/sensorHub/mqtt"
	"example/sensorHub/oauth"
	"example/sensorHub/readings"
	"example/sensorHub/secrets"
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

var logFile string
var secretsKeyFile string

var localServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Sensor Hub server",
	Long:  "Starts the HTTP API server, periodic collection, and serves the embedded UI.",
	RunE:  runServe,
}

func init() {
	localServeCmd.Flags().StringVar(&logFile, "log-file", "", "Path to log file (default: stdout)")
	localServeCmd.Flags().StringVar(&secretsKeyFile, "secrets-key-file", "", "Path to the secret-store key, read when there is no systemd credential or Compose secret (default: <config-dir>/secrets.key)")
	localCmd.AddCommand(localServeCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	err := appProps.InitialiseConfig(localConfigDir)
	if err != nil {
		return fmt.Errorf("failed to initialise application configuration: %w", err)
	}

	tel, err := telemetry.Init(context.Background(), telemetry.Config{
		ServiceName: "sensor-hub",
		Version:     Version,
		LogFilePath: logFile,
	})
	if err != nil {
		return fmt.Errorf("failed to initialise telemetry: %w", err)
	}
	defer tel.Shutdown()

	logger := tel.Logger

	if os.Getenv("SENSOR_HUB_INITIAL_ADMIN") != "" {
		logger.Warn("SENSOR_HUB_INITIAL_ADMIN is ignored since 2.0; remove it from the environment and create the first admin with 'sensor-hub local admin create'")
	}

	bootCfg := appProps.AppConfig()
	secretsKey, err := secrets.LoadKey(secrets.LocationsFor(localConfigDir, secretsKeyFile), bootCfg.DatabasePath, logger)
	if err != nil {
		logger.Error("cannot use the secret-store key", "error", err)
		return err
	}

	db, err := database.Open(bootCfg, logger)
	if err != nil {
		return fmt.Errorf("failed to initialise database: %w", err)
	}

	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("error closing database", "error", err)
		}
	}()

	secretStore, err := secrets.Open(ctx, db, secretsKey, logger)
	if err != nil {
		return fmt.Errorf("failed to open the secret store: %w", err)
	}

	// Start the embedded MQTT broker if enabled. Devices authenticate against
	// the MQTT clients in the database, so it starts once the database is open.
	mqttClientService := service.NewMQTTClientService(database.NewMQTTClientRepository(db, logger), logger)
	embeddedBroker := mqttBrokerPkg.NewEmbeddedBroker(mqttBrokerPkg.BrokerConfig{
		TCPAddress:       bootCfg.MQTTBrokerAddress(),
		ConnectRateLimit: bootCfg.MQTTBrokerConnectRateLimit,
	}, mqttClientService, logger)
	mqttClientService.SetSessions(embeddedBroker)

	if bootCfg.MQTTBrokerEnabled {
		if err := embeddedBroker.Start(); err != nil {
			return fmt.Errorf("failed to start embedded MQTT broker: %w", err)
		}
		defer func() {
			if err := embeddedBroker.Stop(); err != nil {
				logger.Error("error stopping embedded MQTT broker", "error", err)
			}
		}()
	}

	sensorRepo := database.NewSensorRepository(db, logger)
	mtRepo := database.NewMeasurementTypeRepository(db, logger)
	readingsRepo := database.NewReadingsRepository(db, sensorRepo, mtRepo, logger)
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
	readingsSampler := service.NewReadingsSampler(readingsRepo, logger)
	commandHistoryRepo := database.NewSensorCommandHistoryRepository(db, logger)
	commandTracker := actuation.NewCommandTracker(commandHistoryRepo, ws.NewCommandStatusBroadcaster(logger), logger)
	liveView := service.NewLiveView(sensorRepo, logger)
	automationReadings := automation.NewReadingConsumer()
	readingPipeline := readings.NewPipeline(readingsRepo, liveView, logger,
		commandTracker,
		thresholdProcessor,
		liveView,
		automationReadings,
	)
	sensorService := service.NewSensorService(sensorRepo, mtRepo, readingPipeline, liveView, notificationService, readingsSampler, logger)

	aggregationTiers, err := service.ParseAggregationTiers(bootCfg.ReadingsAggregationTiers)
	if err != nil {
		return fmt.Errorf("failed to parse aggregation tiers: %w", err)
	}
	if aggregationTiers == nil {
		aggregationTiers = service.DefaultAggregationTiers
	}
	maintenanceRepo := database.NewMaintenanceRepository(db)

	readingsService := service.NewReadingsService(readingsRepo, mtRepo, aggregationTiers, bootCfg.ReadingsAggregationEnabled, logger)

	// Seed the in-memory current-readings store so a freshly started server serves
	// correct toggle/sensor state on connect without a database query.
	if latest, err := readingsService.ServiceGetLatest(ctx); err != nil {
		logger.Warn("failed to seed current-readings store", "error", err)
	} else {
		ws.SeedReadings(latest)
	}

	go func() {
		if err := readingsSampler.Sample(ctx); err != nil {
			logger.Warn("failed to sample readings row counts at startup", "error", err)
		}
	}()
	propertiesService := service.NewPropertiesService(logger)

	// External config file edits must reach open browsers: broadcast after
	// each successful watcher reload, the same call a PATCH makes.
	appProps.WatchConfigFiles(ctx, func() {
		propertiesService.BroadcastProperties(context.Background())
	})
	automationRepo := database.NewAutomationRepository(db, logger)
	cleanupService := service.NewCleanupService(sensorRepo, readingsRepo, failedRepo, notificationRepo, alertRepo, automationRepo, commandHistoryRepo, maintenanceRepo, readingsSampler, logger)

	userService := service.NewUserService(userRepo, notificationService, logger)
	authService := service.NewAuthService(userRepo, sessionRepo, failedRepo, logger)
	roleService := service.NewRoleService(roleRepo, logger)
	alertManagementService := service.NewAlertManagementService(alertRepo, thresholdProcessor, logger)

	apiKeyRepo := database.NewApiKeyRepository(db, logger)
	apiKeyService := service.NewApiKeyService(apiKeyRepo, userRepo, roleRepo, logger)

	dashboardRepo := database.NewDashboardRepository(db, logger)
	dashboardService := service.NewDashboardService(dashboardRepo, logger)

	mqttBrokerRepo := database.NewMQTTBrokerRepository(db, logger)
	mqttSubRepo := database.NewMQTTSubscriptionRepository(db, logger)
	mqttService := service.NewMQTTService(mqttBrokerRepo, mqttSubRepo, secretStore, logger)

	connManager := mqttBrokerPkg.NewConnectionManager(sensorService, mqttSubRepo, mqttBrokerRepo, secretStore, embeddedBroker, logger)
	mqttService.SetSubscriptionNotifier(connManager)
	commandService := service.NewCommandService(sensorRepo, mqttSubRepo, commandHistoryRepo, connManager, commandTracker, logger)
	if err := commandTracker.RecoverPending(ctx); err != nil {
		return fmt.Errorf("failed to recover pending commands: %w", err)
	}
	automationService := automation.NewService(automationRepo, sensorService, commandService, notificationService, automationReadings, readingsRepo, logger)
	sensorService.SetSensorObserver(automationService)

	middleware.InitAuthMiddleware(authService)
	middleware.InitApiKeyMiddleware(apiKeyService)

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
		mqttClientService,
		oauthAdapter,
		connManager,
		automationService,
	)

	sensorService.ServiceStartPeriodicSensorCollection(ctx)

	cleanupService.StartPeriodicCleanup(ctx)

	if err := automationService.Start(ctx); err != nil {
		return fmt.Errorf("failed to start automations: %w", err)
	}

	return api.InitialiseAndListen(ctx, logger, bootCfg, tel.PrometheusHandler, server)
}
