package database

import (
	"context"
	"database/sql"
	"errors"
	gen "example/sensorHub/gen"
	"example/sensorHub/utils"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"
)

type seriesKey struct {
	sensorID int
	typeID   int
}

type ReadingsRepositoryImpl struct {
	db      *Handles
	sensors SensorIDResolver
	types   MeasurementTypeIDResolver
	logger  *slog.Logger

	pairMu      sync.RWMutex
	knownPairs  map[seriesKey]struct{}
	pairsLoaded bool
}

func NewReadingsRepository(db *Handles, sensors SensorIDResolver, types MeasurementTypeIDResolver, logger *slog.Logger) ReadingsRepository {
	repo := &ReadingsRepositoryImpl{
		db:         db,
		sensors:    sensors,
		types:      types,
		logger:     logger.With("component", "readings_repository"),
		knownPairs: make(map[seriesKey]struct{}),
	}
	if err := repo.loadKnownPairs(context.Background()); err != nil {
		repo.logger.Warn("could not seed the known series set at startup", "error", err)
	}
	return repo
}

func (r *ReadingsRepositoryImpl) loadKnownPairs(ctx context.Context) error {
	r.pairMu.RLock()
	loaded := r.pairsLoaded
	r.pairMu.RUnlock()
	if loaded {
		return nil
	}

	query := fmt.Sprintf("SELECT sensor_id, measurement_type_id FROM %s", TableSensorMeasurementTypes)
	rows, err := r.db.Reader.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("error reading known series: %w", err)
	}
	defer func() { _ = rows.Close() }()

	pairs := make(map[seriesKey]struct{})
	for rows.Next() {
		var key seriesKey
		if err := rows.Scan(&key.sensorID, &key.typeID); err != nil {
			return fmt.Errorf("error scanning known series row: %w", err)
		}
		pairs[key] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error reading known series: %w", err)
	}

	r.pairMu.Lock()
	for key := range pairs {
		r.knownPairs[key] = struct{}{}
	}
	r.pairsLoaded = true
	r.pairMu.Unlock()
	return nil
}

func (r *ReadingsRepositoryImpl) knows(key seriesKey) bool {
	r.pairMu.RLock()
	defer r.pairMu.RUnlock()
	_, ok := r.knownPairs[key]
	return ok
}

func (r *ReadingsRepositoryImpl) remember(keys []seriesKey) {
	if len(keys) == 0 {
		return
	}
	r.pairMu.Lock()
	for _, key := range keys {
		r.knownPairs[key] = struct{}{}
	}
	r.pairMu.Unlock()
}

func (r *ReadingsRepositoryImpl) Ingest(ctx context.Context, batch ReadingBatch) error {
	if len(batch.Readings) == 0 {
		return nil
	}

	sensorID, err := r.sensors.GetSensorIdByName(ctx, batch.SensorName)
	if err != nil {
		return fmt.Errorf("issue finding sensor id: %w", err)
	}

	if err := r.loadKnownPairs(ctx); err != nil {
		return err
	}

	type resolved struct {
		typeID  int
		reading gen.Reading
	}
	recognised := make([]resolved, 0, len(batch.Readings))
	for _, reading := range batch.Readings {
		typeID, err := r.types.GetIdByName(ctx, reading.MeasurementType)
		if err != nil {
			r.logger.Warn("skipping reading with unknown measurement type",
				"sensor", batch.SensorName, "type", reading.MeasurementType)
			continue
		}
		recognised = append(recognised, resolved{typeID: typeID, reading: reading})
	}
	if len(recognised) == 0 {
		return fmt.Errorf("no readings stored: all %d readings had unrecognised measurement types", len(batch.Readings))
	}

	tx, err := r.db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("issue beginning the ingest transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	insert, err := tx.PrepareContext(ctx, insertReadingQuery())
	if err != nil {
		return fmt.Errorf("issue preparing the reading insert: %w", err)
	}
	defer func() { _ = insert.Close() }()

	var firstSeen []seriesKey
	for _, item := range recognised {
		if _, err := insert.ExecContext(ctx, sensorID, item.typeID, item.reading.NumericValue, item.reading.TextState, utils.NormalizeTimeToSpaceFormat(item.reading.Time)); err != nil {
			return fmt.Errorf("issue persisting reading to database: %w", err)
		}

		key := seriesKey{sensorID: sensorID, typeID: item.typeID}
		if r.knows(key) || slices.Contains(firstSeen, key) {
			continue
		}
		if _, err := tx.ExecContext(ctx, insertSeriesQuery(), sensorID, item.typeID); err != nil {
			return fmt.Errorf("issue recording the series: %w", err)
		}
		firstSeen = append(firstSeen, key)
	}

	if err := updateSensorHealthTx(ctx, tx, sensorID, gen.Good, batch.HealthReason); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("issue committing the ingest transaction: %w", err)
	}

	r.remember(firstSeen)
	r.logger.Debug("stored readings", "sensor", batch.SensorName, "count", len(recognised))
	return nil
}

func insertReadingQuery() string {
	return fmt.Sprintf("INSERT INTO %s (sensor_id, measurement_type_id, numeric_value, text_state, time) VALUES (?, ?, ?, ?, ?)", TableReadings)
}

func insertSeriesQuery() string {
	return fmt.Sprintf("INSERT OR IGNORE INTO %s (sensor_id, measurement_type_id) VALUES (?, ?)", TableSensorMeasurementTypes)
}

func (r *ReadingsRepositoryImpl) GetBetweenDates(ctx context.Context, startDate, endDate, sensorName, measurementType string, interval AggregationInterval, aggFunc AggregationFunction) ([]gen.Reading, error) {
	if interval == AggregationRaw || interval == "" {
		return r.getRawBetweenDates(ctx, startDate, endDate, sensorName, measurementType)
	}
	return r.getAggregatedBetweenDates(ctx, startDate, endDate, sensorName, measurementType, interval, aggFunc)
}

type seriesScope struct {
	sensorID *int
	typeID   *int
}

func (s seriesScope) filter(alias string) (clause string, args []any) {
	if s.sensorID != nil {
		clause += " AND " + alias + ".sensor_id = ?"
		args = append(args, *s.sensorID)
	}
	if s.typeID != nil {
		clause += " AND " + alias + ".measurement_type_id = ?"
		args = append(args, *s.typeID)
	}
	return clause, args
}

// resolveSeries resolves the optional sensor/measurement names to ids, which the scope
// renders as an id-based WHERE fragment (plus its bind args). Filtering on sensor_id /
// measurement_type_id lets the (sensor_id, measurement_type_id, time DESC) composite
// index serve the query, instead of a full-table scan filtered by joined names.
// resolved is false when a provided name does not exist, signalling the caller to return
// an empty result (matching the prior LOWER(name) behaviour). Resolution is
// case-insensitive, as before.
func (r *ReadingsRepositoryImpl) resolveSeries(ctx context.Context, sensorName, measurementType string) (scope seriesScope, resolved bool, err error) {
	if sensorName != "" {
		id, e := r.sensors.GetSensorIdByName(ctx, sensorName)
		if errors.Is(e, sql.ErrNoRows) {
			return seriesScope{}, false, nil
		}
		if e != nil {
			return seriesScope{}, false, e
		}
		scope.sensorID = &id
	}
	if measurementType != "" {
		id, e := r.types.GetIdByName(ctx, measurementType)
		if errors.Is(e, sql.ErrNoRows) {
			return seriesScope{}, false, nil
		}
		if e != nil {
			return seriesScope{}, false, e
		}
		scope.typeID = &id
	}
	return scope, true, nil
}

func rawBetweenQuery(seriesClause string) string {
	return fmt.Sprintf(`
		SELECT r.id, s.name, mt.name, r.numeric_value, r.text_state, COALESCE(NULLIF(smt.unit, ''), mt.default_unit), r.time
		FROM %s r
		JOIN sensors s ON r.sensor_id = s.id
		JOIN %s mt ON r.measurement_type_id = mt.id
		LEFT JOIN %s smt ON smt.sensor_id = s.id AND smt.measurement_type_id = mt.id
		WHERE r.time BETWEEN ? AND ?%s
		ORDER BY r.time ASC
	`, TableReadings, TableMeasurementTypes, TableSensorMeasurementTypes, seriesClause)
}

func (r *ReadingsRepositoryImpl) getRawBetweenDates(ctx context.Context, startDate, endDate, sensorName, measurementType string) ([]gen.Reading, error) {
	scope, resolved, err := r.resolveSeries(ctx, sensorName, measurementType)
	if err != nil {
		return nil, fmt.Errorf("error resolving readings filter: %w", err)
	}
	if !resolved {
		return nil, nil
	}
	clause, filterArgs := scope.filter("r")

	args := append([]any{startDate, endDate}, filterArgs...)
	rows, err := r.db.Reader.QueryContext(ctx, rawBetweenQuery(clause), args...)
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
	if aggFunc == AggregationFunctionIncrease {
		return r.getIncreaseBetweenDates(ctx, startDate, endDate, sensorName, measurementType, bucket)
	}

	sqlAgg := aggregateExpression(aggFunc)

	scope, resolved, err := r.resolveSeries(ctx, sensorName, measurementType)
	if err != nil {
		return nil, fmt.Errorf("error resolving readings filter: %w", err)
	}
	if !resolved {
		return nil, nil
	}
	clause, filterArgs := scope.filter("r")

	args := append([]any{startDate, endDate}, filterArgs...)
	rows, err := r.db.Reader.QueryContext(ctx, aggregatedBetweenQuery(sqlAgg, bucket, clause), args...)
	if err != nil {
		return nil, fmt.Errorf("error fetching aggregated readings between %s and %s: %w", startDate, endDate, err)
	}
	defer func() { _ = rows.Close() }()

	return scanReadings(rows)
}

func aggregateExpression(aggFunc AggregationFunction) string {
	switch aggFunc {
	case AggregationFunctionCount:
		return "COUNT(*)"
	case AggregationFunctionMin:
		return "ROUND(MIN(r.numeric_value), 2)"
	case AggregationFunctionMax:
		return "ROUND(MAX(r.numeric_value), 2)"
	default:
		return "ROUND(AVG(r.numeric_value), 2)"
	}
}

func aggregatedBetweenQuery(sqlAgg, bucket, seriesClause string) string {
	return fmt.Sprintf(`
		SELECT 0 AS id, s.name, mt.name, %s, NULL, COALESCE(NULLIF(smt.unit, ''), mt.default_unit), %s AS bucket_time
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
				r.numeric_value, r.text_state, COALESCE(NULLIF(smt.unit, ''), mt.default_unit) AS unit,
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
	scope, resolved, err := r.resolveSeries(ctx, sensorName, measurementType)
	if err != nil {
		return nil, fmt.Errorf("error resolving readings filter: %w", err)
	}
	if !resolved {
		return nil, nil
	}
	clause, filterArgs := scope.filter("r")

	args := append([]any{startDate, endDate}, filterArgs...)
	rows, err := r.db.Reader.QueryContext(ctx, lastBetweenQuery(bucket, clause), args...)
	if err != nil {
		return nil, fmt.Errorf("error fetching last-value readings between %s and %s: %w", startDate, endDate, err)
	}
	defer func() { _ = rows.Close() }()

	return scanReadings(rows)
}

func increaseBetweenQuery(bucket, seriesClause, pairClause string) string {
	return fmt.Sprintf(`
		WITH in_play AS (
			SELECT r.id, r.sensor_id, r.measurement_type_id, r.time, r.numeric_value, %s AS bucket_time, 1 AS in_range
			FROM %s r
			WHERE r.time BETWEEN ? AND ?%s
			UNION ALL
			SELECT r.id, r.sensor_id, r.measurement_type_id, r.time, r.numeric_value, NULL, 0
			FROM %s pair
			JOIN %s r ON r.id = (
				SELECT earlier.id FROM %s earlier
				WHERE earlier.sensor_id = pair.sensor_id AND earlier.measurement_type_id = pair.measurement_type_id
					AND earlier.time < ?
				ORDER BY earlier.time DESC, earlier.id DESC
				LIMIT 1
			)
			WHERE 1 = 1%s
		),
		stepped AS (
			SELECT sensor_id, measurement_type_id, numeric_value, bucket_time, in_range,
				numeric_value - LAG(numeric_value) OVER (PARTITION BY sensor_id, measurement_type_id ORDER BY time, id) AS step
			FROM in_play
		)
		SELECT 0 AS id, s.name, mt.name,
			ROUND(SUM(CASE WHEN st.step IS NULL THEN 0 WHEN st.step < 0 THEN st.numeric_value ELSE st.step END), 2),
			NULL, COALESCE(NULLIF(smt.unit, ''), mt.default_unit), st.bucket_time
		FROM stepped st
		JOIN sensors s ON st.sensor_id = s.id
		JOIN %s mt ON st.measurement_type_id = mt.id
		LEFT JOIN %s smt ON smt.sensor_id = s.id AND smt.measurement_type_id = mt.id
		WHERE st.in_range = 1
		GROUP BY s.name, mt.name, st.bucket_time ORDER BY st.bucket_time ASC
	`, bucket, TableReadings, seriesClause, TableSensorMeasurementTypes, TableReadings, TableReadings, pairClause, TableMeasurementTypes, TableSensorMeasurementTypes)
}

func (r *ReadingsRepositoryImpl) getIncreaseBetweenDates(ctx context.Context, startDate, endDate, sensorName, measurementType, bucket string) ([]gen.Reading, error) {
	scope, resolved, err := r.resolveSeries(ctx, sensorName, measurementType)
	if err != nil {
		return nil, fmt.Errorf("error resolving readings filter: %w", err)
	}
	if !resolved {
		return nil, nil
	}
	clause, filterArgs := scope.filter("r")
	pairClause, pairArgs := scope.filter("pair")

	args := append([]any{startDate, endDate}, filterArgs...)
	args = append(args, startDate)
	args = append(args, pairArgs...)
	rows, err := r.db.Reader.QueryContext(ctx, increaseBetweenQuery(bucket, clause, pairClause), args...)
	if err != nil {
		return nil, fmt.Errorf("error fetching increase readings between %s and %s: %w", startDate, endDate, err)
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

func latestPerSeriesQuery() string {
	return fmt.Sprintf(`
		SELECT r.id, s.name, mt.name, r.numeric_value, r.text_state,
			COALESCE(NULLIF(smt.unit, ''), mt.default_unit), r.time
		FROM %s smt
		JOIN sensors s ON s.id = smt.sensor_id
		JOIN %s mt ON mt.id = smt.measurement_type_id
		JOIN %s r ON r.id = (
			SELECT latest.id FROM %s latest
			WHERE latest.sensor_id = smt.sensor_id AND latest.measurement_type_id = smt.measurement_type_id
			ORDER BY latest.time DESC
			LIMIT 1
		)
	`, TableSensorMeasurementTypes, TableMeasurementTypes, TableReadings, TableReadings)
}

func (r *ReadingsRepositoryImpl) GetLatest(ctx context.Context) ([]gen.Reading, error) {
	rows, err := r.db.Reader.QueryContext(ctx, latestPerSeriesQuery())
	if err != nil {
		return nil, fmt.Errorf("error fetching latest readings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanReadings(rows)
}

func countReadingsPerSensorQuery() string {
	return fmt.Sprintf(`SELECT s.name, COALESCE(counted.total, 0)
		FROM sensors s
		LEFT JOIN (SELECT sensor_id, COUNT(*) AS total FROM %s GROUP BY sensor_id) counted
			ON counted.sensor_id = s.id
		WHERE s.status = 'active'`, TableReadings)
}

func (r *ReadingsRepositoryImpl) CountReadingsPerActiveSensor(ctx context.Context) (map[string]int, error) {
	rows, err := r.db.Reader.QueryContext(ctx, countReadingsPerSensorQuery())
	if err != nil {
		return nil, fmt.Errorf("error counting readings per sensor: %w", err)
	}
	defer func() { _ = rows.Close() }()

	counts := make(map[string]int)
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return nil, fmt.Errorf("error scanning reading count: %w", err)
		}
		counts[name] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error counting readings per sensor: %w", err)
	}
	return counts, nil
}

func deleteOlderThanQuery() string {
	return fmt.Sprintf("DELETE FROM %s WHERE time < ?", TableReadings)
}

func deleteOlderThanForSensorQuery() string {
	return fmt.Sprintf("DELETE FROM %s WHERE sensor_id = ? AND time < ?", TableReadings)
}

func deleteOlderThanExcludingSensorsQuery(excluded int) string {
	placeholders := strings.Repeat("?,", excluded)
	placeholders = placeholders[:len(placeholders)-1]
	return fmt.Sprintf("DELETE FROM %s WHERE time < ? AND sensor_id NOT IN (%s)", TableReadings, placeholders)
}

func (r *ReadingsRepositoryImpl) DeleteReadingsOlderThan(ctx context.Context, cutoffDateTime time.Time) error {
	if _, err := r.db.Writer.ExecContext(ctx, deleteOlderThanQuery(), utils.FormatStorageTime(cutoffDateTime)); err != nil {
		return fmt.Errorf("error deleting old readings: %w", err)
	}
	return nil
}

func (r *ReadingsRepositoryImpl) DeleteReadingsOlderThanForSensor(ctx context.Context, cutoffDateTime time.Time, sensorId int) error {
	if _, err := r.db.Writer.ExecContext(ctx, deleteOlderThanForSensorQuery(), sensorId, utils.FormatStorageTime(cutoffDateTime)); err != nil {
		return fmt.Errorf("error deleting old readings for sensor %d: %w", sensorId, err)
	}
	return nil
}

func (r *ReadingsRepositoryImpl) DeleteReadingsOlderThanExcludingSensors(ctx context.Context, cutoffDateTime time.Time, excludedSensorIds []int) error {
	if len(excludedSensorIds) == 0 {
		return r.DeleteReadingsOlderThan(ctx, cutoffDateTime)
	}
	query := deleteOlderThanExcludingSensorsQuery(len(excludedSensorIds))
	args := make([]any, 0, 1+len(excludedSensorIds))
	args = append(args, utils.FormatStorageTime(cutoffDateTime))
	for _, id := range excludedSensorIds {
		args = append(args, id)
	}
	if _, err := r.db.Writer.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("error deleting old readings: %w", err)
	}
	return nil
}

func scanReadings(rows *sql.Rows) ([]gen.Reading, error) {
	var readings []gen.Reading
	for rows.Next() {
		var reading gen.Reading
		err := rows.Scan(&reading.Id, &reading.SensorName, &reading.MeasurementType, &reading.NumericValue, &reading.TextState, &reading.Unit, &reading.Time)
		if err != nil {
			return nil, fmt.Errorf("error scanning reading row: %w", err)
		}
		reading.Time = utils.NormalizeTimeToRFC3339(reading.Time)
		readings = append(readings, reading)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over reading rows: %w", err)
	}
	return readings, nil
}
