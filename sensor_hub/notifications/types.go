package notifications

import (
	"encoding/json"
	"fmt"
	"time"
)

type NotificationCategory string

const (
	CategoryThresholdAlert    NotificationCategory = "threshold_alert"
	CategoryUserManagement    NotificationCategory = "user_management"
	CategoryConfigChange      NotificationCategory = "config_change"
	CategoryAutomationFailure NotificationCategory = "automation_failure"
)

type NotificationSeverity string

const (
	SeverityInfo    NotificationSeverity = "info"
	SeverityWarning NotificationSeverity = "warning"
	SeverityError   NotificationSeverity = "error"
)

type Notification struct {
	ID        int                    `json:"id"`
	Category  NotificationCategory   `json:"category"`
	Severity  NotificationSeverity   `json:"severity"`
	Title     string                 `json:"title"`
	Message   string                 `json:"message"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

type UserNotification struct {
	ID             int           `json:"id"`
	UserID         int           `json:"user_id"`
	NotificationID int           `json:"notification_id"`
	IsRead         bool          `json:"is_read"`
	IsDismissed    bool          `json:"is_dismissed"`
	ReadAt         *time.Time    `json:"read_at,omitempty"`
	DismissedAt    *time.Time    `json:"dismissed_at,omitempty"`
	Notification   *Notification `json:"notification,omitempty"`
}

type ChannelPreference struct {
	UserID       int                  `json:"user_id,omitempty"`
	Category     NotificationCategory `json:"category"`
	EmailEnabled bool                 `json:"email_enabled"`
	InAppEnabled bool                 `json:"inapp_enabled"`
}

var validCategories = map[NotificationCategory]bool{
	CategoryThresholdAlert:    true,
	CategoryUserManagement:    true,
	CategoryConfigChange:      true,
	CategoryAutomationFailure: true,
}

var validSeverities = map[NotificationSeverity]bool{
	SeverityInfo:    true,
	SeverityWarning: true,
	SeverityError:   true,
}

func (n *Notification) Validate() error {
	if !validCategories[n.Category] {
		return fmt.Errorf("invalid category: %s", n.Category)
	}
	if !validSeverities[n.Severity] {
		return fmt.Errorf("invalid severity: %s", n.Severity)
	}
	if n.Title == "" {
		return fmt.Errorf("title cannot be empty")
	}
	if n.Message == "" {
		return fmt.Errorf("message cannot be empty")
	}
	return nil
}

func (n *Notification) MetadataJSON() ([]byte, error) {
	if n.Metadata == nil {
		return []byte("null"), nil
	}
	return json.Marshal(n.Metadata)
}

func ParseMetadataJSON(data []byte) (map[string]interface{}, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}
