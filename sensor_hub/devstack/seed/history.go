package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"example/sensorHub/alerting"
	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/utils"
)

const (
	historyWindow     = 30 * 24 * time.Hour
	readingInterval   = time.Minute
	spellPeriod       = 4 * 24 * time.Hour
	shortestSpell     = 15 * time.Minute
	longestSpell      = 90 * time.Minute
	readingsPerExec   = 32
	readingColumns    = 5
	bulkWriteCacheKiB = 256 * 1024
)

const seededHealthReason = "seeded history"

type span struct {
	from   time.Time
	points int
}

func (s span) at(point int) time.Time {
	return s.from.Add(time.Duration(point+1) * readingInterval)
}

type healthChange struct {
	status gen.SensorHealthStatus
	at     time.Time
}

type alertSent struct {
	rule   alerting.AlertRule
	value  value
	reason string
	at     time.Time
}

type sensorHistory struct {
	sensorID int
	span     span
	tracks   []track
	health   []healthChange
	alerts   []alertSent
}

func (s *seeder) topUpHistory(ctx context.Context, now time.Time) error {
	started := time.Now()
	ids, err := seededSensorIDs(ctx, s.db.Reader)
	if err != nil {
		return err
	}
	typeIDs, err := s.measurementTypeIDs(ctx)
	if err != nil {
		return err
	}
	r := rand.New(rand.NewPCG(uint64(now.UnixNano()), 0))
	var histories []sensorHistory
	for _, device := range s.devices() {
		sensorID, seeded := ids[device.Name]
		if !seeded {
			continue
		}
		history, err := s.planHistory(ctx, r, sensorID, device, now)
		if err != nil {
			return fmt.Errorf("sensor %s: %w", device.Name, err)
		}
		histories = append(histories, history)
	}
	readings, err := s.writeHistories(ctx, histories, typeIDs)
	if err != nil {
		return err
	}
	s.logger.Info("topped up the history", "sensors", len(histories), "readings", readings, "took", time.Since(started).Round(time.Millisecond))
	return nil
}

func (s *seeder) measurementTypeIDs(ctx context.Context) (map[string]int, error) {
	types, err := s.sensors.ServiceGetAllMeasurementTypes(ctx)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]int, len(types))
	for _, measurementType := range types {
		ids[measurementType.Name] = measurementType.Id
	}
	return ids, nil
}

func (s *seeder) planHistory(ctx context.Context, r *rand.Rand, sensorID int, device device, now time.Time) (sensorHistory, error) {
	latest, newest, err := s.latestReadings(ctx, sensorID)
	if err != nil {
		return sensorHistory{}, err
	}
	from := now.Add(-s.window)
	if newest.After(from) {
		from = newest
	}
	history := sensorHistory{sensorID: sensorID, span: span{from: from, points: int(now.Sub(from) / readingInterval)}}
	if history.span.points <= 0 {
		return history, nil
	}
	for _, signal := range device.signals {
		history.tracks = append(history.tracks, signal.generate(r, history.span.points, latest)...)
	}
	lastHealth, err := s.lastHealthChange(ctx, sensorID)
	if err != nil {
		return sensorHistory{}, err
	}
	history.health = healthChanges(sensorID, history.span, lastHealth, now)
	rules, err := s.alerts.ServiceGetAlertRulesBySensorID(ctx, sensorID)
	if err != nil {
		return sensorHistory{}, err
	}
	history.alerts = alertsSent(rules, history.tracks, history.span)
	return history, nil
}

func (s *seeder) latestReadings(ctx context.Context, sensorID int) (map[string]value, time.Time, error) {
	rows, err := s.db.Reader.QueryContext(ctx, `
		SELECT mt.name, r.numeric_value, r.text_state, r.time
		FROM sensor_measurement_types smt
		JOIN measurement_types mt ON mt.id = smt.measurement_type_id
		JOIN readings r ON r.id = (
			SELECT id FROM readings
			WHERE sensor_id = smt.sensor_id AND measurement_type_id = smt.measurement_type_id
			ORDER BY time DESC LIMIT 1)
		WHERE smt.sensor_id = ?`, sensorID)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to read the newest readings: %w", err)
	}
	defer rows.Close()
	latest := make(map[string]value)
	var newest time.Time
	for rows.Next() {
		var name string
		var number sql.NullFloat64
		var state sql.NullString
		var at database.SQLiteTime
		if err := rows.Scan(&name, &number, &state, &at); err != nil {
			return nil, time.Time{}, fmt.Errorf("failed to read the newest readings: %w", err)
		}
		latest[name] = value{number: number.Float64, state: state.String}
		if at.After(newest) {
			newest = at.Time
		}
	}
	return latest, newest, rows.Err()
}

func (s *seeder) lastHealthChange(ctx context.Context, sensorID int) (healthChange, error) {
	var last healthChange
	var at database.SQLiteTime
	err := s.db.Reader.QueryRowContext(ctx, `
		SELECT health_status, recorded_at FROM sensor_health_history
		WHERE sensor_id = ? ORDER BY datetime(recorded_at) DESC, id DESC LIMIT 1`, sensorID).Scan(&last.status, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return healthChange{}, nil
	}
	if err != nil {
		return healthChange{}, fmt.Errorf("failed to read the latest health: %w", err)
	}
	last.at = at.Time
	return last, nil
}

func healthChanges(sensorID int, span span, last healthChange, now time.Time) []healthChange {
	start := span.at(0)
	if !last.at.Before(start) {
		start = last.at.Add(readingInterval)
	}
	var changes []healthChange
	if last.status != gen.Good && !start.After(now) {
		changes = append(changes, healthChange{status: gen.Good, at: start})
	}
	for index := start.UnixNano() / int64(spellPeriod); ; index++ {
		begins, ends := unhealthySpell(sensorID, index)
		if ends.After(now) {
			return changes
		}
		if begins.After(start) {
			changes = append(changes, healthChange{status: gen.Bad, at: begins}, healthChange{status: gen.Good, at: ends})
		}
	}
}

func unhealthySpell(sensorID int, index int64) (begins, ends time.Time) {
	r := rand.New(rand.NewPCG(uint64(sensorID), uint64(index)))
	begins = time.Unix(0, index*int64(spellPeriod)).UTC().Add(time.Duration(r.Int64N(int64(spellPeriod - longestSpell)))).Truncate(time.Second)
	ends = begins.Add(shortestSpell + time.Duration(r.Int64N(int64(longestSpell-shortestSpell)))).Truncate(time.Second)
	return begins, ends
}

func alertsSent(rules []alerting.AlertRule, tracks []track, span span) []alertSent {
	var sent []alertSent
	for _, rule := range rules {
		for _, track := range tracks {
			if !strings.EqualFold(track.measurement, rule.MeasurementType) {
				continue
			}
			last := rule.LastAlertSentAt
			rateLimit := time.Duration(rule.RateLimitSeconds) * time.Second
			for point, reading := range track.values {
				fires, reason := rule.ShouldAlert(reading.number, reading.state)
				at := span.at(point)
				if !fires || (last != nil && at.Sub(*last) < rateLimit) {
					continue
				}
				sent = append(sent, alertSent{rule: rule, value: reading, reason: reason, at: at})
				last = &at
			}
		}
	}
	return sent
}

func (s *seeder) writeHistories(ctx context.Context, histories []sensorHistory, typeIDs map[string]int) (int, error) {
	if _, err := s.db.Writer.ExecContext(ctx, fmt.Sprintf("PRAGMA cache_size = -%d", bulkWriteCacheKiB)); err != nil {
		return 0, fmt.Errorf("failed to size the page cache: %w", err)
	}
	tx, err := s.db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE staged_readings
		(sensor_id INTEGER, measurement_type_id INTEGER, numeric_value REAL, text_state TEXT, time TEXT)`); err != nil {
		return 0, fmt.Errorf("failed to stage the readings: %w", err)
	}
	readings := 0
	for _, history := range histories {
		if err := writeHistory(ctx, tx, history, typeIDs); err != nil {
			return 0, err
		}
		readings += history.span.points * len(history.tracks)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO readings (sensor_id, measurement_type_id, numeric_value, text_state, time)
		SELECT sensor_id, measurement_type_id, numeric_value, text_state, time FROM staged_readings`); err != nil {
		return 0, fmt.Errorf("failed to write the readings: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DROP TABLE staged_readings"); err != nil {
		return 0, fmt.Errorf("failed to drop the staged readings: %w", err)
	}
	return readings, tx.Commit()
}

func writeHistory(ctx context.Context, tx *sql.Tx, history sensorHistory, typeIDs map[string]int) error {
	if history.span.points <= 0 {
		return nil
	}
	if err := stageReadings(ctx, tx, history, typeIDs); err != nil {
		return err
	}
	if err := writeHealth(ctx, tx, history); err != nil {
		return err
	}
	for _, alert := range history.alerts {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO alert_sent_history (alert_rule_id, sensor_id, measurement_type_id, alert_reason, reading_value, reading_status, sent_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			alert.rule.ID, history.sensorID, alert.rule.MeasurementTypeId, alert.reason, alert.value.number, alert.value.state,
			utils.FormatStorageTime(alert.at)); err != nil {
			return fmt.Errorf("failed to write the alert history: %w", err)
		}
	}
	return nil
}

func writeHealth(ctx context.Context, tx *sql.Tx, history sensorHistory) error {
	if len(history.health) == 0 {
		return nil
	}
	for _, change := range history.health {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO sensor_health_history (sensor_id, health_status, recorded_at) VALUES (?, ?, ?)",
			history.sensorID, change.status, utils.FormatStorageTime(change.at)); err != nil {
			return fmt.Errorf("failed to write the health history: %w", err)
		}
	}
	current := history.health[len(history.health)-1].status
	if _, err := tx.ExecContext(ctx, "UPDATE sensors SET health_status = ?, health_reason = ? WHERE id = ?",
		current, seededHealthReason, history.sensorID); err != nil {
		return fmt.Errorf("failed to bring the sensor health in line with its history: %w", err)
	}
	return nil
}

func stageReadings(ctx context.Context, tx *sql.Tx, history sensorHistory, typeIDs map[string]int) error {
	typeOf := make([]int, len(history.tracks))
	for i, track := range history.tracks {
		typeID, known := typeIDs[track.measurement]
		if !known {
			return fmt.Errorf("measurement type %s does not exist", track.measurement)
		}
		typeOf[i] = typeID
		if _, err := tx.ExecContext(ctx,
			"INSERT OR IGNORE INTO sensor_measurement_types (sensor_id, measurement_type_id) VALUES (?, ?)",
			history.sensorID, typeID); err != nil {
			return fmt.Errorf("failed to record the series %s: %w", track.measurement, err)
		}
	}
	batch, err := tx.PrepareContext(ctx, stageReadingsStatement(readingsPerExec))
	if err != nil {
		return fmt.Errorf("failed to prepare the staged reading insert: %w", err)
	}
	defer batch.Close()

	args := make([]any, 0, readingsPerExec*readingColumns)
	for point := range history.span.points {
		at := utils.FormatStorageTime(history.span.at(point))
		for i, track := range history.tracks {
			reading := track.values[point]
			args = append(args, history.sensorID, typeOf[i], reading.numberOrNil(), reading.stateOrNil(), at)
			if len(args) == cap(args) {
				if _, err := batch.ExecContext(ctx, args...); err != nil {
					return fmt.Errorf("failed to stage readings: %w", err)
				}
				args = args[:0]
			}
		}
	}
	if len(args) == 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, stageReadingsStatement(len(args)/readingColumns), args...); err != nil {
		return fmt.Errorf("failed to stage readings: %w", err)
	}
	return nil
}

func stageReadingsStatement(rows int) string {
	return "INSERT INTO staged_readings (sensor_id, measurement_type_id, numeric_value, text_state, time) VALUES " +
		strings.TrimSuffix(strings.Repeat("("+strings.TrimSuffix(strings.Repeat("?, ", readingColumns), ", ")+"),", rows), ",")
}
