package database

import (
	"context"
	"database/sql"
	"log/slog"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// CreateApiKey tests
// ============================================================================

func TestApiKeyRepository_CreateApiKey_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewApiKeyRepository(handles(db), slog.Default())

	mock.ExpectExec("INSERT INTO api_keys").
		WithArgs("my-key", "shk_abcd", "hash123", 1, nil).
		WillReturnResult(sqlmock.NewResult(1, 1))

	id, err := repo.CreateApiKey(context.Background(), "my-key", "shk_abcd", "hash123", 1, nil)

	assert.NoError(t, err)
	assert.Equal(t, int64(1), id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApiKeyRepository_CreateApiKey_WithExpiry(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewApiKeyRepository(handles(db), slog.Default())

	expiry := time.Now().Add(24 * time.Hour)
	mock.ExpectExec("INSERT INTO api_keys").
		WithArgs("my-key", "shk_abcd", "hash123", 1, expiry).
		WillReturnResult(sqlmock.NewResult(2, 1))

	id, err := repo.CreateApiKey(context.Background(), "my-key", "shk_abcd", "hash123", 1, &expiry)

	assert.NoError(t, err)
	assert.Equal(t, int64(2), id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApiKeyRepository_CreateApiKey_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewApiKeyRepository(handles(db), slog.Default())

	mock.ExpectExec("INSERT INTO api_keys").
		WithArgs("my-key", "shk_abcd", "hash123", 1, nil).
		WillReturnError(sql.ErrConnDone)

	_, err := repo.CreateApiKey(context.Background(), "my-key", "shk_abcd", "hash123", 1, nil)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GetApiKeyByHash tests
// ============================================================================

var apiKeyColumns = []string{"id", "name", "key_prefix", "key_hash", "user_id", "expires_at", "revoked", "last_used_at", "created_at", "updated_at"}

func TestApiKeyRepository_GetApiKeyByHash_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewApiKeyRepository(handles(db), slog.Default())

	now := time.Now()
	rows := sqlmock.NewRows(apiKeyColumns).
		AddRow(1, "my-key", "shk_abcd", "hash123", 1, nil, false, nil, now.Format("2006-01-02 15:04:05"), now.Format("2006-01-02 15:04:05"))

	mock.ExpectQuery("SELECT .+ FROM api_keys").
		WithArgs("hash123").
		WillReturnRows(rows)

	key, err := repo.GetApiKeyByHash(context.Background(), "hash123")

	assert.NoError(t, err)
	assert.NotNil(t, key)
	assert.Equal(t, 1, key.Id)
	assert.Equal(t, "my-key", key.Name)
	assert.Equal(t, "shk_abcd", key.KeyPrefix)
	assert.Equal(t, 1, key.UserId)
	assert.Nil(t, key.ExpiresAt)
	assert.Nil(t, key.LastUsedAt)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApiKeyRepository_GetApiKeyByHash_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewApiKeyRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT .+ FROM api_keys").
		WithArgs("nonexistent").
		WillReturnError(sql.ErrNoRows)

	key, err := repo.GetApiKeyByHash(context.Background(), "nonexistent")

	assert.NoError(t, err)
	assert.Nil(t, key)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApiKeyRepository_GetApiKeyByHash_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewApiKeyRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT .+ FROM api_keys").
		WithArgs("hash123").
		WillReturnError(sql.ErrConnDone)

	key, err := repo.GetApiKeyByHash(context.Background(), "hash123")

	assert.Error(t, err)
	assert.Nil(t, key)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// ListApiKeysForUser tests
// ============================================================================

var listApiKeyColumns = []string{"id", "name", "key_prefix", "user_id", "expires_at", "revoked", "last_used_at", "created_at", "updated_at"}

func TestApiKeyRepository_ListApiKeysForUser_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewApiKeyRepository(handles(db), slog.Default())

	now := time.Now()
	rows := sqlmock.NewRows(listApiKeyColumns).
		AddRow(1, "key-1", "shk_aaaa", 1, nil, false, nil, now.Format("2006-01-02 15:04:05"), now.Format("2006-01-02 15:04:05")).
		AddRow(2, "key-2", "shk_bbbb", 1, now.Add(48*time.Hour).Format("2006-01-02 15:04:05"), true, now.Format("2006-01-02 15:04:05"), now.Format("2006-01-02 15:04:05"), now.Format("2006-01-02 15:04:05"))

	mock.ExpectQuery("SELECT .+ FROM api_keys WHERE user_id").
		WithArgs(1).
		WillReturnRows(rows)

	keys, err := repo.ListApiKeysForUser(context.Background(), 1)

	assert.NoError(t, err)
	assert.Len(t, keys, 2)
	assert.Equal(t, "key-1", keys[0].Name)
	assert.Equal(t, "key-2", keys[1].Name)
	assert.True(t, keys[1].Revoked)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApiKeyRepository_ListApiKeysForUser_Empty(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewApiKeyRepository(handles(db), slog.Default())

	rows := sqlmock.NewRows(listApiKeyColumns)
	mock.ExpectQuery("SELECT .+ FROM api_keys WHERE user_id").
		WithArgs(1).
		WillReturnRows(rows)

	keys, err := repo.ListApiKeysForUser(context.Background(), 1)

	assert.NoError(t, err)
	assert.NotNil(t, keys)
	assert.Empty(t, keys)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// RevokeApiKey tests
// ============================================================================

func TestApiKeyRepository_RevokeApiKey_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewApiKeyRepository(handles(db), slog.Default())

	mock.ExpectExec("UPDATE api_keys SET revoked = 1").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.RevokeApiKey(context.Background(), 1)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// DeleteApiKey tests
// ============================================================================

func TestApiKeyRepository_DeleteApiKey_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewApiKeyRepository(handles(db), slog.Default())

	mock.ExpectExec("DELETE FROM api_keys").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.DeleteApiKey(context.Background(), 1)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// UpdateLastUsed tests
// ============================================================================

func TestApiKeyRepository_UpdateLastUsed_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewApiKeyRepository(handles(db), slog.Default())

	mock.ExpectExec("UPDATE api_keys SET last_used_at").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateLastUsed(context.Background(), 1)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// UpdateApiKeyExpiry tests
// ============================================================================

func TestApiKeyRepository_UpdateApiKeyExpiry_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewApiKeyRepository(handles(db), slog.Default())

	expiry := time.Now().Add(72 * time.Hour)
	mock.ExpectExec("UPDATE api_keys SET expires_at").
		WithArgs(expiry, 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateApiKeyExpiry(context.Background(), 1, &expiry)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApiKeyRepository_UpdateApiKeyExpiry_ClearExpiry(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewApiKeyRepository(handles(db), slog.Default())

	mock.ExpectExec("UPDATE api_keys SET expires_at").
		WithArgs(nil, 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateApiKeyExpiry(context.Background(), 1, nil)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
