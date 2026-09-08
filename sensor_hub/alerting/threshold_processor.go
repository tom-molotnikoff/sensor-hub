package alerting

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"example/sensorHub/notifications"
)

// AlertRepository is the subset of db.AlertRepository needed by ThresholdAlertProcessor.
// Defined locally to avoid an import cycle (db imports alerting).
type AlertRepository interface {
	GetAlertRulesBySensorID(ctx context.Context, sensorID int) ([]AlertRule, error)
	RecordAlertSent(ctx context.Context, ruleID, sensorID, measurementTypeId int, reason string, numericValue float64, statusValue string) error
}

// UserEmailInfo holds a user's ID and email address for targeted email delivery.
type UserEmailInfo struct {
	UserID int
	Email  string
}

// NotificationRepository is the subset of db.NotificationRepository needed by ThresholdAlertProcessor.
// Defined locally to avoid an import cycle (db imports alerting).
type NotificationRepository interface {
	CreateNotification(ctx context.Context, notif notifications.Notification) (int, error)
	AssignNotificationToUsersWithPermission(ctx context.Context, notifID int, permission string) error
	GetUserIDsWithPermission(ctx context.Context, permission string) ([]int, error)
	GetUsersWithPermissionAndEmail(ctx context.Context, permission string) ([]UserEmailInfo, error)
	GetChannelPreference(ctx context.Context, userID int, category notifications.NotificationCategory) (*notifications.ChannelPreference, error)
}

// WebSocketNotifier sends real-time notification messages to connected users.
type WebSocketNotifier interface {
	BroadcastToUser(userID int, message interface{})
}

// EmailNotifier sends email notifications to individual recipients.
type EmailNotifier interface {
	SendNotification(recipient, title, message, category string) error
}

// ReadingAlert carries the values needed to evaluate one reading against alert rules.
type ReadingAlert struct {
	SensorID        int
	SensorName      string
	MeasurementType string
	NumericValue    float64
	StatusValue     string
}

// ThresholdAlertProcessor owns the threshold-alert workflow end-to-end: rule evaluation,
// rate-limiting, alert_history persistence, notification persistence, WebSocket fan-out,
// and email dispatch. It is constructed once and injected into SensorService.
type ThresholdAlertProcessor struct {
	alertRepo AlertRepository
	notifRepo NotificationRepository
	ws        WebSocketNotifier
	email     EmailNotifier
	logger    *slog.Logger
	mu        sync.Mutex
	lastFired map[int]time.Time // rule ID → last fire time (in-memory rate-limit state)
	rules     map[int][]AlertRule
}

// NewThresholdAlertProcessor constructs a ThresholdAlertProcessor. Call once at startup.
func NewThresholdAlertProcessor(
	alertRepo AlertRepository,
	notifRepo NotificationRepository,
	ws WebSocketNotifier,
	email EmailNotifier,
	logger *slog.Logger,
) *ThresholdAlertProcessor {
	return &ThresholdAlertProcessor{
		alertRepo: alertRepo,
		notifRepo: notifRepo,
		ws:        ws,
		email:     email,
		logger:    logger.With("component", "threshold_alert_processor"),
		lastFired: make(map[int]time.Time),
		rules:     make(map[int][]AlertRule),
	}
}

func (p *ThresholdAlertProcessor) InvalidateRules() {
	p.mu.Lock()
	clear(p.rules)
	p.mu.Unlock()
}

func (p *ThresholdAlertProcessor) rulesForSensor(ctx context.Context, sensorID int) ([]AlertRule, error) {
	p.mu.Lock()
	cached, ok := p.rules[sensorID]
	p.mu.Unlock()
	if ok {
		return cached, nil
	}

	rules, err := p.alertRepo.GetAlertRulesBySensorID(ctx, sensorID)
	if err != nil {
		return nil, err
	}
	if rules == nil {
		rules = []AlertRule{}
	}

	p.mu.Lock()
	p.rules[sensorID] = rules
	p.mu.Unlock()
	return rules, nil
}

func ruleForMeasurementType(rules []AlertRule, measurementType string) *AlertRule {
	for i := range rules {
		if rules[i].Enabled && strings.EqualFold(rules[i].MeasurementType, measurementType) {
			return &rules[i]
		}
	}
	return nil
}

// ProcessReading evaluates a sensor reading against configured alert rules and, if
// triggered and not rate-limited, persists an alert_history row, creates a notification,
// broadcasts via WebSocket, and dispatches email (best-effort async).
//
// DB persistence failures are returned as errors. WS errors are logged at WARN and
// broadcast continues for remaining users. Email errors are logged at ERROR; the goroutine
// never propagates them to the caller.
func (p *ThresholdAlertProcessor) ProcessReading(ctx context.Context, r ReadingAlert) error {
	rules, err := p.rulesForSensor(ctx, r.SensorID)
	if err != nil {
		return fmt.Errorf("failed to get alert rules for sensor %d: %w", r.SensorID, err)
	}
	rule := ruleForMeasurementType(rules, r.MeasurementType)
	if rule == nil {
		p.logger.Debug("no alert rule configured, skipping", "sensor", r.SensorName, "sensor_id", r.SensorID, "measurement_type", r.MeasurementType)
		return nil
	}

	shouldAlert, reason := rule.ShouldAlert(r.NumericValue, r.StatusValue)
	if !shouldAlert {
		p.logger.Debug("alert conditions not met", "sensor", r.SensorName, "sensor_id", r.SensorID, "value", r.NumericValue)
		return nil
	}

	// Rate-limit check: DB-based and in-memory under a lock to prevent TOCTOU races.
	p.mu.Lock()
	if rule.IsRateLimited() {
		p.mu.Unlock()
		p.logger.Debug("alert rate limited (DB)", "sensor", r.SensorName, "sensor_id", r.SensorID, "rule_id", rule.ID)
		return nil
	}
	if last, ok := p.lastFired[rule.ID]; ok && rule.RateLimitSeconds > 0 {
		if time.Since(last) < time.Duration(rule.RateLimitSeconds)*time.Second {
			p.mu.Unlock()
			p.logger.Debug("alert rate limited (in-memory)", "sensor", r.SensorName, "sensor_id", r.SensorID, "rule_id", rule.ID)
			return nil
		}
	}
	p.lastFired[rule.ID] = time.Now()
	p.mu.Unlock()

	p.logger.Info("triggering alert", "sensor", r.SensorName, "sensor_id", r.SensorID, "reason", reason, "value", r.NumericValue)

	if err := p.alertRepo.RecordAlertSent(ctx, rule.ID, r.SensorID, rule.MeasurementTypeId, reason, r.NumericValue, r.StatusValue); err != nil {
		p.logger.Error("failed to record alert sent", "sensor_name", r.SensorName, "rule_id", rule.ID, "error", err)
		return fmt.Errorf("failed to record alert sent: %w", err)
	}

	notif := notifications.Notification{
		Category: notifications.CategoryThresholdAlert,
		Severity: notifications.SeverityWarning,
		Title:    fmt.Sprintf("Alert: %s", r.SensorName),
		Message:  fmt.Sprintf("%s (value: %.2f)", reason, r.NumericValue),
		Metadata: map[string]interface{}{
			"sensor_name":   r.SensorName,
			"sensor_type":   r.MeasurementType,
			"numeric_value": r.NumericValue,
		},
	}

	notifID, err := p.notifRepo.CreateNotification(ctx, notif)
	if err != nil {
		p.logger.Error("failed to create notification", "sensor_name", r.SensorName, "rule_id", rule.ID, "error", err)
		return fmt.Errorf("failed to create notification: %w", err)
	}

	const targetPermission = "view_alerts"
	if err := p.notifRepo.AssignNotificationToUsersWithPermission(ctx, notifID, targetPermission); err != nil {
		p.logger.Error("failed to assign notification to users", "sensor_name", r.SensorName, "rule_id", rule.ID, "error", err)
		return fmt.Errorf("failed to assign notification to users: %w", err)
	}

	if p.ws != nil {
		userIDs, err := p.notifRepo.GetUserIDsWithPermission(ctx, targetPermission)
		if err != nil {
			p.logger.Warn("failed to get user IDs for WS broadcast", "sensor_name", r.SensorName, "error", err)
		} else {
			notif.ID = notifID
			for _, userID := range userIDs {
				p.ws.BroadcastToUser(userID, notif)
			}
		}
	}

	if p.email != nil {
		go p.sendEmailNotifications(context.Background(), notif, targetPermission)
	}

	p.logger.Info("alert sent", "sensor", r.SensorName, "sensor_id", r.SensorID, "reason", reason)
	return nil
}

func (p *ThresholdAlertProcessor) sendEmailNotifications(ctx context.Context, notif notifications.Notification, targetPermission string) {
	users, err := p.notifRepo.GetUsersWithPermissionAndEmail(ctx, targetPermission)
	if err != nil {
		p.logger.Error("failed to get users for email notification", "error", err)
		return
	}

	for _, user := range users {
		pref, err := p.notifRepo.GetChannelPreference(ctx, user.UserID, notif.Category)
		if err != nil {
			p.logger.Error("failed to get channel preference", "user_id", user.UserID, "error", err)
			continue
		}
		if pref == nil || !pref.EmailEnabled {
			continue
		}
		if err := p.email.SendNotification(user.Email, notif.Title, notif.Message, string(notif.Category)); err != nil {
			p.logger.Error("failed to send email notification", "email", user.Email, "error", err)
		}
	}
}
