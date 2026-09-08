package database

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

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
	UpdateApiKeyExpiry(ctx context.Context, id int, expiresAt *time.Time) error
	RevokeApiKey(ctx context.Context, id int) error
	DeleteApiKey(ctx context.Context, id int) error
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

func (r *SqlApiKeyRepository) UpdateApiKeyExpiry(ctx context.Context, id int, expiresAt *time.Time) error {
	_, err := r.db.Writer.ExecContext(ctx,
		`UPDATE api_keys SET expires_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		expiresAt, id,
	)
	return err
}

func (r *SqlApiKeyRepository) RevokeApiKey(ctx context.Context, id int) error {
	_, err := r.db.Writer.ExecContext(ctx,
		`UPDATE api_keys SET revoked = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		id,
	)
	return err
}

func (r *SqlApiKeyRepository) DeleteApiKey(ctx context.Context, id int) error {
	_, err := r.db.Writer.ExecContext(ctx, `DELETE FROM api_keys WHERE id = ?`, id)
	return err
}

func (r *SqlApiKeyRepository) UpdateLastUsed(ctx context.Context, id int) error {
	_, err := r.db.Writer.ExecContext(ctx,
		`UPDATE api_keys SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?`,
		id,
	)
	return err
}
