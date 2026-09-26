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
	for _, table := range []string{
		"devseed_metadata (name TEXT PRIMARY KEY, value TEXT NOT NULL)",
		"devseed_sensors (device TEXT PRIMARY KEY, sensor_id INTEGER NOT NULL)",
	} {
		if _, err := db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS "+table); err != nil {
			return marker{}, fmt.Errorf("failed to create %s: %w", table, err)
		}
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

func recordSeededSensor(ctx context.Context, db *sql.DB, device string, sensorID int) error {
	if _, err := db.ExecContext(ctx,
		"INSERT OR REPLACE INTO devseed_sensors (device, sensor_id) VALUES (?, ?)", device, sensorID); err != nil {
		return fmt.Errorf("failed to record the id of seeded sensor %s: %w", device, err)
	}
	return nil
}

func seededSensorIDs(ctx context.Context, db *sql.DB) (map[string]int, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT d.device, d.sensor_id FROM devseed_sensors d JOIN sensors s ON s.id = d.sensor_id")
	if err != nil {
		return nil, fmt.Errorf("failed to read the seeded sensors: %w", err)
	}
	defer rows.Close()
	ids := make(map[string]int)
	for rows.Next() {
		var device string
		var id int
		if err := rows.Scan(&device, &id); err != nil {
			return nil, fmt.Errorf("failed to read the seeded sensors: %w", err)
		}
		ids[device] = id
	}
	return ids, rows.Err()
}
