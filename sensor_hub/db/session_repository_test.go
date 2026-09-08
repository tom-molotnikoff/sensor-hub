package database

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CreateSession tests
// ============================================================================

func TestSessionRepository_CreateSession_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	expiresAt := time.Now().Add(24 * time.Hour)
	mock.ExpectExec("INSERT INTO sessions").
		WithArgs(1, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), expiresAt, sqlmock.AnyArg(), "192.168.1.1", "Mozilla/5.0").
		WillReturnResult(sqlmock.NewResult(1, 1))

	csrf, err := repo.CreateSession(context.Background(), 1, "raw-token-value", expiresAt, "192.168.1.1", "Mozilla/5.0")

	assert.NoError(t, err)
	assert.NotEmpty(t, csrf) // CSRF token should be generated
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_CreateSession_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	expiresAt := time.Now().Add(24 * time.Hour)
	mock.ExpectExec("INSERT INTO sessions").
		WithArgs(1, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), expiresAt, sqlmock.AnyArg(), "192.168.1.1", "Mozilla/5.0").
		WillReturnError(errors.New("database error"))

	csrf, err := repo.CreateSession(context.Background(), 1, "raw-token-value", expiresAt, "192.168.1.1", "Mozilla/5.0")

	assert.Error(t, err)
	assert.Empty(t, csrf)
	assert.Contains(t, err.Error(), "error creating session")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GetUserIdByToken tests
// ============================================================================

func TestSessionRepository_GetUserIdByToken_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	expiresAt := time.Now().Add(1 * time.Hour) // Not expired
	mock.ExpectQuery("SELECT user_id, expires_at FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "expires_at"}).AddRow(42, expiresAt))

	// Update last accessed
	mock.ExpectExec("UPDATE sessions SET last_accessed_at = \\? WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	userId, err := repo.GetUserIdByToken(context.Background(), "valid-token")

	assert.NoError(t, err)
	assert.Equal(t, 42, userId)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_GetUserIdByToken_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT user_id, expires_at FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnError(sql.ErrNoRows)

	userId, err := repo.GetUserIdByToken(context.Background(), "nonexistent-token")

	assert.NoError(t, err)
	assert.Equal(t, 0, userId)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_GetUserIdByToken_Expired(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	expiresAt := time.Now().Add(-1 * time.Hour) // Expired
	mock.ExpectQuery("SELECT user_id, expires_at FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "expires_at"}).AddRow(42, expiresAt))

	// Should delete expired session
	mock.ExpectExec("DELETE FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	userId, err := repo.GetUserIdByToken(context.Background(), "expired-token")

	assert.NoError(t, err)
	assert.Equal(t, 0, userId)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_GetUserIdByToken_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT user_id, expires_at FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnError(errors.New("database error"))

	userId, err := repo.GetUserIdByToken(context.Background(), "some-token")

	assert.Error(t, err)
	assert.Equal(t, 0, userId)
	assert.Contains(t, err.Error(), "error querying session")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GetSessionIdByToken tests
// ============================================================================

func TestSessionRepository_GetSessionIdByToken_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	expiresAt := time.Now().Add(1 * time.Hour)
	mock.ExpectQuery("SELECT id, expires_at FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "expires_at"}).AddRow(int64(123), expiresAt))

	sessionId, err := repo.GetSessionIdByToken(context.Background(), "valid-token")

	assert.NoError(t, err)
	assert.Equal(t, int64(123), sessionId)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_GetSessionIdByToken_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT id, expires_at FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnError(sql.ErrNoRows)

	sessionId, err := repo.GetSessionIdByToken(context.Background(), "nonexistent-token")

	assert.NoError(t, err)
	assert.Equal(t, int64(0), sessionId)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_GetSessionIdByToken_Expired(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	expiresAt := time.Now().Add(-1 * time.Hour)
	mock.ExpectQuery("SELECT id, expires_at FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "expires_at"}).AddRow(int64(123), expiresAt))

	mock.ExpectExec("DELETE FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	sessionId, err := repo.GetSessionIdByToken(context.Background(), "expired-token")

	assert.NoError(t, err)
	assert.Equal(t, int64(0), sessionId)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_GetSessionIdByToken_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT id, expires_at FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnError(errors.New("database error"))

	sessionId, err := repo.GetSessionIdByToken(context.Background(), "some-token")

	assert.Error(t, err)
	assert.Equal(t, int64(0), sessionId)
	assert.Contains(t, err.Error(), "error querying session id")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// DeleteSessionByToken tests
// ============================================================================

func TestSessionRepository_DeleteSessionByToken_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	mock.ExpectExec("DELETE FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.DeleteSessionByToken(context.Background(), "token-to-delete")

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_DeleteSessionByToken_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	mock.ExpectExec("DELETE FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.DeleteSessionByToken(context.Background(), "nonexistent-token")

	assert.NoError(t, err) // Not an error if nothing to delete
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_DeleteSessionByToken_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	mock.ExpectExec("DELETE FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnError(errors.New("database error"))

	err := repo.DeleteSessionByToken(context.Background(), "some-token")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error deleting session")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// DeleteSessionsForUser tests
// ============================================================================

func TestSessionRepository_DeleteSessionsForUser_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	mock.ExpectExec("DELETE FROM sessions WHERE user_id = \\?").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 5))

	err := repo.DeleteSessionsForUser(context.Background(), 1)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_DeleteSessionsForUser_NoSessions(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	mock.ExpectExec("DELETE FROM sessions WHERE user_id = \\?").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.DeleteSessionsForUser(context.Background(), 1)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_DeleteSessionsForUser_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	mock.ExpectExec("DELETE FROM sessions WHERE user_id = \\?").
		WithArgs(1).
		WillReturnError(errors.New("database error"))

	err := repo.DeleteSessionsForUser(context.Background(), 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error deleting sessions")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// ListSessionsForUser tests
// ============================================================================

func TestSessionRepository_ListSessionsForUser_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	mock.ExpectQuery("SELECT id, user_id, created_at, expires_at, last_accessed_at, ip_address, user_agent FROM sessions WHERE user_id = \\?").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows(sessionColumns).
			AddRow(int64(1), 1, now, expiresAt, now, "192.168.1.1", "Mozilla/5.0").
			AddRow(int64(2), 1, now, expiresAt, now, "192.168.1.2", "Chrome/90"))

	sessions, err := repo.ListSessionsForUser(context.Background(), 1)

	assert.NoError(t, err)
	require.Len(t, sessions, 2)
	assert.Equal(t, int64(1), sessions[0].Id)
	assert.Equal(t, "192.168.1.1", sessions[0].IpAddress)
	assert.Equal(t, int64(2), sessions[1].Id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_ListSessionsForUser_Empty(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT id, user_id, created_at, expires_at, last_accessed_at, ip_address, user_agent FROM sessions WHERE user_id = \\?").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows(sessionColumns))

	sessions, err := repo.ListSessionsForUser(context.Background(), 1)

	assert.NoError(t, err)
	assert.Empty(t, sessions)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_ListSessionsForUser_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT id, user_id, created_at, expires_at, last_accessed_at, ip_address, user_agent FROM sessions WHERE user_id = \\?").
		WithArgs(1).
		WillReturnError(errors.New("database error"))

	sessions, err := repo.ListSessionsForUser(context.Background(), 1)

	assert.Error(t, err)
	assert.Nil(t, sessions)
	assert.Contains(t, err.Error(), "error querying sessions")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// RevokeSessionById tests
// ============================================================================

func TestSessionRepository_RevokeSessionById_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	mock.ExpectExec("DELETE FROM sessions WHERE id = \\?").
		WithArgs(int64(123)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.RevokeSessionById(context.Background(), 123)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_RevokeSessionById_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	mock.ExpectExec("DELETE FROM sessions WHERE id = \\?").
		WithArgs(int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.RevokeSessionById(context.Background(), 999)

	assert.NoError(t, err) // Not an error if nothing to revoke
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_RevokeSessionById_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	mock.ExpectExec("DELETE FROM sessions WHERE id = \\?").
		WithArgs(int64(123)).
		WillReturnError(errors.New("database error"))

	err := repo.RevokeSessionById(context.Background(), 123)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error revoking session")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GetCSRFForToken tests
// ============================================================================

func TestSessionRepository_GetCSRFForToken_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	expiresAt := time.Now().Add(1 * time.Hour)
	mock.ExpectQuery("SELECT csrf_token, expires_at FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"csrf_token", "expires_at"}).AddRow("csrf-token-value", expiresAt))

	csrf, err := repo.GetCSRFForToken(context.Background(), "valid-token")

	assert.NoError(t, err)
	assert.Equal(t, "csrf-token-value", csrf)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_GetCSRFForToken_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT csrf_token, expires_at FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnError(sql.ErrNoRows)

	csrf, err := repo.GetCSRFForToken(context.Background(), "nonexistent-token")

	assert.NoError(t, err)
	assert.Empty(t, csrf)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_GetCSRFForToken_Expired(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	expiresAt := time.Now().Add(-1 * time.Hour)
	mock.ExpectQuery("SELECT csrf_token, expires_at FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"csrf_token", "expires_at"}).AddRow("csrf-token-value", expiresAt))

	mock.ExpectExec("DELETE FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	csrf, err := repo.GetCSRFForToken(context.Background(), "expired-token")

	assert.NoError(t, err)
	assert.Empty(t, csrf)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_GetCSRFForToken_NullCSRF(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	expiresAt := time.Now().Add(1 * time.Hour)
	mock.ExpectQuery("SELECT csrf_token, expires_at FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"csrf_token", "expires_at"}).AddRow(nil, expiresAt))

	csrf, err := repo.GetCSRFForToken(context.Background(), "token-with-null-csrf")

	assert.NoError(t, err)
	assert.Empty(t, csrf)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_GetCSRFForToken_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT csrf_token, expires_at FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnError(errors.New("database error"))

	csrf, err := repo.GetCSRFForToken(context.Background(), "some-token")

	assert.Error(t, err)
	assert.Empty(t, csrf)
	assert.Contains(t, err.Error(), "error querying csrf token")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// InsertSessionAudit tests
// ============================================================================

func TestSessionRepository_InsertSessionAudit_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	revokedBy := 2
	reason := "password change"
	mock.ExpectExec("INSERT INTO session_audit").
		WithArgs(int64(123), &revokedBy, "revoke", &reason, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.InsertSessionAudit(context.Background(), 123, &revokedBy, "revoke", &reason)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_InsertSessionAudit_NullOptionalFields(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	mock.ExpectExec("INSERT INTO session_audit").
		WithArgs(int64(123), nil, "logout", nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.InsertSessionAudit(context.Background(), 123, nil, "logout", nil)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_InsertSessionAudit_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	mock.ExpectExec("INSERT INTO session_audit").
		WithArgs(int64(123), nil, "revoke", nil, sqlmock.AnyArg()).
		WillReturnError(errors.New("database error"))

	err := repo.InsertSessionAudit(context.Background(), 123, nil, "revoke", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error inserting session audit")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// Edge case tests
// ============================================================================

func TestSessionRepository_TokenHashing_Consistent(t *testing.T) {
	// Verify that the same token always produces the same hash
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	expiresAt := time.Now().Add(1 * time.Hour)

	// First call
	mock.ExpectQuery("SELECT user_id, expires_at FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "expires_at"}).AddRow(1, expiresAt))
	mock.ExpectExec("UPDATE sessions SET last_accessed_at").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	_, err := repo.GetUserIdByToken(context.Background(), "consistent-token")
	assert.NoError(t, err)

	// Second call with same token - should use same hash
	mock.ExpectQuery("SELECT user_id, expires_at FROM sessions WHERE token_hash = \\?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "expires_at"}).AddRow(1, expiresAt))
	mock.ExpectExec("UPDATE sessions SET last_accessed_at").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	_, err = repo.GetUserIdByToken(context.Background(), "consistent-token")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_CreateSession_GeneratesUniqueCSRF(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSessionRepository(handles(db), slog.Default())

	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectExec("INSERT INTO sessions").
		WithArgs(1, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), expiresAt, sqlmock.AnyArg(), "192.168.1.1", "Mozilla/5.0").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO sessions").
		WithArgs(1, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), expiresAt, sqlmock.AnyArg(), "192.168.1.1", "Mozilla/5.0").
		WillReturnResult(sqlmock.NewResult(2, 1))

	csrf1, err := repo.CreateSession(context.Background(), 1, "token1", expiresAt, "192.168.1.1", "Mozilla/5.0")
	assert.NoError(t, err)

	csrf2, err := repo.CreateSession(context.Background(), 1, "token2", expiresAt, "192.168.1.1", "Mozilla/5.0")
	assert.NoError(t, err)

	// CSRF tokens should be different for different sessions
	assert.NotEqual(t, csrf1, csrf2)
	assert.NoError(t, mock.ExpectationsWereMet())
}
