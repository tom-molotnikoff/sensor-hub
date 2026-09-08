package database

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	gen "example/sensorHub/gen"
)

type SessionRepository interface {
	CreateSession(ctx context.Context, userId int, rawToken string, expiresAt time.Time, ip string, userAgent string) (string, error) // returns csrfToken
	GetAuthenticatedUserByToken(ctx context.Context, rawToken string) (*gen.User, time.Time, error)
	TouchSession(ctx context.Context, rawToken string) error
	GetSessionIdByToken(ctx context.Context, rawToken string) (int64, error)
	DeleteSessionByToken(ctx context.Context, rawToken string) error
	DeleteSessionsForUser(ctx context.Context, userId int) error
	ListSessionsForUser(ctx context.Context, userId int) ([]SessionInfo, error)
	RevokeSessionById(ctx context.Context, sessionId int64) error
	GetCSRFForToken(ctx context.Context, rawToken string) (string, error)
	InsertSessionAudit(ctx context.Context, sessionId int64, revokedByUserId *int, eventType string, reason *string) error
}

type SessionInfo struct {
	Id             int64     `json:"id"`
	UserId         int       `json:"user_id"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	LastAccessedAt time.Time `json:"last_accessed_at"`
	IpAddress      string    `json:"ip_address"`
	UserAgent      string    `json:"user_agent"`
}

type SqlSessionRepository struct {
	db     *Handles
	logger *slog.Logger
}

func NewSessionRepository(db *Handles, logger *slog.Logger) *SqlSessionRepository {
	return &SqlSessionRepository{db: db, logger: logger.With("component", "session_repository")}
}

func tokenHash(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func generateCSRFToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (r *SqlSessionRepository) CreateSession(ctx context.Context, userId int, rawToken string, expiresAt time.Time, ip string, userAgent string) (string, error) {
	csrf, err := generateCSRFToken(24)
	if err != nil {
		return "", fmt.Errorf("failed to generate csrf token: %w", err)
	}
	query := "INSERT INTO sessions (user_id, token_hash, csrf_token, created_at, expires_at, last_accessed_at, ip_address, user_agent) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	_, err = r.db.Writer.ExecContext(ctx, query, userId, tokenHash(rawToken), csrf, time.Now(), expiresAt, time.Now(), ip, userAgent)
	if err != nil {
		return "", fmt.Errorf("error creating session: %w", err)
	}
	return csrf, nil
}

func (r *SqlSessionRepository) GetAuthenticatedUserByToken(ctx context.Context, rawToken string) (*gen.User, time.Time, error) {
	query := `SELECT s.expires_at, s.last_accessed_at,
		u.id, u.username, u.email, u.must_change_password, u.disabled, u.created_at, u.updated_at,
		(SELECT group_concat(r.name, char(31))
			FROM user_roles ur JOIN roles r ON r.id = ur.role_id
			WHERE ur.user_id = u.id),
		(SELECT group_concat(p.name, char(31))
			FROM user_roles ur
			JOIN role_permissions rp ON rp.role_id = ur.role_id
			JOIN permissions p ON p.id = rp.permission_id
			WHERE ur.user_id = u.id)
	FROM sessions s JOIN users u ON u.id = s.user_id
	WHERE s.token_hash = ?`
	var user gen.User
	var expiresAt, lastAccessedAt, createdAt SQLiteTime
	var updatedAt NullSQLiteTime
	var roles, permissions sql.NullString
	err := r.db.Reader.QueryRowContext(ctx, query, tokenHash(rawToken)).Scan(
		&expiresAt, &lastAccessedAt,
		&user.Id, &user.Username, &user.Email, &user.MustChangePassword, &user.Disabled, &createdAt, &updatedAt,
		&roles, &permissions,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, time.Time{}, nil
		}
		return nil, time.Time{}, fmt.Errorf("error querying authenticated user: %w", err)
	}
	if time.Now().After(expiresAt.Time) {
		_ = r.DeleteSessionByToken(ctx, rawToken)
		return nil, time.Time{}, nil
	}
	user.CreatedAt = createdAt.Time
	if updatedAt.Valid {
		user.UpdatedAt = updatedAt.Time
	}
	user.Roles = splitConcatenated(roles)
	user.Permissions = splitConcatenated(permissions)
	return &user, lastAccessedAt.Time, nil
}

func splitConcatenated(v sql.NullString) []string {
	if !v.Valid || v.String == "" {
		return nil
	}
	parts := strings.Split(v.String, "\x1f")
	out := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, p := range parts {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

func (r *SqlSessionRepository) TouchSession(ctx context.Context, rawToken string) error {
	_, err := r.db.Writer.ExecContext(ctx, "UPDATE sessions SET last_accessed_at = ? WHERE token_hash = ?", time.Now(), tokenHash(rawToken))
	if err != nil {
		return fmt.Errorf("error updating last accessed time: %w", err)
	}
	return nil
}

func (r *SqlSessionRepository) GetSessionIdByToken(ctx context.Context, rawToken string) (int64, error) {
	query := "SELECT id, expires_at FROM sessions WHERE token_hash = ?"
	var id int64
	var expiresAt SQLiteTime
	err := r.db.Reader.QueryRowContext(ctx, query, tokenHash(rawToken)).Scan(&id, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("error querying session id: %w", err)
	}
	if time.Now().After(expiresAt.Time) {
		_ = r.DeleteSessionByToken(ctx, rawToken)
		return 0, nil
	}
	return id, nil
}

func (r *SqlSessionRepository) DeleteSessionByToken(ctx context.Context, rawToken string) error {
	_, err := r.db.Writer.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = ?", tokenHash(rawToken))
	if err != nil {
		return fmt.Errorf("error deleting session: %w", err)
	}
	return nil
}

func (r *SqlSessionRepository) DeleteSessionsForUser(ctx context.Context, userId int) error {
	_, err := r.db.Writer.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ?", userId)
	if err != nil {
		return fmt.Errorf("error deleting sessions for user: %w", err)
	}
	return nil
}

func (r *SqlSessionRepository) ListSessionsForUser(ctx context.Context, userId int) ([]SessionInfo, error) {
	rows, err := r.db.Reader.QueryContext(ctx, "SELECT id, user_id, created_at, expires_at, last_accessed_at, ip_address, user_agent FROM sessions WHERE user_id = ? ORDER BY created_at DESC", userId)
	if err != nil {
		return nil, fmt.Errorf("error querying sessions for user: %w", err)
	}
	defer rows.Close()
	var sessions []SessionInfo
	for rows.Next() {
		var s SessionInfo
		var createdAt, expiresAt, lastAccessedAt SQLiteTime
		if err := rows.Scan(&s.Id, &s.UserId, &createdAt, &expiresAt, &lastAccessedAt, &s.IpAddress, &s.UserAgent); err != nil {
			return nil, fmt.Errorf("error scanning session row: %w", err)
		}
		s.CreatedAt = createdAt.Time
		s.ExpiresAt = expiresAt.Time
		s.LastAccessedAt = lastAccessedAt.Time
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sessions rows: %w", err)
	}
	return sessions, nil
}

func (r *SqlSessionRepository) RevokeSessionById(ctx context.Context, sessionId int64) error {
	_, err := r.db.Writer.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", sessionId)
	if err != nil {
		return fmt.Errorf("error revoking session: %w", err)
	}
	return nil
}

func (r *SqlSessionRepository) GetCSRFForToken(ctx context.Context, rawToken string) (string, error) {
	query := "SELECT csrf_token, expires_at FROM sessions WHERE token_hash = ?"
	var csrf sql.NullString
	var expiresAt SQLiteTime
	err := r.db.Reader.QueryRowContext(ctx, query, tokenHash(rawToken)).Scan(&csrf, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("error querying csrf token: %w", err)
	}
	if time.Now().After(expiresAt.Time) {
		_ = r.DeleteSessionByToken(ctx, rawToken)
		return "", nil
	}
	if csrf.Valid {
		return csrf.String, nil
	}
	return "", nil
}

func (r *SqlSessionRepository) InsertSessionAudit(ctx context.Context, sessionId int64, revokedByUserId *int, eventType string, reason *string) error {
	_, err := r.db.Writer.ExecContext(ctx, "INSERT INTO session_audit (session_id, revoked_by_user_id, event_type, reason, created_at) VALUES (?, ?, ?, ?, ?)", sessionId, revokedByUserId, eventType, reason, time.Now())
	if err != nil {
		return fmt.Errorf("error inserting session audit: %w", err)
	}
	return nil
}
