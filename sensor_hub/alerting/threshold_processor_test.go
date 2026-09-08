package alerting_test

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"example/sensorHub/alerting"
	database "example/sensorHub/db"
	"example/sensorHub/notifications"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// ============================================================
// Test database setup
// ============================================================

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file::memory:?_pragma=foreign_keys(1)")
	require.NoError(t, err)
	// In-memory SQLite: without this, the pool may open extra connections that see an
	// empty (un-migrated) database. Single connection ensures all goroutines share the
	// same schema and matches SQLite's single-writer model.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, database.RunMigrations(db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	return db
}

func testHandles(db *sql.DB) *database.Handles {
	return &database.Handles{Reader: db, Writer: db}
}

// ============================================================
// Recording stubs for system-boundary interfaces
// ============================================================

type recordingWS struct {
	mu    sync.Mutex
	calls []wsCall
}

type wsCall struct {
	userID  int
	message interface{}
}

func (r *recordingWS) BroadcastToUser(userID int, message interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, wsCall{userID, message})
}

func (r *recordingWS) broadcastCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

type recordingEmail struct {
	mu    sync.Mutex
	calls []emailCall
	err   error // if set, all sends return this error
}

type emailCall struct {
	recipient, title, message, category string
}

func (r *recordingEmail) SendNotification(recipient, title, message, category string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, emailCall{recipient, title, message, category})
	return r.err
}

func (r *recordingEmail) sendCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

// ============================================================
// notifRepoAdapter adapts *database.SqlNotificationRepository to alerting.NotificationRepository.
// The only difference is GetUsersWithPermissionAndEmail returns []alerting.UserEmailInfo
// instead of []database.UserEmailInfo.
// ============================================================

type notifRepoAdapter struct {
	inner database.NotificationRepository
}

func (a *notifRepoAdapter) CreateNotification(ctx context.Context, notif notifications.Notification) (int, error) {
	return a.inner.CreateNotification(ctx, notif)
}

func (a *notifRepoAdapter) AssignNotificationToUsersWithPermission(ctx context.Context, notifID int, permission string) error {
	return a.inner.AssignNotificationToUsersWithPermission(ctx, notifID, permission)
}

func (a *notifRepoAdapter) GetUserIDsWithPermission(ctx context.Context, permission string) ([]int, error) {
	return a.inner.GetUserIDsWithPermission(ctx, permission)
}

func (a *notifRepoAdapter) GetUsersWithPermissionAndEmail(ctx context.Context, permission string) ([]alerting.UserEmailInfo, error) {
	users, err := a.inner.GetUsersWithPermissionAndEmail(ctx, permission)
	if err != nil {
		return nil, err
	}
	result := make([]alerting.UserEmailInfo, len(users))
	for i, u := range users {
		result[i] = alerting.UserEmailInfo{UserID: u.UserID, Email: u.Email}
	}
	return result, nil
}

func (a *notifRepoAdapter) GetChannelPreference(ctx context.Context, userID int, category notifications.NotificationCategory) (*notifications.ChannelPreference, error) {
	return a.inner.GetChannelPreference(ctx, userID, category)
}

// ============================================================
// Test data helpers
// ============================================================

func newProcessor(t *testing.T, db *sql.DB, ws alerting.WebSocketNotifier, email alerting.EmailNotifier) *alerting.ThresholdAlertProcessor {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	alertRepo := database.NewAlertRepository(testHandles(db), logger)
	notifRepo := &notifRepoAdapter{inner: database.NewNotificationRepository(testHandles(db), logger)}
	return alerting.NewThresholdAlertProcessor(alertRepo, notifRepo, ws, email, logger)
}

func insertSensor(t *testing.T, db *sql.DB, name string) int {
	t.Helper()
	res, err := db.ExecContext(context.Background(),
		"INSERT INTO sensors (name, health_status, enabled) VALUES (?, 'unknown', 1)", name)
	require.NoError(t, err)
	id, _ := res.LastInsertId()
	return int(id)
}

func getMeasurementTypeID(t *testing.T, db *sql.DB, name string) int {
	t.Helper()
	var id int
	err := db.QueryRowContext(context.Background(), "SELECT id FROM measurement_types WHERE name = ?", name).Scan(&id)
	require.NoError(t, err)
	return id
}

func insertNumericAlertRule(t *testing.T, db *sql.DB, sensorID, mtID int, high, low float64, enabled bool, rateLimitSecs int) int {
	t.Helper()
	res, err := db.ExecContext(context.Background(),
		`INSERT INTO sensor_alert_rules (sensor_id, measurement_type_id, alert_type, high_threshold, low_threshold, trigger_status, enabled, rate_limit_seconds)
		 VALUES (?, ?, 'numeric_range', ?, ?, '', ?, ?)`,
		sensorID, mtID, high, low, enabled, rateLimitSecs)
	require.NoError(t, err)
	id, _ := res.LastInsertId()
	return int(id)
}

func insertStatusAlertRule(t *testing.T, db *sql.DB, sensorID, mtID int, triggerStatus string, rateLimitSecs int) int {
	t.Helper()
	res, err := db.ExecContext(context.Background(),
		`INSERT INTO sensor_alert_rules (sensor_id, measurement_type_id, alert_type, high_threshold, low_threshold, trigger_status, enabled, rate_limit_seconds)
		 VALUES (?, ?, 'status_based', 0, 0, ?, 1, ?)`,
		sensorID, mtID, triggerStatus, rateLimitSecs)
	require.NoError(t, err)
	id, _ := res.LastInsertId()
	return int(id)
}

// insertUserWithRole creates a user, assigns the named role, and returns the user ID.
// The admin role has all permissions (including view_alerts) seeded by the migration.
func insertUserWithRole(t *testing.T, db *sql.DB, username, email, role string) int {
	t.Helper()
	res, err := db.ExecContext(context.Background(),
		"INSERT INTO users (username, email, password_hash, must_change_password, disabled) VALUES (?, ?, 'hash', 0, 0)",
		username, email)
	require.NoError(t, err)
	userID, _ := res.LastInsertId()

	var roleID int
	err = db.QueryRowContext(context.Background(), "SELECT id FROM roles WHERE name = ?", role).Scan(&roleID)
	require.NoError(t, err)

	_, err = db.ExecContext(context.Background(), "INSERT INTO user_roles (user_id, role_id) VALUES (?, ?)", userID, roleID)
	require.NoError(t, err)
	return int(userID)
}

func setEmailPreference(t *testing.T, db *sql.DB, userID int, category notifications.NotificationCategory, emailEnabled bool) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO notification_channel_preferences (user_id, category, email_enabled, inapp_enabled)
		 VALUES (?, ?, ?, 1)
		 ON CONFLICT(user_id, category) DO UPDATE SET email_enabled = excluded.email_enabled`,
		userID, category, emailEnabled)
	require.NoError(t, err)
}

func countAlertHistory(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM alert_sent_history").Scan(&n))
	return n
}

func countNotifications(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM notifications").Scan(&n))
	return n
}

func countUserNotifications(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM user_notifications").Scan(&n))
	return n
}

// ============================================================
// Tracer bullet: no rule → processor does nothing
// ============================================================

func TestProcessReading_noRule_doesNothing(t *testing.T) {
	db := newTestDB(t)
	ws := &recordingWS{}
	email := &recordingEmail{}
	p := newProcessor(t, db, ws, email)

	err := p.ProcessReading(context.Background(), alerting.ReadingAlert{
		SensorID:        999,
		SensorName:      "ghost-sensor",
		MeasurementType: "temperature",
		NumericValue:    35.0,
	})

	assert.NoError(t, err)
	assert.Equal(t, 0, countAlertHistory(t, db))
	assert.Equal(t, 0, countNotifications(t, db))
	assert.Equal(t, 0, ws.broadcastCount())
	assert.Equal(t, 0, email.sendCount())
}

// ============================================================
// Below threshold → does nothing
// ============================================================

func TestProcessReading_belowThreshold_doesNothing(t *testing.T) {
	db := newTestDB(t)
	sensorID := insertSensor(t, db, "living-room")
	mtID := getMeasurementTypeID(t, db, "temperature")
	insertNumericAlertRule(t, db, sensorID, mtID, 30.0, 10.0, true, 0)

	ws := &recordingWS{}
	email := &recordingEmail{}
	p := newProcessor(t, db, ws, email)

	err := p.ProcessReading(context.Background(), alerting.ReadingAlert{
		SensorID:        sensorID,
		SensorName:      "living-room",
		MeasurementType: "temperature",
		NumericValue:    22.0, // within [10, 30]
	})

	assert.NoError(t, err)
	assert.Equal(t, 0, countAlertHistory(t, db))
	assert.Equal(t, 0, countNotifications(t, db))
	assert.Equal(t, 0, ws.broadcastCount())
}

// ============================================================
// Disabled rule is filtered by DB → processor sees no rule → does nothing
// ============================================================

func TestProcessReading_disabledRule_doesNothing(t *testing.T) {
	db := newTestDB(t)
	sensorID := insertSensor(t, db, "bedroom")
	mtID := getMeasurementTypeID(t, db, "temperature")
	insertNumericAlertRule(t, db, sensorID, mtID, 30.0, 10.0, false /* disabled */, 0)

	ws := &recordingWS{}
	email := &recordingEmail{}
	p := newProcessor(t, db, ws, email)

	err := p.ProcessReading(context.Background(), alerting.ReadingAlert{
		SensorID:        sensorID,
		SensorName:      "bedroom",
		MeasurementType: "temperature",
		NumericValue:    99.0, // above threshold, but rule is disabled
	})

	assert.NoError(t, err)
	assert.Equal(t, 0, countAlertHistory(t, db))
	assert.Equal(t, 0, countNotifications(t, db))
}

// ============================================================
// Happy path: above threshold → writes alert_history, notification, user_notifications, broadcasts
// ============================================================

func TestProcessReading_aboveThreshold_writesAlertAndNotification(t *testing.T) {
	db := newTestDB(t)
	sensorID := insertSensor(t, db, "boiler-room")
	mtID := getMeasurementTypeID(t, db, "temperature")
	insertNumericAlertRule(t, db, sensorID, mtID, 30.0, 10.0, true, 0)
	userID := insertUserWithRole(t, db, "alice", "alice@example.com", "admin")

	ws := &recordingWS{}
	email := &recordingEmail{}
	p := newProcessor(t, db, ws, email)

	err := p.ProcessReading(context.Background(), alerting.ReadingAlert{
		SensorID:        sensorID,
		SensorName:      "boiler-room",
		MeasurementType: "temperature",
		NumericValue:    35.0, // above 30
	})

	require.NoError(t, err)
	assert.Equal(t, 1, countAlertHistory(t, db))
	assert.Equal(t, 1, countNotifications(t, db))
	assert.Equal(t, 1, countUserNotifications(t, db))

	// WS broadcast fired for alice
	assert.Equal(t, 1, ws.broadcastCount())
	assert.Equal(t, userID, ws.calls[0].userID)

	// Email is async; wait briefly and verify it was attempted for alice (default email_enabled=true)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, email.sendCount())
	assert.Equal(t, "alice@example.com", email.calls[0].recipient)
}

// ============================================================
// Status-based alert fires on matching status
// ============================================================

func TestProcessReading_statusBasedAlert_fires(t *testing.T) {
	db := newTestDB(t)
	sensorID := insertSensor(t, db, "front-door")
	mtID := getMeasurementTypeID(t, db, "contact")
	insertStatusAlertRule(t, db, sensorID, mtID, "open", 0)
	insertUserWithRole(t, db, "bob", "bob@example.com", "admin")

	ws := &recordingWS{}
	email := &recordingEmail{}
	p := newProcessor(t, db, ws, email)

	err := p.ProcessReading(context.Background(), alerting.ReadingAlert{
		SensorID:        sensorID,
		SensorName:      "front-door",
		MeasurementType: "contact",
		StatusValue:     "open",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, countAlertHistory(t, db))
	assert.Equal(t, 1, countNotifications(t, db))
	assert.Equal(t, 1, countUserNotifications(t, db))
	assert.Equal(t, 1, ws.broadcastCount())
}

// ============================================================
// Rate-limited by DB (LastAlertSentAt within window) → skips delivery
// ============================================================

func TestProcessReading_rateLimited_skipsDelivery(t *testing.T) {
	db := newTestDB(t)
	sensorID := insertSensor(t, db, "garage")
	mtID := getMeasurementTypeID(t, db, "temperature")
	ruleID := insertNumericAlertRule(t, db, sensorID, mtID, 30.0, 10.0, true, 3600 /* 1 hour */)

	// Seed alert_sent_history so the DB rate-limit check fires
	_, err := db.ExecContext(context.Background(),
		"INSERT INTO alert_sent_history (alert_rule_id, sensor_id, measurement_type_id, alert_reason, reading_value, reading_status) VALUES (?, ?, ?, 'test', 35.0, '')",
		ruleID, sensorID, mtID)
	require.NoError(t, err)

	ws := &recordingWS{}
	email := &recordingEmail{}
	p := newProcessor(t, db, ws, email)

	err = p.ProcessReading(context.Background(), alerting.ReadingAlert{
		SensorID:        sensorID,
		SensorName:      "garage",
		MeasurementType: "temperature",
		NumericValue:    35.0,
	})

	require.NoError(t, err)
	// First record already in history; processor should have skipped before writing another
	assert.Equal(t, 1, countAlertHistory(t, db))
	assert.Equal(t, 0, countNotifications(t, db))
	assert.Equal(t, 0, ws.broadcastCount())
}

// ============================================================
// Concurrent reads for the same rule fire exactly once (in-memory rate limit)
// ============================================================

func TestProcessReading_concurrentSameRule_firesOnce(t *testing.T) {
	db := newTestDB(t)
	sensorID := insertSensor(t, db, "attic")
	mtID := getMeasurementTypeID(t, db, "temperature")
	insertNumericAlertRule(t, db, sensorID, mtID, 30.0, 10.0, true, 3600)

	ws := &recordingWS{}
	email := &recordingEmail{}
	p := newProcessor(t, db, ws, email)

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			_ = p.ProcessReading(context.Background(), alerting.ReadingAlert{
				SensorID:        sensorID,
				SensorName:      "attic",
				MeasurementType: "temperature",
				NumericValue:    35.0,
			})
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, countAlertHistory(t, db), "exactly one alert_history row expected")
	assert.Equal(t, 1, countNotifications(t, db))
}

// ============================================================
// Email gated by channel preference
// ============================================================

func TestProcessReading_emailGatedByPreference(t *testing.T) {
	db := newTestDB(t)
	sensorID := insertSensor(t, db, "kitchen")
	mtID := getMeasurementTypeID(t, db, "temperature")
	insertNumericAlertRule(t, db, sensorID, mtID, 30.0, 10.0, true, 0)

	userA := insertUserWithRole(t, db, "user-a", "a@example.com", "admin") // email on (default)
	userB := insertUserWithRole(t, db, "user-b", "b@example.com", "admin") // email off
	setEmailPreference(t, db, userB, notifications.CategoryThresholdAlert, false)

	ws := &recordingWS{}
	email := &recordingEmail{}
	p := newProcessor(t, db, ws, email)

	err := p.ProcessReading(context.Background(), alerting.ReadingAlert{
		SensorID:        sensorID,
		SensorName:      "kitchen",
		MeasurementType: "temperature",
		NumericValue:    35.0,
	})

	require.NoError(t, err)

	// Both users get in-app (2 user_notifications, 2 WS broadcasts)
	assert.Equal(t, 2, countUserNotifications(t, db))
	assert.Equal(t, 2, ws.broadcastCount())

	// Only user-a gets email
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, email.sendCount())
	assert.Equal(t, "a@example.com", email.calls[0].recipient)
	_ = userA
}

// ============================================================
// Notification persist failure propagates error
// ============================================================

func TestProcessReading_notificationPersistFails_propagatesError(t *testing.T) {
	db := newTestDB(t)
	sensorID := insertSensor(t, db, "basement")
	mtID := getMeasurementTypeID(t, db, "temperature")
	insertNumericAlertRule(t, db, sensorID, mtID, 30.0, 10.0, true, 0)

	// Use a failing notif repo stub that errors on CreateNotification
	failingNotifRepo := &failingCreateNotificationRepo{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	alertRepo := database.NewAlertRepository(testHandles(db), logger)
	p := alerting.NewThresholdAlertProcessor(alertRepo, failingNotifRepo, nil, nil, logger)

	err := p.ProcessReading(context.Background(), alerting.ReadingAlert{
		SensorID:        sensorID,
		SensorName:      "basement",
		MeasurementType: "temperature",
		NumericValue:    35.0,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create notification")
	// alert_history is written before the notification attempt
	assert.Equal(t, 1, countAlertHistory(t, db))
}

// failingCreateNotificationRepo satisfies alerting.NotificationRepository but errors on CreateNotification.
type failingCreateNotificationRepo struct{}

var errCreateFailed = errors.New("db: simulated write failure")

func (f *failingCreateNotificationRepo) CreateNotification(_ context.Context, _ notifications.Notification) (int, error) {
	return 0, errCreateFailed
}
func (f *failingCreateNotificationRepo) AssignNotificationToUsersWithPermission(_ context.Context, _ int, _ string) error {
	return nil
}
func (f *failingCreateNotificationRepo) GetUserIDsWithPermission(_ context.Context, _ string) ([]int, error) {
	return nil, nil
}
func (f *failingCreateNotificationRepo) GetUsersWithPermissionAndEmail(_ context.Context, _ string) ([]alerting.UserEmailInfo, error) {
	return nil, nil
}
func (f *failingCreateNotificationRepo) GetChannelPreference(_ context.Context, _ int, _ notifications.NotificationCategory) (*notifications.ChannelPreference, error) {
	return nil, nil
}

// ============================================================
// Email send failure does not abort the batch or return an error
// ============================================================

func TestProcessReading_emailSendFails_doesNotAbortBatch(t *testing.T) {
	db := newTestDB(t)
	sensorID := insertSensor(t, db, "server-room")
	mtID := getMeasurementTypeID(t, db, "temperature")
	insertNumericAlertRule(t, db, sensorID, mtID, 30.0, 10.0, true, 0)
	insertUserWithRole(t, db, "carol", "carol@example.com", "admin")

	ws := &recordingWS{}
	email := &recordingEmail{err: errors.New("SMTP connection refused")}
	p := newProcessor(t, db, ws, email)

	err := p.ProcessReading(context.Background(), alerting.ReadingAlert{
		SensorID:        sensorID,
		SensorName:      "server-room",
		MeasurementType: "temperature",
		NumericValue:    35.0,
	})

	// ProcessReading must return nil even when email fails
	require.NoError(t, err)

	// In-app notification and WS broadcast must still complete
	assert.Equal(t, 1, countAlertHistory(t, db))
	assert.Equal(t, 1, countNotifications(t, db))
	assert.Equal(t, 1, countUserNotifications(t, db))
	assert.Equal(t, 1, ws.broadcastCount())

	// Email was attempted (and failed), but that's best-effort
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, email.sendCount())
}

type countingAlertRepo struct {
	inner alerting.AlertRepository
	mu    sync.Mutex
	reads int
}

func (c *countingAlertRepo) GetAlertRulesBySensorID(ctx context.Context, sensorID int) ([]alerting.AlertRule, error) {
	c.mu.Lock()
	c.reads++
	c.mu.Unlock()
	return c.inner.GetAlertRulesBySensorID(ctx, sensorID)
}

func (c *countingAlertRepo) RecordAlertSent(ctx context.Context, ruleID, sensorID, measurementTypeId int, reason string, numericValue float64, statusValue string) error {
	return c.inner.RecordAlertSent(ctx, ruleID, sensorID, measurementTypeId, reason, numericValue, statusValue)
}

func (c *countingAlertRepo) readCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reads
}

func newCountingProcessor(t *testing.T, db *sql.DB) (*alerting.ThresholdAlertProcessor, *countingAlertRepo) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	counting := &countingAlertRepo{inner: database.NewAlertRepository(testHandles(db), logger)}
	notifRepo := &notifRepoAdapter{inner: database.NewNotificationRepository(testHandles(db), logger)}
	return alerting.NewThresholdAlertProcessor(counting, notifRepo, nil, nil, logger), counting
}

func TestProcessReading_readsTheRulesForASensorOnce(t *testing.T) {
	db := newTestDB(t)
	sensorID := insertSensor(t, db, "living-room")
	mtID := getMeasurementTypeID(t, db, "temperature")
	insertNumericAlertRule(t, db, sensorID, mtID, 30.0, 10.0, true, 0)
	p, repo := newCountingProcessor(t, db)

	for range 9 {
		require.NoError(t, p.ProcessReading(context.Background(), alerting.ReadingAlert{
			SensorID:        sensorID,
			SensorName:      "living-room",
			MeasurementType: "temperature",
			NumericValue:    20.0,
		}))
	}

	assert.Equal(t, 1, repo.readCount(), "nine readings, one rule read")
}

func TestProcessReading_readsTheRulesAgainAfterTheyAreInvalidated(t *testing.T) {
	db := newTestDB(t)
	sensorID := insertSensor(t, db, "living-room")
	mtID := getMeasurementTypeID(t, db, "temperature")
	insertNumericAlertRule(t, db, sensorID, mtID, 30.0, 10.0, true, 0)
	p, repo := newCountingProcessor(t, db)

	reading := alerting.ReadingAlert{
		SensorID:        sensorID,
		SensorName:      "living-room",
		MeasurementType: "temperature",
		NumericValue:    20.0,
	}
	require.NoError(t, p.ProcessReading(context.Background(), reading))
	require.Equal(t, 1, repo.readCount())

	p.InvalidateRules()
	require.NoError(t, p.ProcessReading(context.Background(), reading))

	assert.Equal(t, 2, repo.readCount(), "a rule write sends the next reading back to the database")
}
