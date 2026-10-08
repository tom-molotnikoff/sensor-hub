package database

import (
	"context"
	"database/sql"
	"log/slog"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
// Owner-scoped write tests: revoke, delete and update expiry
// ============================================================================

// apiKeyFixture is two users with one key each, in a migrated database.
type apiKeyFixture struct {
	repo       *SqlApiKeyRepository
	h          *Handles
	owner      int
	other      int
	ownerKeyId int
	otherKeyId int
}

func newApiKeyFixture(t *testing.T) apiKeyFixture {
	t.Helper()
	h := newMigratedHandles(t)
	repo := NewApiKeyRepository(h, slog.Default())
	f := apiKeyFixture{repo: repo, h: h,
		owner: insertTestUser(t, h, "owner", "viewer", false),
		other: insertTestUser(t, h, "other", "admin", false),
	}
	id, err := repo.CreateApiKey(context.Background(), "owner-key", "shk_owner", "owner-hash", f.owner, nil)
	require.NoError(t, err)
	f.ownerKeyId = int(id)
	id, err = repo.CreateApiKey(context.Background(), "other-key", "shk_other", "other-hash", f.other, nil)
	require.NoError(t, err)
	f.otherKeyId = int(id)
	return f
}

func (f apiKeyFixture) key(t *testing.T, id int) (revoked bool, expiresAt sql.NullString, exists bool) {
	t.Helper()
	err := f.h.Reader.QueryRow("SELECT revoked, expires_at FROM api_keys WHERE id = ?", id).Scan(&revoked, &expiresAt)
	if err == sql.ErrNoRows {
		return false, expiresAt, false
	}
	require.NoError(t, err)
	return revoked, expiresAt, true
}

func TestApiKeyRepository_ScopedWritesRefuseAnotherUsersKey(t *testing.T) {
	f := newApiKeyFixture(t)
	ctx := context.Background()
	expiry := time.Now().Add(time.Hour)

	assert.ErrorIs(t, f.repo.RevokeApiKey(ctx, f.otherKeyId, &f.owner), ErrApiKeyNotFound)
	assert.ErrorIs(t, f.repo.UpdateApiKeyExpiry(ctx, f.otherKeyId, &f.owner, &expiry), ErrApiKeyNotFound)
	assert.ErrorIs(t, f.repo.DeleteApiKey(ctx, f.otherKeyId, &f.owner), ErrApiKeyNotFound)

	revoked, expiresAt, exists := f.key(t, f.otherKeyId)
	assert.True(t, exists)
	assert.False(t, revoked)
	assert.False(t, expiresAt.Valid)
}

func TestApiKeyRepository_ScopedWritesActOnTheOwnersKey(t *testing.T) {
	f := newApiKeyFixture(t)
	ctx := context.Background()
	expiry := time.Now().Add(time.Hour)

	require.NoError(t, f.repo.UpdateApiKeyExpiry(ctx, f.ownerKeyId, &f.owner, &expiry))
	require.NoError(t, f.repo.RevokeApiKey(ctx, f.ownerKeyId, &f.owner))
	revoked, expiresAt, _ := f.key(t, f.ownerKeyId)
	assert.True(t, revoked)
	assert.True(t, expiresAt.Valid)

	require.NoError(t, f.repo.UpdateApiKeyExpiry(ctx, f.ownerKeyId, &f.owner, nil))
	_, expiresAt, _ = f.key(t, f.ownerKeyId)
	assert.False(t, expiresAt.Valid, "a nil expiry clears it")

	require.NoError(t, f.repo.DeleteApiKey(ctx, f.ownerKeyId, &f.owner))
	_, _, exists := f.key(t, f.ownerKeyId)
	assert.False(t, exists)
}

func TestApiKeyRepository_UnscopedWritesActOnAnyKey(t *testing.T) {
	f := newApiKeyFixture(t)
	ctx := context.Background()
	expiry := time.Now().Add(time.Hour)

	require.NoError(t, f.repo.UpdateApiKeyExpiry(ctx, f.ownerKeyId, nil, &expiry))
	require.NoError(t, f.repo.RevokeApiKey(ctx, f.ownerKeyId, nil))
	revoked, expiresAt, _ := f.key(t, f.ownerKeyId)
	assert.True(t, revoked)
	assert.True(t, expiresAt.Valid)

	require.NoError(t, f.repo.DeleteApiKey(ctx, f.ownerKeyId, nil))
	_, _, exists := f.key(t, f.ownerKeyId)
	assert.False(t, exists)
}

func TestApiKeyRepository_WritesOnAMissingKey(t *testing.T) {
	f := newApiKeyFixture(t)
	ctx := context.Background()

	assert.ErrorIs(t, f.repo.RevokeApiKey(ctx, 999, nil), ErrApiKeyNotFound)
	assert.ErrorIs(t, f.repo.UpdateApiKeyExpiry(ctx, 999, nil, nil), ErrApiKeyNotFound)
	assert.ErrorIs(t, f.repo.DeleteApiKey(ctx, 999, nil), ErrApiKeyNotFound)
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
