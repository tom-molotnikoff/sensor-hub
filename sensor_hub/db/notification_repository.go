package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"example/sensorHub/notifications"
)

type UserEmailInfo struct {
	UserID int
	Email  string
}

type NotificationRepository interface {
	CreateNotification(ctx context.Context, notif notifications.Notification) (int, error)
	AssignNotificationToUser(ctx context.Context, userID, notificationID int) error
	AssignNotificationToUsersWithPermission(ctx context.Context, notificationID int, permission string) error
	GetUserIDsWithPermission(ctx context.Context, permission string) ([]int, error)
	GetUsersWithPermissionAndEmail(ctx context.Context, permission string) ([]UserEmailInfo, error)
	GetNotificationsForUser(ctx context.Context, userID int, limit, offset int, includeDismissed bool) ([]notifications.UserNotification, error)
	GetUnreadCountForUser(ctx context.Context, userID int) (int, error)
	MarkAsRead(ctx context.Context, userID, notificationID int) error
	DismissNotification(ctx context.Context, userID, notificationID int) error
	BulkMarkAsRead(ctx context.Context, userID int) error
	BulkDismiss(ctx context.Context, userID int) error
	DeleteOldNotifications(ctx context.Context, olderThan time.Time) (int64, error)
	GetChannelPreference(ctx context.Context, userID int, category notifications.NotificationCategory) (*notifications.ChannelPreference, error)
	GetAllChannelPreferences(ctx context.Context, userID int) ([]notifications.ChannelPreference, error)
	SetChannelPreference(ctx context.Context, pref notifications.ChannelPreference) error
	GetDefaultChannelPreference(ctx context.Context, category notifications.NotificationCategory) (*notifications.ChannelPreference, error)
}

type SqlNotificationRepository struct {
	db     *Handles
	logger *slog.Logger
}

func NewNotificationRepository(db *Handles, logger *slog.Logger) *SqlNotificationRepository {
	return &SqlNotificationRepository{db: db, logger: logger.With("component", "notification_repository")}
}

func (r *SqlNotificationRepository) CreateNotification(ctx context.Context, notif notifications.Notification) (int, error) {
	if err := notif.Validate(); err != nil {
		return 0, fmt.Errorf("invalid notification: %w", err)
	}

	metadataJSON, err := notif.MetadataJSON()
	if err != nil {
		return 0, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	result, err := r.db.Writer.ExecContext(ctx,
		"INSERT INTO notifications (category, severity, title, message, metadata) VALUES (?, ?, ?, ?, ?)",
		notif.Category, notif.Severity, notif.Title, notif.Message, metadataJSON,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert notification: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}
	return int(id), nil
}

func (r *SqlNotificationRepository) AssignNotificationToUser(ctx context.Context, userID, notificationID int) error {
	_, err := r.db.Writer.ExecContext(ctx,
		"INSERT OR IGNORE INTO user_notifications (user_id, notification_id) VALUES (?, ?)",
		userID, notificationID,
	)
	return err
}

func (r *SqlNotificationRepository) AssignNotificationToUsersWithPermission(ctx context.Context, notificationID int, permission string) error {
	query := `
		INSERT OR IGNORE INTO user_notifications (user_id, notification_id)
		SELECT DISTINCT ur.user_id, ?
		FROM user_roles ur
		JOIN role_permissions rp ON ur.role_id = rp.role_id
		JOIN permissions p ON rp.permission_id = p.id
		WHERE LOWER(p.name) = LOWER(?)`
	_, err := r.db.Writer.ExecContext(ctx, query, notificationID, permission)
	return err
}

func (r *SqlNotificationRepository) GetUserIDsWithPermission(ctx context.Context, permission string) ([]int, error) {
	query := `
		SELECT DISTINCT ur.user_id
		FROM user_roles ur
		JOIN role_permissions rp ON ur.role_id = rp.role_id
		JOIN permissions p ON rp.permission_id = p.id
		WHERE LOWER(p.name) = LOWER(?)`
	rows, err := r.db.Reader.QueryContext(ctx, query, permission)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, id)
	}
	return userIDs, rows.Err()
}

func (r *SqlNotificationRepository) GetUsersWithPermissionAndEmail(ctx context.Context, permission string) ([]UserEmailInfo, error) {
	query := `
		SELECT DISTINCT ur.user_id, u.email
		FROM user_roles ur
		JOIN role_permissions rp ON ur.role_id = rp.role_id
		JOIN permissions p ON rp.permission_id = p.id
		JOIN users u ON ur.user_id = u.id
		WHERE LOWER(p.name) = LOWER(?) AND u.email IS NOT NULL AND u.email != '' AND u.disabled = FALSE`
	rows, err := r.db.Reader.QueryContext(ctx, query, permission)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []UserEmailInfo
	for rows.Next() {
		var info UserEmailInfo
		if err := rows.Scan(&info.UserID, &info.Email); err != nil {
			return nil, err
		}
		users = append(users, info)
	}
	return users, rows.Err()
}

func (r *SqlNotificationRepository) GetNotificationsForUser(ctx context.Context, userID int, limit, offset int, includeDismissed bool) ([]notifications.UserNotification, error) {
	dismissedFilter := "AND un.is_dismissed = FALSE"
	if includeDismissed {
		dismissedFilter = ""
	}

	query := fmt.Sprintf(`
		SELECT un.id, un.user_id, un.notification_id, un.is_read, un.is_dismissed, un.read_at, un.dismissed_at,
		       n.id, n.category, n.severity, n.title, n.message, n.metadata, n.created_at
		FROM user_notifications un
		JOIN notifications n ON un.notification_id = n.id
		WHERE un.user_id = ? %s
		ORDER BY n.created_at DESC
		LIMIT ? OFFSET ?`, dismissedFilter)

	rows, err := r.db.Reader.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query notifications: %w", err)
	}
	defer rows.Close()

	results := make([]notifications.UserNotification, 0)
	for rows.Next() {
		var un notifications.UserNotification
		var n notifications.Notification
		var metadataJSON []byte
		var readAt, dismissedAt NullSQLiteTime
		var createdAt SQLiteTime

		err := rows.Scan(
			&un.ID, &un.UserID, &un.NotificationID, &un.IsRead, &un.IsDismissed, &readAt, &dismissedAt,
			&n.ID, &n.Category, &n.Severity, &n.Title, &n.Message, &metadataJSON, &createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		n.CreatedAt = createdAt.Time
		if readAt.Valid {
			un.ReadAt = &readAt.Time
		}
		if dismissedAt.Valid {
			un.DismissedAt = &dismissedAt.Time
		}
		n.Metadata, _ = notifications.ParseMetadataJSON(metadataJSON)
		un.Notification = &n
		results = append(results, un)
	}
	return results, nil
}

func (r *SqlNotificationRepository) GetUnreadCountForUser(ctx context.Context, userID int) (int, error) {
	var count int
	err := r.db.Reader.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM user_notifications WHERE user_id = ? AND is_read = FALSE AND is_dismissed = FALSE",
		userID,
	).Scan(&count)
	return count, err
}

func (r *SqlNotificationRepository) MarkAsRead(ctx context.Context, userID, notificationID int) error {
	_, err := r.db.Writer.ExecContext(ctx,
		"UPDATE user_notifications SET is_read = 1, read_at = datetime('now') WHERE user_id = ? AND notification_id = ?",
		userID, notificationID,
	)
	return err
}

func (r *SqlNotificationRepository) DismissNotification(ctx context.Context, userID, notificationID int) error {
	_, err := r.db.Writer.ExecContext(ctx,
		"UPDATE user_notifications SET is_dismissed = 1, dismissed_at = datetime('now') WHERE user_id = ? AND notification_id = ?",
		userID, notificationID,
	)
	return err
}

func (r *SqlNotificationRepository) BulkMarkAsRead(ctx context.Context, userID int) error {
	_, err := r.db.Writer.ExecContext(ctx,
		"UPDATE user_notifications SET is_read = 1, read_at = datetime('now') WHERE user_id = ? AND is_read = 0",
		userID,
	)
	return err
}

func (r *SqlNotificationRepository) BulkDismiss(ctx context.Context, userID int) error {
	_, err := r.db.Writer.ExecContext(ctx,
		"UPDATE user_notifications SET is_dismissed = 1, dismissed_at = datetime('now') WHERE user_id = ? AND is_dismissed = 0",
		userID,
	)
	return err
}

func (r *SqlNotificationRepository) DeleteOldNotifications(ctx context.Context, olderThan time.Time) (int64, error) {
	result, err := r.db.Writer.ExecContext(ctx, "DELETE FROM notifications WHERE created_at < ?", olderThan)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *SqlNotificationRepository) GetChannelPreference(ctx context.Context, userID int, category notifications.NotificationCategory) (*notifications.ChannelPreference, error) {
	var pref notifications.ChannelPreference
	err := r.db.Reader.QueryRowContext(ctx,
		"SELECT user_id, category, email_enabled, inapp_enabled FROM notification_channel_preferences WHERE user_id = ? AND LOWER(category) = LOWER(?)",
		userID, category,
	).Scan(&pref.UserID, &pref.Category, &pref.EmailEnabled, &pref.InAppEnabled)

	if err == sql.ErrNoRows {
		return r.GetDefaultChannelPreference(ctx, category)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get channel preference: %w", err)
	}
	return &pref, nil
}

func (r *SqlNotificationRepository) GetDefaultChannelPreference(ctx context.Context, category notifications.NotificationCategory) (*notifications.ChannelPreference, error) {
	var pref notifications.ChannelPreference
	pref.Category = category
	err := r.db.Reader.QueryRowContext(ctx,
		"SELECT email_enabled, inapp_enabled FROM notification_channel_defaults WHERE LOWER(category) = LOWER(?)",
		category,
	).Scan(&pref.EmailEnabled, &pref.InAppEnabled)
	if err != nil {
		return nil, fmt.Errorf("failed to get default preference: %w", err)
	}
	return &pref, nil
}

func (r *SqlNotificationRepository) GetAllChannelPreferences(ctx context.Context, userID int) ([]notifications.ChannelPreference, error) {
	categories := []notifications.NotificationCategory{
		notifications.CategoryThresholdAlert,
		notifications.CategoryUserManagement,
		notifications.CategoryConfigChange,
	}
	var prefs []notifications.ChannelPreference
	for _, cat := range categories {
		pref, err := r.GetChannelPreference(ctx, userID, cat)
		if err != nil {
			return nil, err
		}
		pref.UserID = userID
		prefs = append(prefs, *pref)
	}
	return prefs, nil
}

func (r *SqlNotificationRepository) SetChannelPreference(ctx context.Context, pref notifications.ChannelPreference) error {
	_, err := r.db.Writer.ExecContext(ctx,
		`INSERT INTO notification_channel_preferences (user_id, category, email_enabled, inapp_enabled)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(user_id, category) DO UPDATE SET email_enabled = excluded.email_enabled, inapp_enabled = excluded.inapp_enabled`,
		pref.UserID, pref.Category, pref.EmailEnabled, pref.InAppEnabled,
	)
	return err
}
