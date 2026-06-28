package database

import (
	"context"
	"database/sql"
	"errors"
	gen "example/sensorHub/gen"
	"example/sensorHub/utils"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type ReadingsRepositoryImpl struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewReadingsRepository(db *sql.DB, logger *slog.Logger) ReadingsRepository {
	return &ReadingsRepositoryImpl{db: db, logger: logger.With("component", "readings_repository")}
}

func (r *ReadingsRepositoryImpl) Add(ctx context.Context, readings []gen.Reading) error {
	var stored int
	for _, reading := range readings {
		mtID, err := r.resolveMeasurementTypeID(ctx, reading.MeasurementType)
		if err != nil {
			r.logger.Warn("skipping reading with unknown measurement type",
				"sensor", reading.SensorName, "type", reading.MeasurementType)
			continue
		}

		sensorID, err := r.resolveSensorID(ctx, reading.SensorName)
		if err != nil {
			return fmt.Errorf("issue finding sensor id: %w", err)
		}

		query := fmt.Sprintf("INSERT INTO %s (sensor_id, measurement_type_id, numeric_value, text_state, time) VALUES (?, ?, ?, ?, ?)", TableReadings)
		_, err = r.db.ExecContext(ctx, query, sensorID, mtID, reading.NumericValue, reading.TextState, reading.Time)
		if err != nil {
			return fmt.Errorf("issue persisting reading to database: %w", err)
		}
		stored++
		r.logger.Debug("saved reading to database", "sensor", reading.SensorName, "type", reading.MeasurementType)
	}
	if stored == 0 && len(readings) > 0 {
		return fmt.Errorf("no readings stored: all %d readings had unrecognised measurement types", len(readings))
	}
	return nil
}

func (r *ReadingsRepositoryImpl) GetBetweenDates(ctx context.Context, startDate, endDate, sensorName, measurementType string, interval AggregationInterval, aggFunc AggregationFunction) ([]gen.Reading, error) {
	if interval == AggregationRaw || interval == "" {
		return r.getRawBetweenDates(ctx, startDate, endDate, sensorName, measurementType)
	}
	return r.getAggregatedBetweenDates(ctx, startDate, endDate, sensorName, measurementType, interval, aggFunc)
}

// seriesFilter resolves the optional sensor/measurement names to ids and returns an
// id-based WHERE fragment (plus its bind args). Filtering on r.sensor_id /
// r.measurement_type_id lets the (sensor_id, measurement_type_id, time DESC) composite
// index serve the query, instead of a full-table scan filtered by joined names.
// resolved is false when a provided name does not exist, signalling the caller to return
// an empty result (matching the prior LOWER(name) behaviour). Resolution is
// case-insensitive, as before.
func (r *ReadingsRepositoryImpl) seriesFilter(ctx context.Context, sensorName, measurementType string) (clause string, args []any, resolved bool, err error) {
	if sensorName != "" {
		id, e := r.resolveSensorID(ctx, sensorName)
		if errors.Is(e, sql.ErrNoRows) {
			return "", nil, false, nil
		}
		if e != nil {
			return "", nil, false, e
		}
		clause += " AND r.sensor_id = ?"
		args = append(args, id)
	}
	if measurementType != "" {
		id, e := r.resolveMeasurementTypeID(ctx, measurementType)
		if errors.Is(e, sql.ErrNoRows) {
			return "", nil, false, nil
		}
		if e != nil {
			return "", nil, false, e
		}
		clause += " AND r.measurement_type_id = ?"
		args = append(args, id)
	}
	return clause, args, true, nil
}

func rawBetweenQuery(seriesClause string) string {
	return fmt.Sprintf(`
		SELECT r.id, s.name, mt.name, r.numeric_value, r.text_state, COALESCE(smt.unit, mt.default_unit), r.time
		FROM %s r
		JOIN sensors s ON r.sensor_id = s.id
		JOIN %s mt ON r.measurement_type_id = mt.id
		LEFT JOIN %s smt ON smt.sensor_id = s.id AND smt.measurement_type_id = mt.id
		WHERE r.time BETWEEN ? AND ?%s
		ORDER BY r.time ASC
	`, TableReadings, TableMeasurementTypes, TableSensorMeasurementTypes, seriesClause)
}

func (r *ReadingsRepositoryImpl) getRawBetweenDates(ctx context.Context, startDate, endDate, sensorName, measurementType string) ([]gen.Reading, error) {
	clause, filterArgs, resolved, err := r.seriesFilter(ctx, sensorName, measurementType)
	if err != nil {
		return nil, fmt.Errorf("error resolving readings filter: %w", err)
	}
	if !resolved {
		return nil, nil
	}

	args := append([]any{startDate, endDate}, filterArgs...)
	rows, err := r.db.QueryContext(ctx, rawBetweenQuery(clause), args...)
	if err != nil {
		return nil, fmt.Errorf("error fetching readings between %s and %s: %w", startDate, endDate, err)
	}
	defer func() { _ = rows.Close() }()

	return scanReadings(rows)
}

func (r *ReadingsRepositoryImpl) getAggregatedBetweenDates(ctx context.Context, startDate, endDate, sensorName, measurementType string, interval AggregationInterval, aggFunc AggregationFunction) ([]gen.Reading, error) {
	bucket, err := timeBucketExpression(interval)
	if err != nil {
		return nil, err
	}

	if aggFunc == AggregationFunctionLast {
		return r.getLastBetweenDates(ctx, startDate, endDate, sensorName, measurementType, bucket)
	}

	sqlAgg := "ROUND(AVG(r.numeric_value), 2)"
	if aggFunc == AggregationFunctionCount {
		sqlAgg = "COUNT(*)"
	}

	clause, filterArgs, resolved, err := r.seriesFilter(ctx, sensorName, measurementType)
	if err != nil {
		return nil, fmt.Errorf("error resolving readings filter: %w", err)
	}
	if !resolved {
		return nil, nil
	}

	args := append([]any{startDate, endDate}, filterArgs...)
	rows, err := r.db.QueryContext(ctx, aggregatedBetweenQuery(sqlAgg, bucket, clause), args...)
	if err != nil {
		return nil, fmt.Errorf("error fetching aggregated readings between %s and %s: %w", startDate, endDate, err)
	}
	defer func() { _ = rows.Close() }()

	return scanReadings(rows)
}

func aggregatedBetweenQuery(sqlAgg, bucket, seriesClause string) string {
	return fmt.Sprintf(`
		SELECT 0 AS id, s.name, mt.name, %s, NULL, COALESCE(smt.unit, mt.default_unit), %s AS bucket_time
		FROM %s r
		JOIN sensors s ON r.sensor_id = s.id
		JOIN %s mt ON r.measurement_type_id = mt.id
		LEFT JOIN %s smt ON smt.sensor_id = s.id AND smt.measurement_type_id = mt.id
		WHERE r.time BETWEEN ? AND ?%s
		GROUP BY s.name, mt.name, bucket_time ORDER BY bucket_time ASC
	`, sqlAgg, bucket, TableReadings, TableMeasurementTypes, TableSensorMeasurementTypes, seriesClause)
}

func lastBetweenQuery(bucket, seriesClause string) string {
	return fmt.Sprintf(`
		SELECT sub.id, sub.sensor_name, sub.measurement_type, sub.numeric_value, sub.text_state, sub.unit, sub.bucket_time
		FROM (
			SELECT r.id, s.name AS sensor_name, mt.name AS measurement_type,
				r.numeric_value, r.text_state, COALESCE(smt.unit, mt.default_unit) AS unit,
				%s AS bucket_time,
				ROW_NUMBER() OVER (PARTITION BY r.sensor_id, r.measurement_type_id, %s ORDER BY r.time DESC) AS rn
			FROM %s r
			JOIN sensors s ON r.sensor_id = s.id
			JOIN %s mt ON r.measurement_type_id = mt.id
			LEFT JOIN %s smt ON smt.sensor_id = s.id AND smt.measurement_type_id = mt.id
			WHERE r.time BETWEEN ? AND ?%s
		) sub WHERE sub.rn = 1 ORDER BY sub.bucket_time ASC
	`, bucket, bucket, TableReadings, TableMeasurementTypes, TableSensorMeasurementTypes, seriesClause)
}

func (r *ReadingsRepositoryImpl) getLastBetweenDates(ctx context.Context, startDate, endDate, sensorName, measurementType, bucket string) ([]gen.Reading, error) {
	clause, filterArgs, resolved, err := r.seriesFilter(ctx, sensorName, measurementType)
	if err != nil {
		return nil, fmt.Errorf("error resolving readings filter: %w", err)
	}
	if !resolved {
		return nil, nil
	}

	args := append([]any{startDate, endDate}, filterArgs...)
	rows, err := r.db.QueryContext(ctx, lastBetweenQuery(bucket, clause), args...)
	if err != nil {
		return nil, fmt.Errorf("error fetching last-value readings between %s and %s: %w", startDate, endDate, err)
	}
	defer func() { _ = rows.Close() }()

	return scanReadings(rows)
}

func timeBucketExpression(interval AggregationInterval) (string, error) {
	switch interval {
	case AggregationPT10S:
		return "strftime('%Y-%m-%d %H:%M:', r.time) || printf('%02d', (CAST(strftime('%S', r.time) AS INTEGER) / 10) * 10)", nil
	case AggregationPT1M:
		return "strftime('%Y-%m-%d %H:%M:00', r.time)", nil
	case AggregationPT5M:
		return "strftime('%Y-%m-%d %H:', r.time) || printf('%02d', (CAST(strftime('%M', r.time) AS INTEGER) / 5) * 5) || ':00'", nil
	case AggregationPT15M:
		return "strftime('%Y-%m-%d %H:', r.time) || printf('%02d', (CAST(strftime('%M', r.time) AS INTEGER) / 15) * 15) || ':00'", nil
	case AggregationPT1H:
		return "strftime('%Y-%m-%d %H:00:00', r.time)", nil
	case AggregationP1D:
		return "strftime('%Y-%m-%d 00:00:00', r.time)", nil
	default:
		return "", fmt.Errorf("unsupported aggregation interval: %q", interval)
	}
}

func (r *ReadingsRepositoryImpl) GetLatest(ctx context.Context) ([]gen.Reading, error) {
	query := fmt.Sprintf(`
		SELECT sub.id, sub.sensor_name, sub.measurement_type, sub.numeric_value, sub.text_state, sub.unit, sub.time
		FROM (
			SELECT r.id, s.name AS sensor_name, mt.name AS measurement_type,
				r.numeric_value, r.text_state, COALESCE(smt.unit, mt.default_unit) AS unit, r.time,
				ROW_NUMBER() OVER (PARTITION BY r.sensor_id, r.measurement_type_id ORDER BY r.time DESC) AS rn
			FROM %s r
			JOIN sensors s ON r.sensor_id = s.id
			JOIN %s mt ON r.measurement_type_id = mt.id
			LEFT JOIN %s smt ON smt.sensor_id = s.id AND smt.measurement_type_id = mt.id
		) sub
		WHERE sub.rn = 1
	`, TableReadings, TableMeasurementTypes, TableSensorMeasurementTypes)

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error fetching latest readings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanReadings(rows)
}

func (r *ReadingsRepositoryImpl) GetTotalReadingsBySensorId(ctx context.Context, sensorId int) (int, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE sensor_id = ?", TableReadings)
	var count int
	err := r.db.QueryRowContext(ctx, query, sensorId).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("error fetching total readings for sensor ID %d: %w", sensorId, err)
	}
	return count, nil
}

func (r *ReadingsRepositoryImpl) DeleteReadingsOlderThan(ctx context.Context, cutoffDateTime time.Time) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE time < ?", TableReadings)
	if _, err := r.db.ExecContext(ctx, query, cutoffDateTime); err != nil {
		return fmt.Errorf("error deleting old readings: %w", err)
	}
	return nil
}

func (r *ReadingsRepositoryImpl) DeleteReadingsOlderThanForSensor(ctx context.Context, cutoffDateTime time.Time, sensorId int) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE sensor_id = ? AND time < ?", TableReadings)
	if _, err := r.db.ExecContext(ctx, query, sensorId, cutoffDateTime); err != nil {
		return fmt.Errorf("error deleting old readings for sensor %d: %w", sensorId, err)
	}
	return nil
}

func (r *ReadingsRepositoryImpl) DeleteReadingsOlderThanExcludingSensors(ctx context.Context, cutoffDateTime time.Time, excludedSensorIds []int) error {
	if len(excludedSensorIds) == 0 {
		return r.DeleteReadingsOlderThan(ctx, cutoffDateTime)
	}
	placeholders := strings.Repeat("?,", len(excludedSensorIds))
	placeholders = placeholders[:len(placeholders)-1]
	query := fmt.Sprintf("DELETE FROM %s WHERE time < ? AND sensor_id NOT IN (%s)", TableReadings, placeholders)
	args := make([]any, 0, 1+len(excludedSensorIds))
	args = append(args, cutoffDateTime)
	for _, id := range excludedSensorIds {
		args = append(args, id)
	}
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("error deleting old readings: %w", err)
	}
	return nil
}

func (r *ReadingsRepositoryImpl) resolveMeasurementTypeID(ctx context.Context, name string) (int, error) {
	var id int
	err := r.db.QueryRowContext(ctx, fmt.Sprintf("SELECT id FROM %s WHERE LOWER(name) = LOWER(?)", TableMeasurementTypes), name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("measurement type %q not found: %w", name, err)
	}
	return id, nil
}

func (r *ReadingsRepositoryImpl) resolveSensorID(ctx context.Context, name string) (int, error) {
	var id int
	err := r.db.QueryRowContext(ctx, "SELECT id FROM sensors WHERE LOWER(name) = LOWER(?)", name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("sensor %q not found: %w", name, err)
	}
	return id, nil
}

func scanReadings(rows *sql.Rows) ([]gen.Reading, error) {
	var readings []gen.Reading
	for rows.Next() {
		var reading gen.Reading
		err := rows.Scan(&reading.Id, &reading.SensorName, &reading.MeasurementType, &reading.NumericValue, &reading.TextState, &reading.Unit, &reading.Time)
		if err != nil {
			return nil, fmt.Errorf("error scanning reading row: %w", err)
		}
		reading.Time = utils.NormalizeTimeToSpaceFormat(reading.Time)
		readings = append(readings, reading)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over reading rows: %w", err)
	}
	return readings, nil
}
