package database

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// RecordFailedAttempt tests
// ============================================================================

func TestFailedLoginRepository_RecordFailedAttempt_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewFailedLoginRepository(handles(db), slog.Default())

	userId := 1
	mock.ExpectExec("INSERT INTO failed_login_attempts").
		WithArgs("testuser", &userId, "192.168.1.1", sqlmock.AnyArg(), "invalid password").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.RecordFailedAttempt(context.Background(), "testuser", &userId, "192.168.1.1", "invalid password")

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFailedLoginRepository_RecordFailedAttempt_NullUserId(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewFailedLoginRepository(handles(db), slog.Default())

	mock.ExpectExec("INSERT INTO failed_login_attempts").
		WithArgs("unknownuser", nil, "192.168.1.1", sqlmock.AnyArg(), "user not found").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.RecordFailedAttempt(context.Background(), "unknownuser", nil, "192.168.1.1", "user not found")

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFailedLoginRepository_RecordFailedAttempt_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewFailedLoginRepository(handles(db), slog.Default())

	mock.ExpectExec("INSERT INTO failed_login_attempts").
		WithArgs("testuser", nil, "192.168.1.1", sqlmock.AnyArg(), "test").
		WillReturnError(errors.New("database error"))

	err := repo.RecordFailedAttempt(context.Background(), "testuser", nil, "192.168.1.1", "test")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error recording failed login attempt")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// CountRecentFailedAttemptsByUsername tests
// ============================================================================

func TestFailedLoginRepository_CountRecentFailedAttemptsByUsername_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewFailedLoginRepository(handles(db), slog.Default())

	window := 15 * time.Minute
	mock.ExpectQuery("SELECT COUNT\\(1\\) FROM failed_login_attempts WHERE LOWER\\(username\\) = LOWER\\(\\?\\) AND attempt_time > \\?").
		WithArgs("testuser", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	count, err := repo.CountRecentFailedAttemptsByUsername(context.Background(), "testuser", window)

	assert.NoError(t, err)
	assert.Equal(t, 3, count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFailedLoginRepository_CountRecentFailedAttemptsByUsername_Zero(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewFailedLoginRepository(handles(db), slog.Default())

	window := 15 * time.Minute
	mock.ExpectQuery("SELECT COUNT\\(1\\) FROM failed_login_attempts WHERE LOWER\\(username\\) = LOWER\\(\\?\\) AND attempt_time > \\?").
		WithArgs("cleanuser", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	count, err := repo.CountRecentFailedAttemptsByUsername(context.Background(), "cleanuser", window)

	assert.NoError(t, err)
	assert.Equal(t, 0, count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFailedLoginRepository_CountRecentFailedAttemptsByUsername_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewFailedLoginRepository(handles(db), slog.Default())

	window := 15 * time.Minute
	mock.ExpectQuery("SELECT COUNT\\(1\\) FROM failed_login_attempts WHERE LOWER\\(username\\) = LOWER\\(\\?\\) AND attempt_time > \\?").
		WithArgs("testuser", sqlmock.AnyArg()).
		WillReturnError(errors.New("database error"))

	count, err := repo.CountRecentFailedAttemptsByUsername(context.Background(), "testuser", window)

	assert.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "error counting failed attempts by username")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// CountRecentFailedAttemptsByIP tests
// ============================================================================

func TestFailedLoginRepository_CountRecentFailedAttemptsByIP_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewFailedLoginRepository(handles(db), slog.Default())

	window := 15 * time.Minute
	mock.ExpectQuery("SELECT COUNT\\(1\\) FROM failed_login_attempts WHERE ip_address = \\? AND attempt_time > \\?").
		WithArgs("192.168.1.1", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	count, err := repo.CountRecentFailedAttemptsByIP(context.Background(), "192.168.1.1", window)

	assert.NoError(t, err)
	assert.Equal(t, 5, count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFailedLoginRepository_CountRecentFailedAttemptsByIP_Zero(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewFailedLoginRepository(handles(db), slog.Default())

	window := 15 * time.Minute
	mock.ExpectQuery("SELECT COUNT\\(1\\) FROM failed_login_attempts WHERE ip_address = \\? AND attempt_time > \\?").
		WithArgs("10.0.0.1", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	count, err := repo.CountRecentFailedAttemptsByIP(context.Background(), "10.0.0.1", window)

	assert.NoError(t, err)
	assert.Equal(t, 0, count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFailedLoginRepository_CountRecentFailedAttemptsByIP_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewFailedLoginRepository(handles(db), slog.Default())

	window := 15 * time.Minute
	mock.ExpectQuery("SELECT COUNT\\(1\\) FROM failed_login_attempts WHERE ip_address = \\? AND attempt_time > \\?").
		WithArgs("192.168.1.1", sqlmock.AnyArg()).
		WillReturnError(errors.New("database error"))

	count, err := repo.CountRecentFailedAttemptsByIP(context.Background(), "192.168.1.1", window)

	assert.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "error counting failed attempts by ip")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// DeleteRecentFailedAttemptsByIP tests
// ============================================================================

func TestFailedLoginRepository_DeleteRecentFailedAttemptsByIP_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewFailedLoginRepository(handles(db), slog.Default())

	window := 15 * time.Minute
	mock.ExpectExec("DELETE FROM failed_login_attempts WHERE ip_address = \\? AND attempt_time > \\?").
		WithArgs("192.168.1.1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 5))

	err := repo.DeleteRecentFailedAttemptsByIP(context.Background(), "192.168.1.1", window)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFailedLoginRepository_DeleteRecentFailedAttemptsByIP_NothingToDelete(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewFailedLoginRepository(handles(db), slog.Default())

	window := 15 * time.Minute
	mock.ExpectExec("DELETE FROM failed_login_attempts WHERE ip_address = \\? AND attempt_time > \\?").
		WithArgs("192.168.1.1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.DeleteRecentFailedAttemptsByIP(context.Background(), "192.168.1.1", window)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFailedLoginRepository_DeleteRecentFailedAttemptsByIP_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewFailedLoginRepository(handles(db), slog.Default())

	window := 15 * time.Minute
	mock.ExpectExec("DELETE FROM failed_login_attempts WHERE ip_address = \\? AND attempt_time > \\?").
		WithArgs("192.168.1.1", sqlmock.AnyArg()).
		WillReturnError(errors.New("database error"))

	err := repo.DeleteRecentFailedAttemptsByIP(context.Background(), "192.168.1.1", window)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error deleting failed attempts by ip")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// DeleteAttemptsOlderThan tests
// ============================================================================

func TestFailedLoginRepository_DeleteAttemptsOlderThan_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewFailedLoginRepository(handles(db), slog.Default())

	threshold := time.Now().Add(-24 * time.Hour)
	mock.ExpectExec("DELETE FROM failed_login_attempts WHERE attempt_time < \\?").
		WithArgs(threshold).
		WillReturnResult(sqlmock.NewResult(0, 100))

	err := repo.DeleteAttemptsOlderThan(context.Background(), threshold)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFailedLoginRepository_DeleteAttemptsOlderThan_NothingToDelete(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewFailedLoginRepository(handles(db), slog.Default())

	threshold := time.Now().Add(-24 * time.Hour)
	mock.ExpectExec("DELETE FROM failed_login_attempts WHERE attempt_time < \\?").
		WithArgs(threshold).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.DeleteAttemptsOlderThan(context.Background(), threshold)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFailedLoginRepository_DeleteAttemptsOlderThan_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewFailedLoginRepository(handles(db), slog.Default())

	threshold := time.Now().Add(-24 * time.Hour)
	mock.ExpectExec("DELETE FROM failed_login_attempts WHERE attempt_time < \\?").
		WithArgs(threshold).
		WillReturnError(errors.New("database error"))

	err := repo.DeleteAttemptsOlderThan(context.Background(), threshold)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error deleting old failed login attempts")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// Edge case tests
// ============================================================================

func TestFailedLoginRepository_RecordFailedAttempt_EmptyUsername(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewFailedLoginRepository(handles(db), slog.Default())

	mock.ExpectExec("INSERT INTO failed_login_attempts").
		WithArgs("", nil, "192.168.1.1", sqlmock.AnyArg(), "empty username").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.RecordFailedAttempt(context.Background(), "", nil, "192.168.1.1", "empty username")

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFailedLoginRepository_CountRecentFailedAttemptsByIP_IPv6(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewFailedLoginRepository(handles(db), slog.Default())

	window := 15 * time.Minute
	mock.ExpectQuery("SELECT COUNT\\(1\\) FROM failed_login_attempts WHERE ip_address = \\? AND attempt_time > \\?").
		WithArgs("2001:0db8:85a3:0000:0000:8a2e:0370:7334", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	count, err := repo.CountRecentFailedAttemptsByIP(context.Background(), "2001:0db8:85a3:0000:0000:8a2e:0370:7334", window)

	assert.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFailedLoginRepository_CountRecentFailedAttemptsByUsername_VeryLongWindow(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewFailedLoginRepository(handles(db), slog.Default())

	window := 30 * 24 * time.Hour // 30 days
	mock.ExpectQuery("SELECT COUNT\\(1\\) FROM failed_login_attempts WHERE LOWER\\(username\\) = LOWER\\(\\?\\) AND attempt_time > \\?").
		WithArgs("testuser", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(50))

	count, err := repo.CountRecentFailedAttemptsByUsername(context.Background(), "testuser", window)

	assert.NoError(t, err)
	assert.Equal(t, 50, count)
	assert.NoError(t, mock.ExpectationsWereMet())
}
