package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	gen "example/sensorHub/gen"
)

type SqlDashboardRepository struct {
	db     *Handles
	logger *slog.Logger
}

func NewDashboardRepository(db *Handles, logger *slog.Logger) *SqlDashboardRepository {
	return &SqlDashboardRepository{
		db:     db,
		logger: logger.With("component", "dashboard_repository"),
	}
}

func (r *SqlDashboardRepository) Create(ctx context.Context, dashboard *gen.Dashboard) (int, error) {
	query := `INSERT INTO dashboards (user_id, name, config, shared, is_default) VALUES (?, ?, ?, ?, ?)`
	result, err := r.db.Writer.ExecContext(ctx, query, dashboard.UserId, dashboard.Name, dashboard.Config, dashboard.Shared, dashboard.IsDefault)
	if err != nil {
		return 0, fmt.Errorf("error creating dashboard: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("error getting last insert id: %w", err)
	}
	r.logger.Debug("created dashboard", "id", id, "user_id", dashboard.UserId, "name", dashboard.Name)
	return int(id), nil
}

func (r *SqlDashboardRepository) GetById(ctx context.Context, id int) (*gen.Dashboard, error) {
	query := `SELECT id, user_id, name, config, shared, is_default, created_at, updated_at FROM dashboards WHERE id = ?`
	row := r.db.Reader.QueryRowContext(ctx, query, id)
	d := &gen.Dashboard{}
	var createdAt, updatedAt SQLiteTime
	err := row.Scan(&d.Id, &d.UserId, &d.Name, &d.Config, &d.Shared, &d.IsDefault, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error querying dashboard by id: %w", err)
	}
	d.CreatedAt = createdAt.Time
	d.UpdatedAt = updatedAt.Time
	r.logger.Debug("fetched dashboard", "id", id)
	return d, nil
}

func (r *SqlDashboardRepository) GetByUserId(ctx context.Context, userId int) ([]gen.Dashboard, error) {
	query := `SELECT id, user_id, name, config, shared, is_default, created_at, updated_at
		FROM dashboards WHERE user_id = ? OR shared = 1 ORDER BY is_default DESC, name ASC`
	rows, err := r.db.Reader.QueryContext(ctx, query, userId)
	if err != nil {
		return nil, fmt.Errorf("error querying dashboards for user: %w", err)
	}
	defer rows.Close()

	var dashboards []gen.Dashboard
	for rows.Next() {
		var d gen.Dashboard
		var createdAt, updatedAt SQLiteTime
		if err := rows.Scan(&d.Id, &d.UserId, &d.Name, &d.Config, &d.Shared, &d.IsDefault, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("error scanning dashboard row: %w", err)
		}
		d.CreatedAt = createdAt.Time
		d.UpdatedAt = updatedAt.Time
		dashboards = append(dashboards, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating dashboard rows: %w", err)
	}
	r.logger.Debug("fetched dashboards for user", "user_id", userId, "count", len(dashboards))
	return dashboards, nil
}

func (r *SqlDashboardRepository) Update(ctx context.Context, dashboard *gen.Dashboard) error {
	query := `UPDATE dashboards SET name = ?, config = ?, shared = ?, updated_at = datetime('now') WHERE id = ?`
	_, err := r.db.Writer.ExecContext(ctx, query, dashboard.Name, dashboard.Config, dashboard.Shared, dashboard.Id)
	if err != nil {
		return fmt.Errorf("error updating dashboard: %w", err)
	}
	r.logger.Debug("updated dashboard", "id", dashboard.Id)
	return nil
}

func (r *SqlDashboardRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM dashboards WHERE id = ?`
	_, err := r.db.Writer.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting dashboard: %w", err)
	}
	r.logger.Debug("deleted dashboard", "id", id)
	return nil
}

func (r *SqlDashboardRepository) SetDefault(ctx context.Context, userId int, dashboardId int) error {
	tx, err := r.db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error beginning transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(ctx, `UPDATE dashboards SET is_default = 0 WHERE user_id = ?`, userId)
	if err != nil {
		return fmt.Errorf("error clearing default dashboard: %w", err)
	}

	_, err = tx.ExecContext(ctx, `UPDATE dashboards SET is_default = 1, updated_at = datetime('now') WHERE id = ? AND user_id = ?`, dashboardId, userId)
	if err != nil {
		return fmt.Errorf("error setting default dashboard: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("error committing set-default transaction: %w", err)
	}
	r.logger.Debug("set default dashboard", "user_id", userId, "dashboard_id", dashboardId)
	return nil
}

func (r *SqlDashboardRepository) GetDefaultForUser(ctx context.Context, userId int) (*gen.Dashboard, error) {
	query := `SELECT id, user_id, name, config, shared, is_default, created_at, updated_at FROM dashboards WHERE user_id = ? AND is_default = 1`
	row := r.db.Reader.QueryRowContext(ctx, query, userId)
	d := &gen.Dashboard{}
	var createdAt, updatedAt SQLiteTime
	err := row.Scan(&d.Id, &d.UserId, &d.Name, &d.Config, &d.Shared, &d.IsDefault, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error querying default dashboard: %w", err)
	}
	d.CreatedAt = createdAt.Time
	d.UpdatedAt = updatedAt.Time
	return d, nil
}

func (r *SqlDashboardRepository) CreateCopy(ctx context.Context, sourceDashboardId int, targetUserId int) (int, error) {
	source, err := r.GetById(ctx, sourceDashboardId)
	if err != nil {
		return 0, fmt.Errorf("error fetching source dashboard: %w", err)
	}
	if source == nil {
		return 0, fmt.Errorf("source dashboard not found")
	}

	copy := &gen.Dashboard{
		UserId: targetUserId,
		Name:   source.Name + " (shared)",
		Config: source.Config,
		Shared: false,
	}
	return r.Create(ctx, copy)
}
