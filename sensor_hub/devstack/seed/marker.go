package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	markerSeededAt    = "entities_seeded_at"
	markerAdminAPIKey = "admin_api_key"
)

type marker struct {
	seeded      bool
	adminAPIKey string
}

func loadMarker(ctx context.Context, db *sql.DB) (marker, error) {
	if _, err := db.ExecContext(ctx,
		"CREATE TABLE IF NOT EXISTS devseed_metadata (name TEXT PRIMARY KEY, value TEXT NOT NULL)"); err != nil {
		return marker{}, fmt.Errorf("failed to create devseed_metadata: %w", err)
	}
	rows, err := db.QueryContext(ctx, "SELECT name, value FROM devseed_metadata")
	if err != nil {
		return marker{}, fmt.Errorf("failed to read devseed_metadata: %w", err)
	}
	defer rows.Close()

	var loaded marker
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return marker{}, fmt.Errorf("failed to read devseed_metadata: %w", err)
		}
		switch name {
		case markerSeededAt:
			loaded.seeded = true
		case markerAdminAPIKey:
			loaded.adminAPIKey = value
		}
	}
	if err := rows.Err(); err != nil {
		return marker{}, fmt.Errorf("failed to read devseed_metadata: %w", err)
	}
	return loaded, nil
}

func writeMarker(ctx context.Context, db *sql.DB, adminAPIKey string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for name, value := range map[string]string{
		markerAdminAPIKey: adminAPIKey,
		markerSeededAt:    time.Now().UTC().Format(time.RFC3339),
	} {
		if _, err := tx.ExecContext(ctx, "INSERT INTO devseed_metadata (name, value) VALUES (?, ?)", name, value); err != nil {
			return fmt.Errorf("failed to write %s: %w", name, err)
		}
	}
	return tx.Commit()
}
