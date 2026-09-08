package database

import (
	"context"
	"fmt"
)

type maintenanceRepository struct {
	db *Handles
}

func NewMaintenanceRepository(db *Handles) MaintenanceRepository {
	return &maintenanceRepository{db: db}
}

func (r *maintenanceRepository) Vacuum(ctx context.Context) error {
	_, err := r.db.Writer.ExecContext(ctx, "VACUUM")
	if err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}
	return nil
}

func (r *maintenanceRepository) Optimise(ctx context.Context) error {
	_, err := r.db.Writer.ExecContext(ctx, "PRAGMA optimize")
	if err != nil {
		return fmt.Errorf("failed to optimise database: %w", err)
	}
	return nil
}

func (r *maintenanceRepository) DatabaseStats(ctx context.Context) (*DatabaseStatsResult, error) {
	var stats DatabaseStatsResult

	if err := r.db.Reader.QueryRowContext(ctx, "PRAGMA page_count").Scan(&stats.PageCount); err != nil {
		return nil, fmt.Errorf("failed to get page_count: %w", err)
	}
	if err := r.db.Reader.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&stats.FreelistCount); err != nil {
		return nil, fmt.Errorf("failed to get freelist_count: %w", err)
	}
	if err := r.db.Reader.QueryRowContext(ctx, "PRAGMA page_size").Scan(&stats.PageSize); err != nil {
		return nil, fmt.Errorf("failed to get page_size: %w", err)
	}

	return &stats, nil
}
