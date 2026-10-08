package database

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"
)

// ErrApiKeyNotFound is returned when no key matches the id, or, for an
// owner-scoped operation, when the key belongs to another user.
var ErrApiKeyNotFound = errors.New("api key not found")

type ApiKey struct {
	Id         int        `json:"id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	KeyHash    string     `json:"-"`
	UserId     int        `json:"user_id"`
	ExpiresAt  *time.Time `json:"expires_at"`
	Revoked    bool       `json:"revoked"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type ApiKeyRepository interface {
	CreateApiKey(ctx context.Context, name string, keyPrefix string, keyHash string, userId int, expiresAt *time.Time) (int64, error)
	GetApiKeyByHash(ctx context.Context, keyHash string) (*ApiKey, error)
	ListApiKeysForUser(ctx context.Context, userId int) ([]ApiKey, error)
	// UpdateApiKeyExpiry, RevokeApiKey and DeleteApiKey act on the key with the
	// given id. A non-nil ownerId restricts them to that user's key. They
	// return ErrApiKeyNotFound when no key matched.
	UpdateApiKeyExpiry(ctx context.Context, id int, ownerId *int, expiresAt *time.Time) error
	RevokeApiKey(ctx context.Context, id int, ownerId *int) error
	DeleteApiKey(ctx context.Context, id int, ownerId *int) error
	UpdateLastUsed(ctx context.Context, id int) error
}

type SqlApiKeyRepository struct {
	db     *Handles
	logger *slog.Logger
}

func NewApiKeyRepository(db *Handles, logger *slog.Logger) *SqlApiKeyRepository {
	return &SqlApiKeyRepository{db: db, logger: logger.With("component", "api_key_repository")}
}

func (r *SqlApiKeyRepository) CreateApiKey(ctx context.Context, name string, keyPrefix string, keyHash string, userId int, expiresAt *time.Time) (int64, error) {
	result, err := r.db.Writer.ExecContext(ctx,
		`INSERT INTO api_keys (name, key_prefix, key_hash, user_id, expires_at) VALUES (?, ?, ?, ?, ?)`,
		name, keyPrefix, keyHash, userId, expiresAt,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r *SqlApiKeyRepository) GetApiKeyByHash(ctx context.Context, keyHash string) (*ApiKey, error) {
	row := r.db.Reader.QueryRowContext(ctx,
		`SELECT id, name, key_prefix, key_hash, user_id, expires_at, revoked, last_used_at, created_at, updated_at
		 FROM api_keys
		 WHERE key_hash = ? AND revoked = 0 AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)`,
		keyHash,
	)

	var key ApiKey
	var expiresAt NullSQLiteTime
	var lastUsedAt NullSQLiteTime
	var createdAt SQLiteTime
	var updatedAt SQLiteTime

	err := row.Scan(
		&key.Id, &key.Name, &key.KeyPrefix, &key.KeyHash, &key.UserId,
		&expiresAt, &key.Revoked, &lastUsedAt, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if expiresAt.Valid {
		key.ExpiresAt = &expiresAt.Time
	}
	if lastUsedAt.Valid {
		key.LastUsedAt = &lastUsedAt.Time
	}
	key.CreatedAt = createdAt.Time
	key.UpdatedAt = updatedAt.Time

	return &key, nil
}

func (r *SqlApiKeyRepository) ListApiKeysForUser(ctx context.Context, userId int) ([]ApiKey, error) {
	rows, err := r.db.Reader.QueryContext(ctx,
		`SELECT id, name, key_prefix, user_id, expires_at, revoked, last_used_at, created_at, updated_at
		 FROM api_keys WHERE user_id = ? ORDER BY created_at DESC`,
		userId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []ApiKey
	for rows.Next() {
		var key ApiKey
		var expiresAt NullSQLiteTime
		var lastUsedAt NullSQLiteTime
		var createdAt SQLiteTime
		var updatedAt SQLiteTime

		err := rows.Scan(
			&key.Id, &key.Name, &key.KeyPrefix, &key.UserId,
			&expiresAt, &key.Revoked, &lastUsedAt, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}

		if expiresAt.Valid {
			key.ExpiresAt = &expiresAt.Time
		}
		if lastUsedAt.Valid {
			key.LastUsedAt = &lastUsedAt.Time
		}
		key.CreatedAt = createdAt.Time
		key.UpdatedAt = updatedAt.Time

		keys = append(keys, key)
	}

	if keys == nil {
		keys = []ApiKey{}
	}

	return keys, rows.Err()
}

func (r *SqlApiKeyRepository) UpdateApiKeyExpiry(ctx context.Context, id int, ownerId *int, expiresAt *time.Time) error {
	where, args := apiKeyMatch(id, ownerId)
	return r.execOnKey(ctx, `UPDATE api_keys SET expires_at = ?, updated_at = CURRENT_TIMESTAMP WHERE `+where, append([]any{expiresAt}, args...)...)
}

func (r *SqlApiKeyRepository) RevokeApiKey(ctx context.Context, id int, ownerId *int) error {
	where, args := apiKeyMatch(id, ownerId)
	return r.execOnKey(ctx, `UPDATE api_keys SET revoked = 1, updated_at = CURRENT_TIMESTAMP WHERE `+where, args...)
}

func (r *SqlApiKeyRepository) DeleteApiKey(ctx context.Context, id int, ownerId *int) error {
	where, args := apiKeyMatch(id, ownerId)
	return r.execOnKey(ctx, `DELETE FROM api_keys WHERE `+where, args...)
}

// apiKeyMatch is the WHERE clause for a single key: by id alone, or by id
// AND user_id when the operation is scoped to the key's owner.
func apiKeyMatch(id int, ownerId *int) (string, []any) {
	if ownerId == nil {
		return "id = ?", []any{id}
	}
	return "id = ? AND user_id = ?", []any{id, *ownerId}
}

func (r *SqlApiKeyRepository) execOnKey(ctx context.Context, query string, args ...any) error {
	res, err := r.db.Writer.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrApiKeyNotFound
	}
	return nil
}

func (r *SqlApiKeyRepository) UpdateLastUsed(ctx context.Context, id int) error {
	_, err := r.db.Writer.ExecContext(ctx,
		`UPDATE api_keys SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?`,
		id,
	)
	return err
}
