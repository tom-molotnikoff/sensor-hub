package seed

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	database "example/sensorHub/db"

	_ "modernc.org/sqlite"
)

const Version = 1

const insertBatchRows = 500

type Shape struct {
	Sensors          int
	MeasurementTypes int
	Days             int
	Readings         int
}

var Default = Shape{Sensors: 8, MeasurementTypes: 9, Days: 90, Readings: 5_000_000}

var measurementTypeNames = []string{
	"temperature",
	"humidity",
	"pressure",
	"power",
	"battery",
	"voltage",
	"luminance",
	"link_quality",
	"energy",
}

var errNoMeasurementTypes = fmt.Errorf("shape requests more measurement types than the schema seeds (%d)", len(measurementTypeNames))

func Generate(ctx context.Context, dbPath string, shape Shape, logger *slog.Logger) error {
	if shape.Sensors < 1 || shape.Days < 1 || shape.Readings < 1 {
		return fmt.Errorf("shape must have at least one sensor, day and reading, got %+v", shape)
	}
	if shape.MeasurementTypes < 1 || shape.MeasurementTypes > len(measurementTypeNames) {
		return errNoMeasurementTypes
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("could not create seed directory: %w", err)
	}
	if err := removeDatabaseFiles(dbPath); err != nil {
		return err
	}

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(OFF)", dbPath))
	if err != nil {
		return fmt.Errorf("could not open seed database: %w", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()

	if err := database.RunMigrations(db, logger); err != nil {
		return fmt.Errorf("could not migrate seed database: %w", err)
	}

	started := time.Now()

	sensorIDs, err := insertSensors(ctx, db, shape.Sensors)
	if err != nil {
		return err
	}

	typeIDs, err := measurementTypeIDs(ctx, db, shape.MeasurementTypes)
	if err != nil {
		return err
	}

	if err := assignMeasurementTypes(ctx, db, sensorIDs, typeIDs); err != nil {
		return err
	}

	if err := insertReadings(ctx, db, shape, sensorIDs, typeIDs, logger); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", Version)); err != nil {
		return fmt.Errorf("could not stamp seed version: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("could not checkpoint seed database: %w", err)
	}

	logger.Info("seed database written",
		"path", dbPath,
		"version", Version,
		"readings", shape.Readings,
		"duration", time.Since(started).Round(time.Second))
	return nil
}

func StoredVersion(dbPath string) (int, error) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=query_only(1)", dbPath))
	if err != nil {
		return 0, fmt.Errorf("could not open seed database: %w", err)
	}
	defer db.Close()

	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("could not read seed version: %w", err)
	}
	return version, nil
}

func IsCurrent(dbPath string) bool {
	if _, err := os.Stat(dbPath); err != nil {
		return false
	}
	version, err := StoredVersion(dbPath)
	return err == nil && version == Version
}

func removeDatabaseFiles(dbPath string) error {
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(dbPath + suffix); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("could not remove existing seed file %s: %w", dbPath+suffix, err)
		}
	}
	return nil
}

func insertSensors(ctx context.Context, db *sql.DB, count int) ([]int64, error) {
	ids := make([]int64, 0, count)
	for i := range count {
		result, err := db.ExecContext(ctx,
			"INSERT INTO sensors (name, sensor_driver, config, health_status, health_reason, enabled) VALUES (?, 'sensor-hub-http-temperature', '{}', 'good', 'seeded', 1)",
			fmt.Sprintf("seed-sensor-%02d", i+1))
		if err != nil {
			return nil, fmt.Errorf("could not insert seed sensor: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("could not read seed sensor id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func measurementTypeIDs(ctx context.Context, db *sql.DB, count int) ([]int64, error) {
	ids := make([]int64, 0, count)
	for _, name := range measurementTypeNames[:count] {
		var id int64
		if err := db.QueryRowContext(ctx, "SELECT id FROM measurement_types WHERE name = ?", name).Scan(&id); err != nil {
			return nil, fmt.Errorf("could not resolve measurement type %q: %w", name, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func assignMeasurementTypes(ctx context.Context, db *sql.DB, sensorIDs, typeIDs []int64) error {
	for _, sensorID := range sensorIDs {
		for _, typeID := range typeIDs {
			if _, err := db.ExecContext(ctx,
				"INSERT OR IGNORE INTO sensor_measurement_types (sensor_id, measurement_type_id, unit) SELECT ?, id, default_unit FROM measurement_types WHERE id = ?",
				sensorID, typeID); err != nil {
				return fmt.Errorf("could not assign measurement type to seed sensor: %w", err)
			}
		}
	}
	return nil
}

func insertReadings(ctx context.Context, db *sql.DB, shape Shape, sensorIDs, typeIDs []int64, logger *slog.Logger) error {
	series := len(sensorIDs) * len(typeIDs)
	perSeries := shape.Readings / series
	remainder := shape.Readings % series

	window := time.Duration(shape.Days) * 24 * time.Hour
	end := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	start := end.Add(-window)

	statement := insertStatement(insertBatchRows)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not begin seed transaction: %w", err)
	}
	defer func() { tx.Rollback() }()

	batch := make([]any, 0, insertBatchRows*4)
	written := 0

	flush := func(rows int) error {
		if rows == 0 {
			return nil
		}
		query := statement
		if rows != insertBatchRows {
			query = insertStatement(rows)
		}
		if _, err := tx.ExecContext(ctx, query, batch...); err != nil {
			return fmt.Errorf("could not insert seed readings: %w", err)
		}
		batch = batch[:0]
		return nil
	}

	for sensorIndex, sensorID := range sensorIDs {
		for typeIndex, typeID := range typeIDs {
			rows := perSeries
			if seriesIndex := sensorIndex*len(typeIDs) + typeIndex; seriesIndex < remainder {
				rows++
			}
			if rows == 0 {
				continue
			}

			step := window
			if rows > 1 {
				step = window / time.Duration(rows-1)
			}
			for row := range rows {
				at := start.Add(time.Duration(row) * step)
				batch = append(batch, sensorID, typeID, seriesValue(typeIndex, row), at.Format("2006-01-02 15:04:05"))
				written++

				if len(batch) == insertBatchRows*4 {
					if err := flush(insertBatchRows); err != nil {
						return err
					}
					if written%(insertBatchRows*2000) == 0 {
						if err := tx.Commit(); err != nil {
							return fmt.Errorf("could not commit seed batch: %w", err)
						}
						logger.Info("seeding readings", "written", written, "of", shape.Readings)
						if tx, err = db.BeginTx(ctx, nil); err != nil {
							return fmt.Errorf("could not begin seed transaction: %w", err)
						}
					}
				}
			}
		}
	}

	if err := flush(len(batch) / 4); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("could not commit seed readings: %w", err)
	}
	return nil
}

func insertStatement(rows int) string {
	var b strings.Builder
	b.WriteString("INSERT INTO readings (sensor_id, measurement_type_id, numeric_value, time) VALUES ")
	for i := range rows {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("(?, ?, ?, ?)")
	}
	return b.String()
}

func seriesValue(typeIndex, row int) float64 {
	base := 10.0 + float64(typeIndex)*10.0
	return math.Round((base+5.0*math.Sin(float64(row)/64.0))*100) / 100
}
