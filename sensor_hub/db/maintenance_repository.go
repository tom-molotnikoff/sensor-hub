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

func (r *maintenanceRepository) ReclaimFreePages(ctx context.Context, chunkPages int) (int64, error) {
	if chunkPages <= 0 {
		return 0, fmt.Errorf("chunk size must be positive, got %d", chunkPages)
	}

	before, err := r.writerFreelistCount(ctx)
	if err != nil {
		return 0, err
	}

	if _, err := r.db.Writer.ExecContext(ctx, fmt.Sprintf("PRAGMA incremental_vacuum(%d)", chunkPages)); err != nil {
		return 0, fmt.Errorf("failed to reclaim free pages: %w", err)
	}

	after, err := r.writerFreelistCount(ctx)
	if err != nil {
		return 0, err
	}

	return before - after, nil
}

func (r *maintenanceRepository) Checkpoint(ctx context.Context) (*CheckpointResult, error) {
	var result CheckpointResult
	row := r.db.Writer.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	if err := row.Scan(&result.Busy, &result.LogPages, &result.CheckpointedPages); err != nil {
		return nil, fmt.Errorf("failed to checkpoint WAL: %w", err)
	}
	return &result, nil
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

func (r *maintenanceRepository) writerFreelistCount(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.Writer.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to get freelist_count: %w", err)
	}
	return count, nil
}
