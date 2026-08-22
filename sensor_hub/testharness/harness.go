//go:build integration

package testharness

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"example/sensorHub/actuation"
	"example/sensorHub/alerting"
	"example/sensorHub/api"
	"example/sensorHub/api/middleware"
	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	_ "example/sensorHub/drivers" // register sensor drivers
	gen "example/sensorHub/gen"
	mqttpkg "example/sensorHub/mqtt"
	"example/sensorHub/notifications"
	"example/sensorHub/service"
	"example/sensorHub/smtp"
	"example/sensorHub/ws"

	"github.com/gin-gonic/gin"
)

// Env holds references to the running test server and its components.
type Env struct {
	ServerURL         string
	AdminUser         string
	AdminPass         string
	ConfigDir         string
	DB                *sql.DB
	ConnectionManager *mqttpkg.ConnectionManager
	WSCapture         *RecordingWSNotifier
	EmailCapture      *RecordingEmailNotifier
}

const (
	DefaultAdminUser = "testadmin"
	DefaultAdminPass = "testpassword123"
)

// StartServer creates a temp DB, wires up all services, starts the Gin server
// on a random port, and creates an admin user. Cleanup via t.Cleanup.
func StartServer(t interface {
	Helper()
	Fatalf(string, ...any)
	Cleanup(func())
}, sensorURLs []string) *Env {
	env, cleanup, err := startServer(sensorURLs)
	if err != nil {
		cleanup()
		if th, ok := t.(interface{ Fatalf(string, ...any) }); ok {
			th.Fatalf("failed to start server: %v", err)
		}
		return nil
	}
	t.Cleanup(cleanup)
	return env
}

// StartServerForMain is like StartServer but for use in TestMain where
// *testing.T is not available. Returns a cleanup function.
func StartServerForMain(sensorURLs []string) (*Env, func(), error) {
	return startServer(sensorURLs)
}

func startServer(sensorURLs []string) (*Env, func(), error) {
	tmpDir, err := os.MkdirTemp("", "sensor-hub-integration-*")
	if err != nil {
		return nil, func() {}, fmt.Errorf("failed to create temp dir: %w", err)
	}

	cleanupDir := func() { os.RemoveAll(tmpDir) }

	dbPath := filepath.Join(tmpDir, "test.db")
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		cleanupDir()
		return nil, func() {}, fmt.Errorf("failed to create config dir: %w", err)
	}

	// Write minimal config files
	appPropsContent := fmt.Sprintf(
		"sensor.collection.interval=300\nsensor.discovery.skip=true\ndatabase.path=%s\nlog.level=debug\nauth.bcrypt.cost=4\nmqtt.broker.enabled=false\n", dbPath)
	writeFileOrErr(filepath.Join(configDir, "application.properties"), appPropsContent)
	writeFileOrErr(filepath.Join(configDir, "database.properties"), fmt.Sprintf("database.path=%s\n", dbPath))
	writeFileOrErr(filepath.Join(configDir, "smtp.properties"), "smtp.user=\n")

	if err := appProps.InitialiseConfig(configDir); err != nil {
		cleanupDir()
		return nil, func() {}, fmt.Errorf("failed to initialise config: %w", err)
	}

	logger := slog.Default()

	db, err := database.InitialiseDatabase(logger)
	if err != nil {
		cleanupDir()
		return nil, func() {}, fmt.Errorf("failed to initialise database: %w", err)
	}

	// Build the full service graph, mirroring cmd/serve.go
	sensorRepo := database.NewSensorRepository(db, logger)
	readingsRepo := database.NewReadingsRepository(db, logger)
	mtRepo := database.NewMeasurementTypeRepository(db, logger)
	alertRepo := database.NewAlertRepository(db, logger)
	notificationRepo := database.NewNotificationRepository(db, logger)
	userRepo := database.NewUserRepository(db, logger)
	sessionRepo := database.NewSessionRepository(db, logger)
	failedRepo := database.NewFailedLoginRepository(db, logger)
	roleRepo := database.NewRoleRepository(db, logger)
	apiKeyRepo := database.NewApiKeyRepository(db, logger)

	smtpNotifier := smtp.NewSMTPNotifier(logger)
	wsBroadcaster := ws.NewNotificationBroadcaster(logger)
	notificationService := service.NewNotificationService(notificationRepo, wsBroadcaster, logger)
	notificationService.SetEmailNotifier(smtpNotifier)

	wsCapture := &RecordingWSNotifier{}
	emailCapture := &RecordingEmailNotifier{}
	thresholdProcessor := alerting.NewThresholdAlertProcessor(alertRepo, &harnessNotifRepoAdapter{notificationRepo}, wsCapture, emailCapture, logger)
	sensorService := service.NewSensorService(sensorRepo, readingsRepo, mtRepo, thresholdProcessor, notificationService, logger)

	tiers := service.DefaultAggregationTiers
	readingsService := service.NewReadingsService(readingsRepo, mtRepo, tiers, appProps.AppConfig().ReadingsAggregationEnabled, logger)
	propertiesService := service.NewPropertiesService(logger)
	maintenanceRepo := database.NewMaintenanceRepository(db)
	_ = service.NewCleanupService(sensorRepo, readingsRepo, failedRepo, notificationRepo, alertRepo, maintenanceRepo, logger)

	userService := service.NewUserService(userRepo, notificationService, logger)
	authService := service.NewAuthService(userRepo, sessionRepo, failedRepo, roleRepo, logger)
	roleService := service.NewRoleService(roleRepo, logger)
	alertManagementService := service.NewAlertManagementService(alertRepo, logger)
	apiKeyService := service.NewApiKeyService(apiKeyRepo, userRepo, roleRepo, logger)

	// Init middleware
	middleware.InitAuthMiddleware(authService)
	middleware.InitPermissionMiddleware(roleRepo)
	middleware.InitApiKeyMiddleware(apiKeyService)

	dashboardRepo := database.NewDashboardRepository(db, logger)
	dashboardService := service.NewDashboardService(dashboardRepo, logger)

	mqttBrokerRepo := database.NewMQTTBrokerRepository(db, logger)
	mqttSubRepo := database.NewMQTTSubscriptionRepository(db, logger)
	commandHistoryRepo := database.NewSensorCommandHistoryRepository(db, logger)
	mqttService := service.NewMQTTService(mqttBrokerRepo, mqttSubRepo, logger)
	connManager := mqttpkg.NewConnectionManager(sensorService, mqttSubRepo, mqttBrokerRepo, logger)
	mqttService.SetSubscriptionNotifier(connManager)
	commandTracker := actuation.NewCommandTracker(commandHistoryRepo, ws.NewCommandStatusBroadcaster(logger), logger)
	commandService := service.NewCommandService(sensorRepo, mqttSubRepo, commandHistoryRepo, connManager, commandTracker, logger)
	sensorService.SetReadingsObserver(commandTracker)
	if err := commandTracker.RecoverPending(context.Background()); err != nil {
		db.Close()
		cleanupDir()
		return nil, func() {}, fmt.Errorf("failed to recover pending commands: %w", err)
	}

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
		nil, // no OAuth in tests
		connManager,
	)

	// Build Gin router (mirrors api.go without TLS/OTEL/CORS/SPA)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())

	apiGroup := router.Group("/api")
	apiGroup.Use(middleware.CSRFMiddleware())

	gen.RegisterHandlersWithOptions(apiGroup, server, gen.GinServerOptions{
		Middlewares: []gen.MiddlewareFunc{api.RouteAuthAndPermissionMiddleware()},
	})

	// Start HTTP server on random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		db.Close()
		cleanupDir()
		return nil, func() {}, fmt.Errorf("failed to listen: %w", err)
	}
	serverURL := fmt.Sprintf("http://%s", listener.Addr().String())

	srv := &http.Server{Handler: router}
	go srv.Serve(listener)

	// Mirror cmd/serve.go: watch for external config edits and broadcast
	// reloads to properties websocket subscribers.
	watcherCtx, stopWatcher := context.WithCancel(context.Background())
	appProps.WatchConfigFiles(watcherCtx, func() {
		propertiesService.BroadcastProperties(context.Background())
	})

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		stopWatcher()
		connManager.Stop()
		srv.Shutdown(ctx)
		db.Close()
		cleanupDir()
	}

	// Create admin user
	if err := authService.CreateInitialAdminIfNone(context.Background(), DefaultAdminUser, DefaultAdminPass); err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("failed to create admin user: %w", err)
	}

	if err := connManager.Start(context.Background()); err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("failed to start mqtt connection manager: %w", err)
	}

	return &Env{
		ServerURL:         serverURL,
		AdminUser:         DefaultAdminUser,
		AdminPass:         DefaultAdminPass,
		ConfigDir:         configDir,
		DB:                db,
		ConnectionManager: connManager,
		WSCapture:         wsCapture,
		EmailCapture:      emailCapture,
	}, cleanup, nil
}

func writeFileOrErr(path, content string) {
	os.WriteFile(path, []byte(content), 0644)
}

// RecordingWSNotifier implements alerting.WebSocketNotifier and records every
// BroadcastToUser call. Used by integration tests to assert WS delivery.
type RecordingWSNotifier struct {
	mu      sync.Mutex
	userIDs []int
}

func (r *RecordingWSNotifier) BroadcastToUser(userID int, message interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.userIDs = append(r.userIDs, userID)
}

// UserIDs returns a copy of all user IDs that have been broadcast to.
func (r *RecordingWSNotifier) UserIDs() []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int(nil), r.userIDs...)
}

// Reset clears captured calls — call at the start of each test that asserts WS.
func (r *RecordingWSNotifier) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.userIDs = nil
}

// RecordingEmailNotifier implements alerting.EmailNotifier and records every
// SendNotification call. Used by integration tests to assert email delivery.
type RecordingEmailNotifier struct {
	mu         sync.Mutex
	recipients []string
}

func (r *RecordingEmailNotifier) SendNotification(recipient, title, message, category string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recipients = append(r.recipients, recipient)
	return nil
}

// Recipients returns a copy of all recipient email addresses.
func (r *RecordingEmailNotifier) Recipients() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.recipients...)
}

// Reset clears captured calls — call at the start of each test that asserts email.
func (r *RecordingEmailNotifier) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recipients = nil
}

type harnessNotifRepoAdapter struct {
	repo database.NotificationRepository
}

func (a *harnessNotifRepoAdapter) CreateNotification(ctx context.Context, notif notifications.Notification) (int, error) {
	return a.repo.CreateNotification(ctx, notif)
}

func (a *harnessNotifRepoAdapter) AssignNotificationToUsersWithPermission(ctx context.Context, notifID int, permission string) error {
	return a.repo.AssignNotificationToUsersWithPermission(ctx, notifID, permission)
}

func (a *harnessNotifRepoAdapter) GetUserIDsWithPermission(ctx context.Context, permission string) ([]int, error) {
	return a.repo.GetUserIDsWithPermission(ctx, permission)
}

func (a *harnessNotifRepoAdapter) GetUsersWithPermissionAndEmail(ctx context.Context, permission string) ([]alerting.UserEmailInfo, error) {
	users, err := a.repo.GetUsersWithPermissionAndEmail(ctx, permission)
	if err != nil {
		return nil, err
	}
	result := make([]alerting.UserEmailInfo, len(users))
	for i, u := range users {
		result[i] = alerting.UserEmailInfo{UserID: u.UserID, Email: u.Email}
	}
	return result, nil
}

func (a *harnessNotifRepoAdapter) GetChannelPreference(ctx context.Context, userID int, category notifications.NotificationCategory) (*notifications.ChannelPreference, error) {
	return a.repo.GetChannelPreference(ctx, userID, category)
}
