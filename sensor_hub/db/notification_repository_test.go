package database

import (
	"context"
	"database/sql"
	"log/slog"
	"testing"
	"time"

	"example/sensorHub/notifications"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestNotificationRepository_CreateNotification(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewNotificationRepository(handles(db), slog.Default())

	mock.ExpectExec("INSERT INTO notifications").
		WithArgs("user_management", "info", "Test Title", "Test Message", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	notif := notifications.Notification{
		Category: notifications.CategoryUserManagement,
		Severity: notifications.SeverityInfo,
		Title:    "Test Title",
		Message:  "Test Message",
	}

	id, err := repo.CreateNotification(context.Background(), notif)
	require.NoError(t, err)
	require.Equal(t, 1, id)
}

func TestNotificationRepository_CreateNotification_Invalid(t *testing.T) {
	db, _ := newMockDB(t)
	repo := NewNotificationRepository(handles(db), slog.Default())

	notif := notifications.Notification{
		Category: "invalid",
		Severity: notifications.SeverityInfo,
		Title:    "Test",
		Message:  "Test",
	}

	_, err := repo.CreateNotification(context.Background(), notif)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid notification")
}

func TestNotificationRepository_GetUnreadCountForUser(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewNotificationRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT COUNT").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	count, err := repo.GetUnreadCountForUser(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 5, count)
}

func TestNotificationRepository_AssignNotificationToUser(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewNotificationRepository(handles(db), slog.Default())

	mock.ExpectExec("INSERT OR IGNORE INTO user_notifications").
		WithArgs(1, 5).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.AssignNotificationToUser(context.Background(), 1, 5)
	require.NoError(t, err)
}

func TestNotificationRepository_MarkAsRead(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewNotificationRepository(handles(db), slog.Default())

	mock.ExpectExec("UPDATE user_notifications SET is_read = 1").
		WithArgs(1, 5).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.MarkAsRead(context.Background(), 1, 5)
	require.NoError(t, err)
}

func TestNotificationRepository_DismissNotification(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewNotificationRepository(handles(db), slog.Default())

	mock.ExpectExec("UPDATE user_notifications SET is_dismissed = 1").
		WithArgs(1, 5).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.DismissNotification(context.Background(), 1, 5)
	require.NoError(t, err)
}

func TestNotificationRepository_BulkMarkAsRead(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewNotificationRepository(handles(db), slog.Default())

	mock.ExpectExec("UPDATE user_notifications SET is_read = 1").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 10))

	err := repo.BulkMarkAsRead(context.Background(), 1)
	require.NoError(t, err)
}

func TestNotificationRepository_BulkDismiss(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewNotificationRepository(handles(db), slog.Default())

	mock.ExpectExec("UPDATE user_notifications SET is_dismissed = 1").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 10))

	err := repo.BulkDismiss(context.Background(), 1)
	require.NoError(t, err)
}

func TestNotificationRepository_DeleteOldNotifications(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewNotificationRepository(handles(db), slog.Default())

	cutoff := time.Now().AddDate(0, 0, -90)

	mock.ExpectExec("DELETE FROM notifications WHERE created_at").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 5))

	deleted, err := repo.DeleteOldNotifications(context.Background(), cutoff)
	require.NoError(t, err)
	require.Equal(t, int64(5), deleted)
}

func TestNotificationRepository_GetChannelPreference_UserOverride(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewNotificationRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT .* FROM notification_channel_preferences").
		WithArgs(1, "user_management").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "category", "email_enabled", "inapp_enabled"}).
			AddRow(1, "user_management", false, true))

	pref, err := repo.GetChannelPreference(context.Background(), 1, notifications.CategoryUserManagement)
	require.NoError(t, err)
	require.False(t, pref.EmailEnabled)
	require.True(t, pref.InAppEnabled)
}

func TestNotificationRepository_GetChannelPreference_FallbackToDefault(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewNotificationRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT .* FROM notification_channel_preferences").
		WithArgs(1, "threshold_alert").
		WillReturnError(sql.ErrNoRows)

	mock.ExpectQuery("SELECT .* FROM notification_channel_defaults").
		WithArgs("threshold_alert").
		WillReturnRows(sqlmock.NewRows([]string{"email_enabled", "inapp_enabled"}).
			AddRow(true, true))

	pref, err := repo.GetChannelPreference(context.Background(), 1, notifications.CategoryThresholdAlert)
	require.NoError(t, err)
	require.True(t, pref.EmailEnabled)
	require.True(t, pref.InAppEnabled)
}

func TestNotificationRepository_SetChannelPreference(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewNotificationRepository(handles(db), slog.Default())

	mock.ExpectExec("INSERT INTO notification_channel_preferences").
		WithArgs(1, "user_management", false, true).
		WillReturnResult(sqlmock.NewResult(1, 1))

	pref := notifications.ChannelPreference{
		UserID:       1,
		Category:     notifications.CategoryUserManagement,
		EmailEnabled: false,
		InAppEnabled: true,
	}
	err := repo.SetChannelPreference(context.Background(), pref)
	require.NoError(t, err)
}
