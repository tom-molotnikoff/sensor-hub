package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	gen "example/sensorHub/gen"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

type SensorRepository struct {
	db       *Handles
	logger   *slog.Logger
	nameMu   sync.RWMutex
	nameToID map[string]int
}

func NewSensorRepository(db *Handles, logger *slog.Logger) *SensorRepository {
	return &SensorRepository{
		db:       db,
		logger:   logger.With("component", "sensor_repository"),
		nameToID: make(map[string]int),
	}
}

func (s *SensorRepository) forgetNames() {
	s.nameMu.Lock()
	clear(s.nameToID)
	s.nameMu.Unlock()
}

func (s *SensorRepository) SensorExists(ctx context.Context, name string) (bool, error) {
	query := "SELECT COUNT(1) FROM sensors WHERE LOWER(name) = LOWER(?)"
	var count int
	err := s.db.Reader.QueryRowContext(ctx, query, name).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("error checking if sensor exists: %w", err)
	}
	return count > 0, nil
}

func (s *SensorRepository) SetEnabledSensorByName(ctx context.Context, name string, enabled bool) error {
	query := "UPDATE sensors SET enabled = ?, health_status = ? WHERE LOWER(name) = LOWER(?)"
	if !enabled {
		tx, err := s.db.Writer.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("error beginning transaction for sensor enable update: %w", err)
		}

		var sensorId int
		var currentStatus sql.NullString
		err = tx.QueryRowContext(ctx, "SELECT id, health_status FROM sensors WHERE LOWER(name) = LOWER(?)", name).Scan(&sensorId, &currentStatus)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			tx.Rollback()
			return fmt.Errorf("error fetching current sensor health for disable: %w", err)
		}

		query = "UPDATE sensors SET enabled = ?, health_status = ?, health_reason = 'unknown' WHERE LOWER(name) = LOWER(?)"
		result, err := tx.ExecContext(ctx, query, enabled, gen.Unknown, name)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("error updating sensor enabled status: %w", err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("error fetching rows affected after update: %w", err)
		}
		if rowsAffected == 0 {
			tx.Rollback()
			return fmt.Errorf("no changes were made to sensor %s", name)
		}

		if sensorId > 0 && currentStatus.Valid && gen.SensorHealthStatus(currentStatus.String) != gen.Unknown {
			insertQuery := fmt.Sprintf("INSERT INTO %s (sensor_id, health_status) VALUES (?, ?)", TableSensorHealthHistory)
			if _, err := tx.ExecContext(ctx, insertQuery, sensorId, gen.Unknown); err != nil {
				tx.Rollback()
				return fmt.Errorf("error inserting sensor health history: %w", err)
			}
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("error committing sensor enable update: %w", err)
		}
		return nil
	}
	result, err := s.db.Writer.ExecContext(ctx, query, enabled, gen.Unknown, name)
	if err != nil {
		return fmt.Errorf("error updating sensor enabled status: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error fetching rows affected after update: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no changes were made to sensor %s", name)
	}
	return nil
}

func (s *SensorRepository) GetSensorIdByName(ctx context.Context, sensorName string) (int, error) {
	key := strings.ToLower(sensorName)

	s.nameMu.RLock()
	sensorID, cached := s.nameToID[key]
	s.nameMu.RUnlock()
	if cached {
		return sensorID, nil
	}

	query := "SELECT id FROM sensors WHERE LOWER(name) = ?"
	if err := s.db.Reader.QueryRowContext(ctx, query, key).Scan(&sensorID); err != nil {
		return 0, fmt.Errorf("could not find sensor id for name %s: %w", sensorName, err)
	}

	s.nameMu.Lock()
	s.nameToID[key] = sensorID
	s.nameMu.Unlock()
	return sensorID, nil
}

func (s *SensorRepository) DeleteHealthHistoryOlderThan(ctx context.Context, cutoffDate time.Time) error {
	tx, err := s.db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error beginning transaction for health history cleanup: %w", err)
	}

	cutoff := cutoffDate.UTC().Format("2006-01-02 15:04:05")

	insertCheckpointQuery := fmt.Sprintf(`
		INSERT INTO %s (sensor_id, health_status, recorded_at)
		SELECT sensor_id, health_status, ?
		FROM (
			SELECT sensor_id, health_status,
				ROW_NUMBER() OVER (PARTITION BY sensor_id ORDER BY datetime(recorded_at) DESC, id DESC) AS row_num
			FROM %s
			WHERE datetime(recorded_at) < datetime(?)
		)
		WHERE row_num = 1
	`, TableSensorHealthHistory, TableSensorHealthHistory)
	if _, err := tx.ExecContext(ctx, insertCheckpointQuery, cutoff, cutoff); err != nil {
		tx.Rollback()
		return fmt.Errorf("error inserting retained health history checkpoints: %w", err)
	}

	insertCurrentStateQuery := fmt.Sprintf(`
		INSERT INTO %s (sensor_id, health_status, recorded_at)
		SELECT s.id, s.health_status, ?
		FROM sensors s
		WHERE s.health_status IS NOT NULL
		  AND NOT EXISTS (
			SELECT 1
			FROM %s h
			WHERE h.sensor_id = s.id
			  AND datetime(h.recorded_at) >= datetime(?)
		  )
	`, TableSensorHealthHistory, TableSensorHealthHistory)
	if _, err := tx.ExecContext(ctx, insertCurrentStateQuery, cutoff, cutoff); err != nil {
		tx.Rollback()
		return fmt.Errorf("error backfilling retained health history checkpoints: %w", err)
	}

	deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE datetime(recorded_at) < datetime(?)", TableSensorHealthHistory)
	if _, err := tx.ExecContext(ctx, deleteQuery, cutoff); err != nil {
		tx.Rollback()
		return fmt.Errorf("error deleting old sensor health history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing health history cleanup: %w", err)
	}

	return nil
}

func (s *SensorRepository) GetSensorHealthHistoryById(ctx context.Context, sensorId int, since time.Time) ([]gen.SensorHealthHistory, error) {
	query := fmt.Sprintf(`
		WITH latest_before AS (
			SELECT id, sensor_id, health_status, recorded_at
			FROM %s
			WHERE sensor_id = ? AND datetime(recorded_at) < datetime(?)
			ORDER BY datetime(recorded_at) DESC, id DESC
			LIMIT 1
		),
		within_window AS (
			SELECT id, sensor_id, health_status, recorded_at
			FROM %s
			WHERE sensor_id = ? AND datetime(recorded_at) >= datetime(?)
		)
		SELECT id, sensor_id, health_status, recorded_at
		FROM (
			SELECT id, sensor_id, health_status, recorded_at FROM within_window
			UNION ALL
			SELECT id, sensor_id, health_status, recorded_at FROM latest_before
		)
		ORDER BY datetime(recorded_at) DESC, id DESC
	`, TableSensorHealthHistory, TableSensorHealthHistory)
	formattedSince := since.UTC().Format("2006-01-02 15:04:05")
	rows, err := s.db.Reader.QueryContext(ctx, query, sensorId, formattedSince, sensorId, formattedSince)
	if err != nil {
		return nil, fmt.Errorf("error querying sensor health history: %w", err)
	}
	defer rows.Close()

	history := make([]gen.SensorHealthHistory, 0)
	for rows.Next() {
		var record gen.SensorHealthHistory
		var recordedAt SQLiteTime
		if err := rows.Scan(&record.Id, &record.SensorId, &record.HealthStatus, &recordedAt); err != nil {
			return nil, fmt.Errorf("error scanning sensor health history row: %w", err)
		}
		record.RecordedAt = recordedAt.Time
		history = append(history, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over sensor health history rows: %w", err)
	}
	return history, nil
}

func (s *SensorRepository) DeleteSensorByName(ctx context.Context, name string) error {
	sensorId, err := s.GetSensorIdByName(ctx, name)
	if err != nil {
		return fmt.Errorf("error retrieving sensor ID for deletion: %w", err)
	}

	/*
	 TODO: transaction is good but purge should be its own service
	 so as to not hold up the API call whilst potentially deleting
	 a lot of data. eg: schedule a purge and let the other service
	 handle it asynchronously.
	*/

	txn, err := s.db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error beginning transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			txn.Rollback()
			panic(p)
		} else if err != nil {
			txn.Rollback()
		} else {
			err = txn.Commit()
		}
		s.forgetNames()
	}()

	purgeQuery := fmt.Sprintf("DELETE FROM %s WHERE sensor_id = ?", TableReadings)
	_, err = txn.Exec(purgeQuery, sensorId)
	if err != nil {
		return fmt.Errorf("error purging readings for sensor ID %d: %w", sensorId, err)
	}
	sensorMtPurgeQuery := fmt.Sprintf("DELETE FROM %s WHERE sensor_id = ?", TableSensorMeasurementTypes)
	_, err = txn.Exec(sensorMtPurgeQuery, sensorId)
	if err != nil {
		return fmt.Errorf("error purging sensor measurement types for sensor ID %d: %w", sensorId, err)
	}
	healthHistoryPurgeQuery := fmt.Sprintf("DELETE FROM %s WHERE sensor_id = ?", TableSensorHealthHistory)
	_, err = txn.Exec(healthHistoryPurgeQuery, sensorId)
	if err != nil {
		return fmt.Errorf("error purging sensor health history for sensor ID %d: %w", sensorId, err)
	}
	commandHistoryPurgeQuery := fmt.Sprintf("DELETE FROM %s WHERE sensor_id = ?", TableSensorCommandHistory)
	_, err = txn.Exec(commandHistoryPurgeQuery, sensorId)
	if err != nil {
		return fmt.Errorf("error purging sensor command history for sensor ID %d: %w", sensorId, err)
	}

	query := "DELETE FROM sensors WHERE LOWER(name) = LOWER(?)"
	result, err := txn.Exec(query, name)
	if err != nil {
		return fmt.Errorf("error deleting sensor: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error fetching rows affected after delete: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no sensor found with name %s to delete", name)
	}

	return nil
}

func (s *SensorRepository) GetSensorsByDriver(ctx context.Context, sensorDriver string) ([]gen.Sensor, error) {
	query := "SELECT id, name, external_id, sensor_driver, config, health_status, health_reason, enabled, status, retention_hours, metadata FROM sensors WHERE LOWER(sensor_driver) = LOWER(?)"
	rows, err := s.db.Reader.QueryContext(ctx, query, sensorDriver)
	if err != nil {
		return nil, fmt.Errorf("error querying sensors by driver: %w", err)
	}
	defer rows.Close()

	var sensors []gen.Sensor
	for rows.Next() {
		sensor, err := scanSensorRow(rows)
		if err != nil {
			return nil, fmt.Errorf("error scanning sensor row: %w", err)
		}
		sensors = append(sensors, sensor)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over sensor rows: %w", err)
	}
	return sensors, nil
}

func (s *SensorRepository) UpdateSensorById(ctx context.Context, sensor gen.Sensor, retentionHoursPresent bool) error {
	configJSON, err := json.Marshal(sensor.Config)
	if err != nil {
		return fmt.Errorf("error marshalling sensor config: %w", err)
	}
	metadataJSON, err := json.Marshal(sensorMetadataValue(sensor.Metadata))
	if err != nil {
		return fmt.Errorf("error marshalling sensor metadata: %w", err)
	}

	var result sql.Result
	if retentionHoursPresent {
		// retention_hours was explicitly provided (even if null — meaning "clear it").
		query := "UPDATE sensors SET name = ?, sensor_driver = ?, config = ?, metadata = ?, retention_hours = ? WHERE id = ?"
		result, err = s.db.Writer.ExecContext(ctx, query, sensor.Name, sensor.SensorDriver, string(configJSON), string(metadataJSON), sensor.RetentionHours, sensor.Id)
	} else {
		query := "UPDATE sensors SET name = ?, sensor_driver = ?, config = ?, metadata = ? WHERE id = ?"
		result, err = s.db.Writer.ExecContext(ctx, query, sensor.Name, sensor.SensorDriver, string(configJSON), string(metadataJSON), sensor.Id)
	}
	if err != nil {
		return fmt.Errorf("error updating sensor: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error fetching rows affected after update: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no changes were made to sensor %s", sensor.Name)
	}
	s.forgetNames()
	return nil
}

func (s *SensorRepository) AddSensor(ctx context.Context, sensor gen.Sensor) error {
	if sensor.Name == "" || sensor.SensorDriver == "" {
		return fmt.Errorf("sensor name and sensor driver cannot be empty")
	}
	if sensor.Config == nil {
		sensor.Config = make(map[string]string)
	}

	configJSON, err := json.Marshal(sensor.Config)
	if err != nil {
		return fmt.Errorf("error marshalling sensor config: %w", err)
	}
	metadataJSON, err := json.Marshal(sensorMetadataValue(sensor.Metadata))
	if err != nil {
		return fmt.Errorf("error marshalling sensor metadata: %w", err)
	}

	query := "INSERT INTO sensors (name, external_id, sensor_driver, config, metadata, health_reason, enabled, status) VALUES (?, ?, ?, ?, ?, 'unknown', ?, ?)"
	status := sensor.Status
	if status == "" {
		status = gen.SensorStatusActive
	}
	_, err = s.db.Writer.ExecContext(ctx, query, sensor.Name, sensor.ExternalId, sensor.SensorDriver, string(configJSON), string(metadataJSON), true, status)
	if err != nil {
		return fmt.Errorf("error adding new sensor: %w", err)
	}
	s.forgetNames()
	return nil
}

func (s *SensorRepository) GetSensorById(ctx context.Context, id int) (*gen.Sensor, error) {
	query := "SELECT id, name, external_id, sensor_driver, config, health_status, health_reason, enabled, status, retention_hours, metadata FROM sensors WHERE id = ?"
	sensor, err := scanSensorRow(s.db.Reader.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no sensor found with id %d", id)
		}
		return nil, fmt.Errorf("error querying sensor by id: %w", err)
	}
	return &sensor, nil
}

func (s *SensorRepository) GetSensorByName(ctx context.Context, name string) (*gen.Sensor, error) {
	query := "SELECT id, name, external_id, sensor_driver, config, health_status, health_reason, enabled, status, retention_hours, metadata FROM sensors WHERE LOWER(name) = LOWER(?)"
	sensor, err := scanSensorRow(s.db.Reader.QueryRowContext(ctx, query, name))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no sensor found with name %s", name)
		}
		return nil, fmt.Errorf("error querying sensor by name: %w", err)
	}
	return &sensor, nil
}

func (s *SensorRepository) GetSensorByExternalId(ctx context.Context, externalId string) (*gen.Sensor, error) {
	query := "SELECT id, name, external_id, sensor_driver, config, health_status, health_reason, enabled, status, retention_hours, metadata FROM sensors WHERE LOWER(external_id) = LOWER(?)"
	sensor, err := scanSensorRow(s.db.Reader.QueryRowContext(ctx, query, externalId))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no sensor found with external_id %s: %w", externalId, err)
		}
		return nil, fmt.Errorf("error querying sensor by external_id: %w", err)
	}
	return &sensor, nil
}

func (s *SensorRepository) SensorExistsByExternalId(ctx context.Context, externalId string) (bool, error) {
	query := "SELECT COUNT(1) FROM sensors WHERE LOWER(external_id) = LOWER(?)"
	var count int
	err := s.db.Reader.QueryRowContext(ctx, query, externalId).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("error checking if sensor exists by external_id: %w", err)
	}
	return count > 0, nil
}

func (s *SensorRepository) GetAllSensors(ctx context.Context) ([]gen.Sensor, error) {
	query := "SELECT id, name, external_id, sensor_driver, config, health_status, health_reason, enabled, status, retention_hours, metadata FROM sensors"
	rows, err := s.db.Reader.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying all sensors: %w", err)
	}
	defer rows.Close()

	var sensors []gen.Sensor
	for rows.Next() {
		sensor, err := scanSensorRow(rows)
		if err != nil {
			return nil, fmt.Errorf("error scanning sensor row: %w", err)
		}
		sensors = append(sensors, sensor)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over sensor rows: %w", err)
	}
	return sensors, nil
}

func (s *SensorRepository) UpdateSensorHealthById(ctx context.Context, sensorId int, healthStatus gen.SensorHealthStatus, healthReason string) error {
	tx, err := s.db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error beginning transaction for sensor health update: %w", err)
	}

	if err := updateSensorHealthTx(ctx, tx, sensorId, healthStatus, healthReason); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing sensor health update: %w", err)
	}

	return nil
}

func updateSensorHealthTx(ctx context.Context, tx *sql.Tx, sensorId int, healthStatus gen.SensorHealthStatus, healthReason string) error {
	historyQuery := fmt.Sprintf(`INSERT INTO %s (sensor_id, health_status)
		SELECT id, ? FROM sensors
		WHERE id = ? AND health_status IS NOT NULL AND health_status != ?`, TableSensorHealthHistory)
	if _, err := tx.ExecContext(ctx, historyQuery, healthStatus, sensorId, healthStatus); err != nil {
		return fmt.Errorf("error inserting sensor health history: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "UPDATE sensors SET health_status = ?, health_reason = ? WHERE id = ?", healthStatus, healthReason, sensorId); err != nil {
		return fmt.Errorf("error updating sensor health status: %w", err)
	}
	return nil
}

// TODO - implement methods for getting sensor health over time for reporting - see V5__sensor_health_history.sql

// scannable is satisfied by both *sql.Row and *sql.Rows.
type scannable interface {
	Scan(dest ...any) error
}

// scanSensorRow scans a sensor row (columns: id, name, external_id, sensor_driver, config,
// health_status, health_reason, enabled, status, retention_hours, metadata) and unmarshals
// the JSON config/metadata columns into the Sensor maps.
func scanSensorRow(row scannable) (gen.Sensor, error) {
	var s gen.Sensor
	var configJSON string
	var metadataJSON string
	var externalId sql.NullString
	var retentionHours sql.NullInt64
	err := row.Scan(&s.Id, &s.Name, &externalId, &s.SensorDriver, &configJSON, &s.HealthStatus, &s.HealthReason, &s.Enabled, &s.Status, &retentionHours, &metadataJSON)
	if err != nil {
		return s, err
	}
	if externalId.Valid {
		s.ExternalId = &externalId.String
	}
	if retentionHours.Valid {
		v := int(retentionHours.Int64)
		s.RetentionHours = &v
	}
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &s.Config); err != nil {
			return s, fmt.Errorf("failed to unmarshal sensor config: %w", err)
		}
	}
	if s.Config == nil {
		s.Config = make(map[string]string)
	}
	metadata := make(map[string]interface{})
	if metadataJSON != "" {
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
			return s, fmt.Errorf("failed to unmarshal sensor metadata: %w", err)
		}
	}
	s.Metadata = &metadata
	return s, nil
}

func sensorMetadataValue(metadata *map[string]interface{}) map[string]interface{} {
	if metadata == nil {
		return map[string]interface{}{}
	}
	return *metadata
}

func (sr *SensorRepository) GetSensorsByStatus(ctx context.Context, status string) ([]gen.Sensor, error) {
	query := "SELECT id, name, external_id, sensor_driver, config, health_status, health_reason, enabled, status, retention_hours, metadata FROM sensors WHERE LOWER(status) = LOWER(?)"
	rows, err := sr.db.Reader.QueryContext(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("error querying sensors by status: %w", err)
	}
	defer rows.Close()

	var sensors []gen.Sensor
	for rows.Next() {
		sensor, err := scanSensorRow(rows)
		if err != nil {
			return nil, fmt.Errorf("error scanning sensor row: %w", err)
		}
		sensors = append(sensors, sensor)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over sensor rows: %w", err)
	}
	return sensors, nil
}

// GetSensorsWithRetention returns all sensors that have a custom retention_hours set.
func (sr *SensorRepository) GetSensorsWithRetention(ctx context.Context) ([]gen.Sensor, error) {
	query := "SELECT id, name, external_id, sensor_driver, config, health_status, health_reason, enabled, status, retention_hours, metadata FROM sensors WHERE retention_hours IS NOT NULL"
	rows, err := sr.db.Reader.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying sensors with custom retention: %w", err)
	}
	defer rows.Close()

	var sensors []gen.Sensor
	for rows.Next() {
		sensor, err := scanSensorRow(rows)
		if err != nil {
			return nil, fmt.Errorf("error scanning sensor row: %w", err)
		}
		sensors = append(sensors, sensor)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over sensor rows: %w", err)
	}
	return sensors, nil
}

func (sr *SensorRepository) UpdateSensorStatus(ctx context.Context, sensorId int, status string) error {
	query := "UPDATE sensors SET status = ?, enabled = CASE WHEN ? = 'active' THEN 1 ELSE enabled END WHERE id = ?"
	result, err := sr.db.Writer.ExecContext(ctx, query, status, status, sensorId)
	if err != nil {
		return fmt.Errorf("error updating sensor status: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error fetching rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("sensor with id %d not found", sensorId)
	}
	return nil
}
