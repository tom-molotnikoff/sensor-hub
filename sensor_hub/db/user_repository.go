package database

import (
	"context"
	"database/sql"
	"errors"
	gen "example/sensorHub/gen"
	"fmt"
	"log/slog"
	"time"
)

type SqlUserRepository struct {
	db     *Handles
	logger *slog.Logger
}

func NewUserRepository(db *Handles, logger *slog.Logger) *SqlUserRepository {
	return &SqlUserRepository{db: db, logger: logger.With("component", "user_repository")}
}

func (r *SqlUserRepository) CreateUser(ctx context.Context, user gen.User, passwordHash string) (int, error) {
	query := "INSERT INTO users (username, email, password_hash, must_change_password, disabled, created_at) VALUES (?, ?, ?, ?, ?, ?)"
	res, err := r.db.Writer.ExecContext(ctx, query, user.Username, user.Email, passwordHash, user.MustChangePassword, user.Disabled, time.Now())
	if err != nil {
		return 0, fmt.Errorf("error creating user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("error fetching last insert id: %w", err)
	}
	for _, role := range user.Roles {
		if err := r.AssignRoleToUser(ctx, int(id), role); err != nil {
			r.logger.Warn("failed to assign role to user", "role", role, "user_id", id, "error", err)
		}
	}
	return int(id), nil
}

func (r *SqlUserRepository) GetUserByUsername(ctx context.Context, username string) (*gen.User, string, error) {
	query := "SELECT id, username, email, must_change_password, disabled, created_at, updated_at, password_hash FROM users WHERE LOWER(username) = LOWER(?)"
	var user gen.User
	var passwordHash string
	var createdAt SQLiteTime
	var updatedAt NullSQLiteTime
	err := r.db.Reader.QueryRowContext(ctx, query, username).Scan(&user.Id, &user.Username, &user.Email, &user.MustChangePassword, &user.Disabled, &createdAt, &updatedAt, &passwordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("error querying user by username: %w", err)
	}
	user.CreatedAt = createdAt.Time
	if updatedAt.Valid {
		user.UpdatedAt = updatedAt.Time
	}
	roles, err := r.GetRolesForUser(ctx, user.Id)
	if err != nil {
		return nil, "", fmt.Errorf("error fetching roles for user: %w", err)
	}
	user.Roles = roles
	return &user, passwordHash, nil
}

func (r *SqlUserRepository) GetUserById(ctx context.Context, id int) (*gen.User, error) {
	query := "SELECT id, username, email, must_change_password, disabled, created_at, updated_at FROM users WHERE id = ?"
	var user gen.User
	var createdAt SQLiteTime
	var updatedAt NullSQLiteTime
	err := r.db.Reader.QueryRowContext(ctx, query, id).Scan(&user.Id, &user.Username, &user.Email, &user.MustChangePassword, &user.Disabled, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("error querying user by id: %w", err)
	}
	user.CreatedAt = createdAt.Time
	if updatedAt.Valid {
		user.UpdatedAt = updatedAt.Time
	}
	roles, err := r.GetRolesForUser(ctx, user.Id)
	if err != nil {
		return nil, fmt.Errorf("error fetching roles for user: %w", err)
	}
	user.Roles = roles
	return &user, nil
}

func (r *SqlUserRepository) ListUsers(ctx context.Context) ([]gen.User, error) {
	query := "SELECT id, username, email, must_change_password, disabled, created_at, updated_at FROM users"
	rows, err := r.db.Reader.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []gen.User
	for rows.Next() {
		var user gen.User
		var createdAt SQLiteTime
		var updatedAt NullSQLiteTime
		if err := rows.Scan(&user.Id, &user.Username, &user.Email, &user.MustChangePassword, &user.Disabled, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("error scanning user row: %w", err)
		}
		user.CreatedAt = createdAt.Time
		if updatedAt.Valid {
			user.UpdatedAt = updatedAt.Time
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over user rows: %w", err)
	}

	// Fetch roles after closing the rows cursor to avoid deadlock with MaxOpenConns(1)
	for i := range users {
		roles, err := r.GetRolesForUser(ctx, users[i].Id)
		if err != nil {
			return nil, fmt.Errorf("error fetching roles for user: %w", err)
		}
		users[i].Roles = roles
	}
	return users, nil
}

func (r *SqlUserRepository) UpdatePassword(ctx context.Context, userId int, passwordHash string, mustChange bool) error {
	query := "UPDATE users SET password_hash = ?, must_change_password = ?, updated_at = ? WHERE id = ?"
	_, err := r.db.Writer.ExecContext(ctx, query, passwordHash, mustChange, time.Now(), userId)
	if err != nil {
		return fmt.Errorf("error updating password for user: %w", err)
	}
	return nil
}

func (r *SqlUserRepository) SetDisabled(ctx context.Context, userId int, disabled bool) error {
	query := "UPDATE users SET disabled = ?, updated_at = ? WHERE id = ?"
	_, err := r.db.Writer.ExecContext(ctx, query, disabled, time.Now(), userId)
	if err != nil {
		return fmt.Errorf("error updating disabled flag for user: %w", err)
	}
	return nil
}

func (r *SqlUserRepository) AssignRoleToUser(ctx context.Context, userId int, roleName string) error {
	var roleId int
	err := r.db.Reader.QueryRowContext(ctx, "SELECT id FROM roles WHERE LOWER(name) = LOWER(?)", roleName).Scan(&roleId)
	if err != nil {
		return fmt.Errorf("error finding role %s: %w", roleName, err)
	}
	_, err = r.db.Writer.ExecContext(ctx, "INSERT OR IGNORE INTO user_roles (user_id, role_id) VALUES (?, ?)", userId, roleId)
	if err != nil {
		return fmt.Errorf("error assigning role to user: %w", err)
	}
	return nil
}

func (r *SqlUserRepository) GetRolesForUser(ctx context.Context, userId int) ([]string, error) {
	query := "SELECT r.name FROM roles r JOIN user_roles ur ON r.id = ur.role_id WHERE ur.user_id = ?"
	rows, err := r.db.Reader.QueryContext(ctx, query, userId)
	if err != nil {
		return nil, fmt.Errorf("error querying roles for user: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, fmt.Errorf("error scanning role row: %w", err)
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over role rows: %w", err)
	}
	return roles, nil
}

func (r *SqlUserRepository) DeleteSessionsForUser(ctx context.Context, userId int) error {
	_, err := r.db.Writer.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ?", userId)
	if err != nil {
		return fmt.Errorf("error deleting sessions for user: %w", err)
	}
	return nil
}

func (r *SqlUserRepository) DeleteSessionsForUserExcept(ctx context.Context, userId int, keepToken string) error {
	if keepToken == "" {
		return r.DeleteSessionsForUser(ctx, userId)
	}
	keepHash := tokenHash(keepToken)
	var cnt int
	err := r.db.Reader.QueryRowContext(ctx, "SELECT COUNT(1) FROM sessions WHERE user_id = ? AND token_hash = ?", userId, keepHash).Scan(&cnt)
	if err != nil {
		return fmt.Errorf("error checking current session existence: %w", err)
	}
	if cnt == 0 {
		r.logger.Warn("session token not found for user; skipping deletion to avoid lockout", "user_id", userId)
		return nil
	}
	res, err := r.db.Writer.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ? AND token_hash != ?", userId, keepHash)
	if err != nil {
		return fmt.Errorf("error deleting sessions for user except token: %w", err)
	}
	if ra, err := res.RowsAffected(); err == nil {
		r.logger.Info("deleted sessions for user", "deleted_count", ra, "user_id", userId, "preserved_token_hash", keepHash)
	}
	return nil
}

func (r *SqlUserRepository) DeleteUserById(ctx context.Context, userId int) error {
	tx, err := r.db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, "DELETE FROM user_roles WHERE user_id = ?", userId); err != nil {
		return fmt.Errorf("error deleting user roles: %w", err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM api_keys WHERE user_id = ?", userId); err != nil {
		return fmt.Errorf("error deleting api keys for user: %w", err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ?", userId); err != nil {
		return fmt.Errorf("error deleting sessions for user: %w", err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM sensor_command_history WHERE user_id = ?", userId); err != nil {
		return fmt.Errorf("error deleting sensor command history for user: %w", err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM users WHERE id = ?", userId); err != nil {
		return fmt.Errorf("error deleting user: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error committing delete user transaction: %w", err)
	}
	return nil
}

func (r *SqlUserRepository) SetMustChangeFlag(ctx context.Context, userId int, mustChange bool) error {
	_, err := r.db.Writer.ExecContext(ctx, "UPDATE users SET must_change_password = ?, updated_at = ? WHERE id = ?", mustChange, time.Now(), userId)
	if err != nil {
		return fmt.Errorf("error updating must_change_password: %w", err)
	}
	return nil
}

func (r *SqlUserRepository) SetRolesForUser(ctx context.Context, userId int, roles []string) error {
	tx, err := r.db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, "DELETE FROM user_roles WHERE user_id = ?", userId); err != nil {
		return fmt.Errorf("error clearing user roles: %w", err)
	}
	for _, role := range roles {
		var roleId int
		if err := tx.QueryRowContext(ctx, "SELECT id FROM roles WHERE LOWER(name) = LOWER(?)", role).Scan(&roleId); err != nil {
			return fmt.Errorf("error finding role %s: %w", role, err)
		}
		if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO user_roles (user_id, role_id) VALUES (?, ?)", userId, roleId); err != nil {
			return fmt.Errorf("error assigning role %s to user: %w", role, err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error committing set roles transaction: %w", err)
	}
	return nil
}
