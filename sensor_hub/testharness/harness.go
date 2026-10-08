//go:build integration

package testharness

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	"example/sensorHub/automation"
	database "example/sensorHub/db"
	_ "example/sensorHub/drivers" // register sensor drivers
	gen "example/sensorHub/gen"
	mqttpkg "example/sensorHub/mqtt"
	"example/sensorHub/notifications"
	"example/sensorHub/readings"
	"example/sensorHub/service"
	"example/sensorHub/smtp"
	"example/sensorHub/testharness/fixtures"
	"example/sensorHub/web"
	"example/sensorHub/ws"

	"github.com/gin-gonic/gin"
)

// Env holds references to the running test server and its components.
type Env struct {
	ServerURL         string
	AdminUser         string
	AdminPass         string
	ConfigDir         string
	DB                *database.Handles
	Readings          *readings.Pipeline
	ConnectionManager *mqttpkg.ConnectionManager
	WSCapture         *RecordingWSNotifier
	EmailCapture      *RecordingEmailNotifier

	// MQTTBrokerAddress is where devices reach the hub's embedded broker, and
	// MQTTClient is a credential it accepts, limited to zigbee2mqtt/.
	MQTTBrokerAddress string
	MQTTClient        fixtures.MQTTClient

	ui         fs.FS
	listenAddr string
	stop       func()
}

const (
	DefaultAdminUser = "testadmin"
	DefaultAdminPass = "testpassword123"
)

var DefaultMQTTClient = fixtures.MQTTClient{Name: "zigbee2mqtt", TopicPrefix: "zigbee2mqtt/", Password: "testmqttpassword"}

// StartServer creates a temp DB, wires up all services, starts the Gin server
// on a random port, and creates an admin user. Cleanup via t.Cleanup.
func StartServer(t interface {
	Helper()
	Fatalf(string, ...any)
	Cleanup(func())
}, sensorURLs []string) *Env {
	env, cleanup, err := startServer(serverOptions{})
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
	return startServer(serverOptions{})
}

type serverOptions struct {
	seedPath   string
	ui         fs.FS
	listenAddr string
}

func startServer(opts serverOptions) (*Env, func(), error) {
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

	if opts.seedPath != "" {
		if err := copyFile(opts.seedPath, dbPath); err != nil {
			cleanupDir()
			return nil, func() {}, fmt.Errorf("failed to copy seed database: %w", err)
		}
	}

	mqttBrokerPort, err := freeTCPPort()
	if err != nil {
		cleanupDir()
		return nil, func() {}, fmt.Errorf("failed to reserve a port for the embedded broker: %w", err)
	}

	// Write minimal config files. Loopback is a trusted proxy, as nginx is on a
	// packaged install, so a test can stand in for the proxy.
	appPropsContent := fmt.Sprintf(
		"sensor.collection.interval=300\ndatabase.path=%s\nlog.level=debug\nauth.bcrypt.cost=4\nmqtt.broker.enabled=true\nmqtt.broker.port=%d\nhttp.trusted.proxies=127.0.0.1,::1\n", dbPath, mqttBrokerPort)
	writeFileOrErr(filepath.Join(configDir, "application.properties"), appPropsContent)
	writeFileOrErr(filepath.Join(configDir, "database.properties"), fmt.Sprintf("database.path=%s\n", dbPath))
	writeFileOrErr(filepath.Join(configDir, "smtp.properties"), "smtp.user=\n")

	env := &Env{
		AdminUser:         DefaultAdminUser,
		AdminPass:         DefaultAdminPass,
		ConfigDir:         configDir,
		MQTTBrokerAddress: fmt.Sprintf("127.0.0.1:%d", mqttBrokerPort),
		MQTTClient:        DefaultMQTTClient,
		ui:                opts.ui,
	}
	listenAddr := opts.listenAddr
	if listenAddr == "" {
		listenAddr = "127.0.0.1:0"
	}
	if err := env.boot(listenAddr); err != nil {
		cleanupDir()
		return nil, func() {}, err
	}
	return env, func() {
		env.stop()
		cleanupDir()
	}, nil
}

// Restart stops the hub and starts it again on the same database and address,
// as a hub restarted for an update would be. Logins survive it.
func (e *Env) Restart() error {
	e.stop()
	return e.boot(e.listenAddr)
}

func (e *Env) boot(listenAddr string) error {
	if err := appProps.InitialiseConfig(e.ConfigDir); err != nil {
		return fmt.Errorf("failed to initialise config: %w", err)
	}

	logger := slog.Default()

	db, err := database.Open(appProps.AppConfig(), logger)
	if err != nil {
		return fmt.Errorf("failed to initialise database: %w", err)
	}

	// As in cmd/local_serve.go, the embedded broker authenticates devices
	// against the MQTT clients in the database.
	mqttClientRepo := database.NewMQTTClientRepository(db, logger)
	if err := fixtures.EnsureMQTTClient(context.Background(), mqttClientRepo, e.MQTTClient); err != nil {
		db.Close()
		return err
	}
	mqttClientService := service.NewMQTTClientService(mqttClientRepo, logger)
	embeddedBroker := mqttpkg.NewEmbeddedBroker(mqttpkg.BrokerConfig{
		TCPAddress: fmt.Sprintf(":%d", appProps.AppConfig().MQTTBrokerPort),
	}, mqttClientService, logger)
	mqttClientService.SetSessions(embeddedBroker)
	if err := embeddedBroker.Start(); err != nil {
		db.Close()
		return fmt.Errorf("failed to start embedded MQTT broker: %w", err)
	}

	// Build the full service graph, mirroring cmd/local_serve.go
	sensorRepo := database.NewSensorRepository(db, logger)
	mtRepo := database.NewMeasurementTypeRepository(db, logger)
	readingsRepo := database.NewReadingsRepository(db, sensorRepo, mtRepo, logger)
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
	readingsSampler := service.NewReadingsSampler(readingsRepo, logger)
	if err := readingsSampler.Sample(context.Background()); err != nil {
		_ = embeddedBroker.Stop()
		db.Close()
		return fmt.Errorf("failed to sample readings row counts: %w", err)
	}
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

	tiers := service.DefaultAggregationTiers
	readingsService := service.NewReadingsService(readingsRepo, mtRepo, tiers, appProps.AppConfig().ReadingsAggregationEnabled, logger)
	propertiesService := service.NewPropertiesService(logger)
	maintenanceRepo := database.NewMaintenanceRepository(db)
	automationRepo := database.NewAutomationRepository(db, logger)
	_ = service.NewCleanupService(sensorRepo, readingsRepo, failedRepo, notificationRepo, alertRepo, automationRepo, commandHistoryRepo, maintenanceRepo, readingsSampler, logger)

	userService := service.NewUserService(userRepo, notificationService, logger)
	authService := service.NewAuthService(userRepo, sessionRepo, failedRepo, logger)
	roleService := service.NewRoleService(roleRepo, logger)
	alertManagementService := service.NewAlertManagementService(alertRepo, thresholdProcessor, logger)
	apiKeyService := service.NewApiKeyService(apiKeyRepo, userRepo, roleRepo, logger)

	// Init middleware
	middleware.InitAuthMiddleware(authService)
	middleware.InitApiKeyMiddleware(apiKeyService)

	dashboardRepo := database.NewDashboardRepository(db, logger)
	dashboardService := service.NewDashboardService(dashboardRepo, logger)

	mqttBrokerRepo := database.NewMQTTBrokerRepository(db, logger)
	mqttSubRepo := database.NewMQTTSubscriptionRepository(db, logger)
	mqttService := service.NewMQTTService(mqttBrokerRepo, mqttSubRepo, logger)
	connManager := mqttpkg.NewConnectionManager(sensorService, mqttSubRepo, mqttBrokerRepo, embeddedBroker, logger)
	mqttService.SetSubscriptionNotifier(connManager)
	commandService := service.NewCommandService(sensorRepo, mqttSubRepo, commandHistoryRepo, connManager, commandTracker, logger)
	if err := commandTracker.RecoverPending(context.Background()); err != nil {
		_ = embeddedBroker.Stop()
		db.Close()
		return fmt.Errorf("failed to recover pending commands: %w", err)
	}
	automationService := automation.NewService(automationRepo, sensorService, commandService, notificationService, automationReadings, readingsRepo, logger)
	sensorService.SetSensorObserver(automationService)

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
		nil, // no OAuth in tests
		connManager,
		automationService,
	)

	gin.SetMode(gin.TestMode)
	router, err := api.NewEngine(appProps.AppConfig().HTTPTrustedProxies)
	if err != nil {
		_ = embeddedBroker.Stop()
		db.Close()
		return err
	}
	api.RegisterAPIRoutes(router, server)

	if e.ui != nil {
		web.RegisterSPAHandlerFS(router, e.ui)
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		_ = embeddedBroker.Stop()
		db.Close()
		return fmt.Errorf("failed to listen: %w", err)
	}
	serverURL := fmt.Sprintf("http://%s", listener.Addr().String())

	srv := &http.Server{Handler: router}
	go srv.Serve(listener)

	// Mirror cmd/local_serve.go: watch for external config edits and broadcast
	// reloads to properties websocket subscribers.
	watcherCtx, stopWatcher := context.WithCancel(context.Background())
	appProps.WatchConfigFiles(watcherCtx, func() {
		propertiesService.BroadcastProperties(context.Background())
	})

	automationCtx, stopAutomations := context.WithCancel(context.Background())
	e.stop = func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		stopAutomations()
		stopWatcher()
		connManager.Stop()
		srv.Shutdown(ctx)
		_ = embeddedBroker.Stop()
		db.Close()
	}
	e.listenAddr = listener.Addr().String()
	e.ServerURL = serverURL
	e.DB = db
	e.Readings = readingPipeline
	e.ConnectionManager = connManager
	e.WSCapture = wsCapture
	e.EmailCapture = emailCapture

	adminHash, err := service.HashFirstAdminPassword(DefaultAdminPass)
	if err != nil {
		e.stop()
		return fmt.Errorf("failed to hash admin password: %w", err)
	}
	admin := gen.User{Username: DefaultAdminUser, MustChangePassword: true}
	_, err = userRepo.CreateFirstAdmin(context.Background(), admin, adminHash)
	if err != nil && !errors.Is(err, database.ErrAdminExists) {
		e.stop()
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	if err := connManager.Start(context.Background()); err != nil {
		e.stop()
		return fmt.Errorf("failed to start mqtt connection manager: %w", err)
	}

	// As in cmd/local_serve.go, automations start once MQTT is connected, so a run
	// resumed on startup can send its commands.
	if err := automationService.Start(automationCtx); err != nil {
		e.stop()
		return fmt.Errorf("failed to start automations: %w", err)
	}
	return nil
}

func freeTCPPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
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
