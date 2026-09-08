package database

import "context"

// MaintenanceRepository provides database maintenance operations for SQLite.
type MaintenanceRepository interface {
	ReclaimFreePages(ctx context.Context, chunkPages int) (int64, error)
	Checkpoint(ctx context.Context) (*CheckpointResult, error)
	Optimise(ctx context.Context) error
	DatabaseStats(ctx context.Context) (*DatabaseStatsResult, error)
}

// DatabaseStatsResult holds page-level statistics from SQLite PRAGMAs.
type DatabaseStatsResult struct {
	PageCount     int64
	FreelistCount int64
	PageSize      int64
}

type CheckpointResult struct {
	Busy              int64
	LogPages          int64
	CheckpointedPages int64
}

// SizeBytes returns the total database size in bytes.
func (s *DatabaseStatsResult) SizeBytes() int64 {
	return s.PageCount * s.PageSize
}

// FreelistBytes returns the reclaimable space in bytes.
func (s *DatabaseStatsResult) FreelistBytes() int64 {
	return s.FreelistCount * s.PageSize
}

// FreelistRatio returns the proportion of free pages (0.0–1.0).
func (s *DatabaseStatsResult) FreelistRatio() float64 {
	if s.PageCount == 0 {
		return 0
	}
	return float64(s.FreelistCount) / float64(s.PageCount)
}
