package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"example/sensorHub/alerting"
	"example/sensorHub/notifications"
)

type seededRule struct {
	device      string
	measurement string
	alertType   alerting.AlertType
	low, high   float64
	trigger     string
	rateLimit   time.Duration
}

var seededRules = []seededRule{
	{device: "living-room-sensor", measurement: "temperature", alertType: alerting.AlertTypeNumericRange, low: 18, high: 25, rateLimit: time.Hour},
	{device: "kitchen-sensor", measurement: "humidity", alertType: alerting.AlertTypeNumericRange, low: 35, high: 65, rateLimit: time.Hour},
	{device: "front-door", measurement: "contact", alertType: alerting.AlertTypeStatusBased, trigger: stateOff, rateLimit: 6 * time.Hour},
}

const notificationPageSize = 1000

type seededNotification struct {
	notification notifications.Notification
	permission   string
	read         bool
}

var seededNotifications = []seededNotification{
	alertNotification("living-room-sensor", "value 25.14 is above high threshold 25.00", 25.14, true),
	userNotification("created", "viewer", true),
	sensorNotification("added", "office-plug", true),
	alertNotification("front-door", "status is false", 0, true),
	alertNotification("kitchen-sensor", "value 34.62 is below low threshold 35.00", 34.62, false),
	sensorNotification("updated", "hallway-motion", false),
	alertNotification("living-room-sensor", "value 17.86 is below low threshold 18.00", 17.86, false),
}

func alertNotification(sensor, reason string, reading float64, read bool) seededNotification {
	return seededNotification{
		notification: notifications.Notification{
			Category: notifications.CategoryThresholdAlert,
			Severity: notifications.SeverityWarning,
			Title:    "Alert: " + sensor,
			Message:  fmt.Sprintf("%s (value: %.2f)", reason, reading),
		},
		permission: "view_alerts",
		read:       read,
	}
}

func userNotification(action, username string, read bool) seededNotification {
	return seededNotification{
		notification: notifications.Notification{
			Category: notifications.CategoryUserManagement,
			Severity: notifications.SeverityInfo,
			Title:    "User " + action,
			Message:  fmt.Sprintf("User '%s' was %s", username, action),
		},
		permission: "view_notifications_user_mgmt",
		read:       read,
	}
}

func sensorNotification(action, sensor string, read bool) seededNotification {
	return seededNotification{
		notification: notifications.Notification{
			Category: notifications.CategoryConfigChange,
			Severity: notifications.SeverityInfo,
			Title:    "Sensor " + action,
			Message:  fmt.Sprintf("Sensor '%s' was %s", sensor, action),
		},
		permission: "view_notifications_config",
		read:       read,
	}
}

func (s *seeder) createAlertRules(ctx context.Context, sensorIDs map[string]int) error {
	typeIDs, err := s.measurementTypeIDs(ctx)
	if err != nil {
		return err
	}
	for _, seeded := range seededRules {
		sensorID := sensorIDs[seeded.device]
		existing, err := s.alerts.ServiceGetAlertRulesBySensorID(ctx, sensorID)
		if err != nil {
			return err
		}
		if hasRule(existing, seeded) {
			s.logger.Info("alert rule exists", "sensor", seeded.device, "measurement", seeded.measurement)
			continue
		}
		rule := &alerting.AlertRule{
			SensorID:          sensorID,
			MeasurementTypeId: typeIDs[seeded.measurement],
			AlertType:         seeded.alertType,
			HighThreshold:     seeded.high,
			LowThreshold:      seeded.low,
			TriggerStatus:     seeded.trigger,
			Enabled:           true,
			RateLimitSeconds:  int(seeded.rateLimit.Seconds()),
		}
		if err := rule.Validate(); err != nil {
			return fmt.Errorf("alert rule on %s %s: %w", seeded.device, seeded.measurement, err)
		}
		if err := s.alerts.ServiceCreateAlertRule(ctx, rule); err != nil {
			return err
		}
		s.logger.Info("created alert rule", "sensor", seeded.device, "measurement", seeded.measurement, "type", seeded.alertType)
	}
	return nil
}

func hasRule(rules []alerting.AlertRule, seeded seededRule) bool {
	for _, rule := range rules {
		if strings.EqualFold(rule.MeasurementType, seeded.measurement) && rule.AlertType == seeded.alertType {
			return true
		}
	}
	return false
}

func (s *seeder) createNotifications(ctx context.Context, userIDs map[string]int) error {
	received, err := s.receivedNotifications(ctx, userIDs[adminUsername])
	if err != nil {
		return err
	}
	for _, seeded := range seededNotifications {
		if _, exists := received[seeded.notification.Message]; exists {
			continue
		}
		if _, err := s.notifications.CreateNotification(ctx, seeded.notification, seeded.permission); err != nil {
			return err
		}
	}
	for _, user := range devUsers {
		if err := s.markSeededNotificationsRead(ctx, userIDs[user.Username]); err != nil {
			return err
		}
	}
	s.logger.Info("created notifications", "count", len(seededNotifications))
	return nil
}

func (s *seeder) markSeededNotificationsRead(ctx context.Context, userID int) error {
	received, err := s.receivedNotifications(ctx, userID)
	if err != nil {
		return err
	}
	for _, seeded := range seededNotifications {
		item, ok := received[seeded.notification.Message]
		if !ok || !seeded.read || item.IsRead {
			continue
		}
		if err := s.notifications.MarkAsRead(ctx, userID, item.NotificationID); err != nil {
			return err
		}
	}
	return nil
}

func (s *seeder) receivedNotifications(ctx context.Context, userID int) (map[string]notifications.UserNotification, error) {
	received, err := s.notifications.GetNotificationsForUser(ctx, userID, notificationPageSize, 0, true)
	if err != nil {
		return nil, err
	}
	byMessage := make(map[string]notifications.UserNotification, len(received))
	for _, item := range received {
		if item.Notification != nil {
			byMessage[item.Notification.Message] = item
		}
	}
	return byMessage, nil
}
