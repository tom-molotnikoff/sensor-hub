package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type FailedLoginRepository interface {
	RecordFailedAttempt(ctx context.Context, username string, userId *int, ip string, reason string) error
	CountRecentFailedAttemptsByUsername(ctx context.Context, username string, window time.Duration) (int, error)
	CountRecentFailedAttemptsByIP(ctx context.Context, ip string, window time.Duration) (int, error)
	DeleteRecentFailedAttemptsByIP(ctx context.Context, ip string, window time.Duration) error
	DeleteAttemptsOlderThan(ctx context.Context, threshold time.Time) error
}

type SqlFailedLoginRepository struct {
	db     *Handles
	logger *slog.Logger
}

func NewFailedLoginRepository(db *Handles, logger *slog.Logger) *SqlFailedLoginRepository {
	return &SqlFailedLoginRepository{db: db, logger: logger.With("component", "failed_login_repository")}
}

func (r *SqlFailedLoginRepository) RecordFailedAttempt(ctx context.Context, username string, userId *int, ip string, reason string) error {
	r.logger.Info("recording failed login attempt", "username", username, "user_id", userId, "ip", ip, "reason", reason)
	query := "INSERT INTO failed_login_attempts (username, user_id, ip_address, attempt_time, reason) VALUES (?, ?, ?, ?, ?)"
	_, err := r.db.Writer.ExecContext(ctx, query, username, userId, ip, time.Now(), reason)
	if err != nil {
		return fmt.Errorf("error recording failed login attempt: %w", err)
	}
	return nil
}

func (r *SqlFailedLoginRepository) CountRecentFailedAttemptsByUsername(ctx context.Context, username string, window time.Duration) (int, error) {
	query := "SELECT COUNT(1) FROM failed_login_attempts WHERE LOWER(username) = LOWER(?) AND attempt_time > ?"
	threshold := time.Now().Add(-window)
	var count int
	err := r.db.Reader.QueryRowContext(ctx, query, username, threshold).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("error counting failed attempts by username: %w", err)
	}
	return count, nil
}

func (r *SqlFailedLoginRepository) CountRecentFailedAttemptsByIP(ctx context.Context, ip string, window time.Duration) (int, error) {
	query := "SELECT COUNT(1) FROM failed_login_attempts WHERE ip_address = ? AND attempt_time > ?"
	threshold := time.Now().Add(-window)
	var count int
	err := r.db.Reader.QueryRowContext(ctx, query, ip, threshold).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("error counting failed attempts by ip: %w", err)
	}
	return count, nil
}

func (r *SqlFailedLoginRepository) DeleteRecentFailedAttemptsByIP(ctx context.Context, ip string, window time.Duration) error {
	query := "DELETE FROM failed_login_attempts WHERE ip_address = ? AND attempt_time > ?"
	threshold := time.Now().Add(-window)
	_, err := r.db.Writer.ExecContext(ctx, query, ip, threshold)
	if err != nil {
		return fmt.Errorf("error deleting failed attempts by ip: %w", err)
	}
	return nil
}

func (r *SqlFailedLoginRepository) DeleteAttemptsOlderThan(ctx context.Context, threshold time.Time) error {
	query := "DELETE FROM failed_login_attempts WHERE attempt_time < ?"
	_, err := r.db.Writer.ExecContext(ctx, query, threshold)
	if err != nil {
		return fmt.Errorf("error deleting old failed login attempts: %w", err)
	}
	return nil
}
