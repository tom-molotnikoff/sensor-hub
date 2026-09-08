package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"example/sensorHub/alerting"
	gen "example/sensorHub/gen"
)

type AlertRepository interface {
	GetAlertRule(ctx context.Context, sensorID, measurementTypeId int) (*alerting.AlertRule, error)
	GetAlertRuleByID(ctx context.Context, ruleID int) (*alerting.AlertRule, error)
	GetAlertRuleBySensorID(ctx context.Context, sensorID int) (*alerting.AlertRule, error)
	GetAlertRulesBySensorID(ctx context.Context, sensorID int) ([]alerting.AlertRule, error)
	RecordAlertSent(ctx context.Context, ruleID, sensorID, measurementTypeId int, reason string, numericValue float64, statusValue string) error
	GetAllAlertRules(ctx context.Context) ([]alerting.AlertRule, error)
	GetAlertRuleBySensorName(ctx context.Context, sensorName string) (*alerting.AlertRule, error)
	CreateAlertRule(ctx context.Context, rule *alerting.AlertRule) error
	UpdateAlertRule(ctx context.Context, rule *alerting.AlertRule) error
	DeleteAlertRule(ctx context.Context, ruleID int) error
	GetAlertHistory(ctx context.Context, sensorID int, limit int) ([]gen.AlertHistoryEntry, error)
	DeleteAlertHistoryOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

type AlertRepositoryImpl struct {
	db     *Handles
	logger *slog.Logger
}

func NewAlertRepository(db *Handles, logger *slog.Logger) AlertRepository {
	return &AlertRepositoryImpl{db: db, logger: logger.With("component", "alert_repository")}
}

func (r *AlertRepositoryImpl) GetAlertRule(ctx context.Context, sensorID, measurementTypeId int) (*alerting.AlertRule, error) {
	query := `
		SELECT 
			ar.id,
			ar.sensor_id,
			s.name,
			ar.measurement_type_id,
			mt.name,
			ar.alert_type,
			ar.high_threshold,
			ar.low_threshold,
			ar.trigger_status,
			ar.enabled,
			ar.rate_limit_seconds,
			ah.sent_at
		FROM sensor_alert_rules ar
		JOIN sensors s ON ar.sensor_id = s.id
		JOIN measurement_types mt ON ar.measurement_type_id = mt.id
		LEFT JOIN (
			SELECT alert_rule_id, MAX(sent_at) as sent_at
			FROM alert_sent_history
			GROUP BY alert_rule_id
		) ah ON ar.id = ah.alert_rule_id
		WHERE ar.sensor_id = ? AND ar.measurement_type_id = ? AND ar.enabled = TRUE
		LIMIT 1
	`

	var rule alerting.AlertRule
	var lastAlertSent NullSQLiteTime
	var triggerStatus sql.NullString

	err := r.db.Reader.QueryRowContext(ctx, query, sensorID, measurementTypeId).Scan(
		&rule.ID,
		&rule.SensorID,
		&rule.SensorName,
		&rule.MeasurementTypeId,
		&rule.MeasurementType,
		&rule.AlertType,
		&rule.HighThreshold,
		&rule.LowThreshold,
		&triggerStatus,
		&rule.Enabled,
		&rule.RateLimitSeconds,
		&lastAlertSent,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get alert rule for sensor %d, measurement type %d: %w", sensorID, measurementTypeId, err)
	}

	if lastAlertSent.Valid {
		rule.LastAlertSentAt = &lastAlertSent.Time
	}
	if triggerStatus.Valid {
		rule.TriggerStatus = triggerStatus.String
	}

	return &rule, nil
}

func (r *AlertRepositoryImpl) GetAlertRuleBySensorID(ctx context.Context, sensorID int) (*alerting.AlertRule, error) {
	query := `
		SELECT 
			ar.id,
			ar.sensor_id,
			s.name,
			ar.measurement_type_id,
			mt.name,
			ar.alert_type,
			ar.high_threshold,
			ar.low_threshold,
			ar.trigger_status,
			ar.enabled,
			ar.rate_limit_seconds,
			ah.sent_at
		FROM sensor_alert_rules ar
		JOIN sensors s ON ar.sensor_id = s.id
		JOIN measurement_types mt ON ar.measurement_type_id = mt.id
		LEFT JOIN (
			SELECT alert_rule_id, MAX(sent_at) as sent_at
			FROM alert_sent_history
			GROUP BY alert_rule_id
		) ah ON ar.id = ah.alert_rule_id
		WHERE ar.sensor_id = ? AND ar.enabled = TRUE
		LIMIT 1
	`

	var rule alerting.AlertRule
	var lastAlertSent NullSQLiteTime
	var triggerStatus sql.NullString

	err := r.db.Reader.QueryRowContext(ctx, query, sensorID).Scan(
		&rule.ID,
		&rule.SensorID,
		&rule.SensorName,
		&rule.MeasurementTypeId,
		&rule.MeasurementType,
		&rule.AlertType,
		&rule.HighThreshold,
		&rule.LowThreshold,
		&triggerStatus,
		&rule.Enabled,
		&rule.RateLimitSeconds,
		&lastAlertSent,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get alert rule for sensor %d: %w", sensorID, err)
	}

	if lastAlertSent.Valid {
		rule.LastAlertSentAt = &lastAlertSent.Time
	}

	if triggerStatus.Valid {
		rule.TriggerStatus = triggerStatus.String
	}

	return &rule, nil
}

func (r *AlertRepositoryImpl) GetAlertRuleByID(ctx context.Context, ruleID int) (*alerting.AlertRule, error) {
	query := `
		SELECT 
			ar.id,
			ar.sensor_id,
			s.name,
			ar.measurement_type_id,
			mt.name,
			ar.alert_type,
			ar.high_threshold,
			ar.low_threshold,
			ar.trigger_status,
			ar.enabled,
			ar.rate_limit_seconds,
			ah.sent_at
		FROM sensor_alert_rules ar
		JOIN sensors s ON ar.sensor_id = s.id
		JOIN measurement_types mt ON ar.measurement_type_id = mt.id
		LEFT JOIN (
			SELECT alert_rule_id, MAX(sent_at) as sent_at
			FROM alert_sent_history
			GROUP BY alert_rule_id
		) ah ON ar.id = ah.alert_rule_id
		WHERE ar.id = ?
	`

	var rule alerting.AlertRule
	var lastAlertSent NullSQLiteTime
	var triggerStatus sql.NullString

	err := r.db.Reader.QueryRowContext(ctx, query, ruleID).Scan(
		&rule.ID,
		&rule.SensorID,
		&rule.SensorName,
		&rule.MeasurementTypeId,
		&rule.MeasurementType,
		&rule.AlertType,
		&rule.HighThreshold,
		&rule.LowThreshold,
		&triggerStatus,
		&rule.Enabled,
		&rule.RateLimitSeconds,
		&lastAlertSent,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get alert rule %d: %w", ruleID, err)
	}

	if lastAlertSent.Valid {
		rule.LastAlertSentAt = &lastAlertSent.Time
	}
	if triggerStatus.Valid {
		rule.TriggerStatus = triggerStatus.String
	}

	return &rule, nil
}

func (r *AlertRepositoryImpl) GetAlertRulesBySensorID(ctx context.Context, sensorID int) ([]alerting.AlertRule, error) {
	query := `
		SELECT 
			ar.id,
			ar.sensor_id,
			s.name,
			ar.measurement_type_id,
			mt.name,
			ar.alert_type,
			ar.high_threshold,
			ar.low_threshold,
			ar.trigger_status,
			ar.enabled,
			ar.rate_limit_seconds,
			ah.sent_at
		FROM sensor_alert_rules ar
		JOIN sensors s ON ar.sensor_id = s.id
		JOIN measurement_types mt ON ar.measurement_type_id = mt.id
		LEFT JOIN (
			SELECT alert_rule_id, MAX(sent_at) as sent_at
			FROM alert_sent_history
			GROUP BY alert_rule_id
		) ah ON ar.id = ah.alert_rule_id
		WHERE ar.sensor_id = ?
	`

	rows, err := r.db.Reader.QueryContext(ctx, query, sensorID)
	if err != nil {
		return nil, fmt.Errorf("failed to get alert rules for sensor %d: %w", sensorID, err)
	}
	defer rows.Close()

	var rules []alerting.AlertRule
	for rows.Next() {
		var rule alerting.AlertRule
		var lastAlertSent NullSQLiteTime
		var triggerStatus sql.NullString
		err := rows.Scan(
			&rule.ID,
			&rule.SensorID,
			&rule.SensorName,
			&rule.MeasurementTypeId,
			&rule.MeasurementType,
			&rule.AlertType,
			&rule.HighThreshold,
			&rule.LowThreshold,
			&triggerStatus,
			&rule.Enabled,
			&rule.RateLimitSeconds,
			&lastAlertSent,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan alert rule: %w", err)
		}
		if triggerStatus.Valid {
			rule.TriggerStatus = triggerStatus.String
		}
		if lastAlertSent.Valid {
			rule.LastAlertSentAt = &lastAlertSent.Time
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

func (r *AlertRepositoryImpl) RecordAlertSent(ctx context.Context, ruleID, sensorID, measurementTypeId int, reason string, numericValue float64, statusValue string) error {
	query := `
		INSERT INTO alert_sent_history 
		(alert_rule_id, sensor_id, measurement_type_id, alert_reason, reading_value, reading_status)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Writer.ExecContext(ctx, query, ruleID, sensorID, measurementTypeId, reason, numericValue, statusValue)
	if err != nil {
		return fmt.Errorf("failed to record alert sent: %w", err)
	}

	return nil
}

func (r *AlertRepositoryImpl) GetAllAlertRules(ctx context.Context) ([]alerting.AlertRule, error) {
	query := `
		SELECT 
			sar.id,
			sar.sensor_id,
			s.name,
			sar.measurement_type_id,
			mt.name,
			sar.alert_type,
			sar.high_threshold,
			sar.low_threshold,
			sar.trigger_status,
			sar.enabled,
			sar.rate_limit_seconds,
			ash.sent_at
		FROM sensor_alert_rules sar
		INNER JOIN sensors s ON sar.sensor_id = s.id
		INNER JOIN measurement_types mt ON sar.measurement_type_id = mt.id
		LEFT JOIN (
			SELECT alert_rule_id, MAX(sent_at) as sent_at
			FROM alert_sent_history
			GROUP BY alert_rule_id
		) ash ON sar.id = ash.alert_rule_id
	`

	rows, err := r.db.Reader.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all alert rules: %w", err)
	}
	defer rows.Close()

	var rules []alerting.AlertRule
	for rows.Next() {
		var rule alerting.AlertRule
		var lastAlertSentAt NullSQLiteTime
		var triggerStatus sql.NullString
		err := rows.Scan(
			&rule.ID,
			&rule.SensorID,
			&rule.SensorName,
			&rule.MeasurementTypeId,
			&rule.MeasurementType,
			&rule.AlertType,
			&rule.HighThreshold,
			&rule.LowThreshold,
			&triggerStatus,
			&rule.Enabled,
			&rule.RateLimitSeconds,
			&lastAlertSentAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan alert rule: %w", err)
		}
		if triggerStatus.Valid {
			rule.TriggerStatus = triggerStatus.String
		}
		if lastAlertSentAt.Valid {
			rule.LastAlertSentAt = &lastAlertSentAt.Time
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

func (r *AlertRepositoryImpl) GetAlertRuleBySensorName(ctx context.Context, sensorName string) (*alerting.AlertRule, error) {
	query := `
		SELECT 
			sar.id,
			sar.sensor_id,
			s.name,
			sar.measurement_type_id,
			mt.name,
			sar.alert_type,
			sar.high_threshold,
			sar.low_threshold,
			sar.trigger_status,
			sar.enabled,
			sar.rate_limit_seconds,
			ash.sent_at
		FROM sensor_alert_rules sar
		INNER JOIN sensors s ON sar.sensor_id = s.id
		INNER JOIN measurement_types mt ON sar.measurement_type_id = mt.id
		LEFT JOIN (
			SELECT alert_rule_id, MAX(sent_at) as sent_at
			FROM alert_sent_history
			GROUP BY alert_rule_id
		) ash ON sar.id = ash.alert_rule_id
		WHERE LOWER(s.name) = LOWER(?)
	`

	var rule alerting.AlertRule
	var lastAlertSentAt NullSQLiteTime
	var triggerStatus sql.NullString
	err := r.db.Reader.QueryRowContext(ctx, query, sensorName).Scan(
		&rule.ID,
		&rule.SensorID,
		&rule.SensorName,
		&rule.MeasurementTypeId,
		&rule.MeasurementType,
		&rule.AlertType,
		&rule.HighThreshold,
		&rule.LowThreshold,
		&triggerStatus,
		&rule.Enabled,
		&rule.RateLimitSeconds,
		&lastAlertSentAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("alert rule not found for sensor: %s", sensorName)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get alert rule by sensor name: %w", err)
	}

	if triggerStatus.Valid {
		rule.TriggerStatus = triggerStatus.String
	}
	if lastAlertSentAt.Valid {
		rule.LastAlertSentAt = &lastAlertSentAt.Time
	}
	return &rule, nil
}

func (r *AlertRepositoryImpl) CreateAlertRule(ctx context.Context, rule *alerting.AlertRule) error {
	query := `
		INSERT INTO sensor_alert_rules 
		(sensor_id, measurement_type_id, alert_type, high_threshold, low_threshold, trigger_status, rate_limit_seconds, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Writer.ExecContext(ctx, query,
		rule.SensorID,
		rule.MeasurementTypeId,
		rule.AlertType,
		rule.HighThreshold,
		rule.LowThreshold,
		rule.TriggerStatus,
		rule.RateLimitSeconds,
		rule.Enabled,
	)

	if err != nil {
		return fmt.Errorf("failed to create alert rule: %w", err)
	}

	return nil
}

func (r *AlertRepositoryImpl) UpdateAlertRule(ctx context.Context, rule *alerting.AlertRule) error {
	query := `
		UPDATE sensor_alert_rules
		SET alert_type = ?,
			high_threshold = ?,
			low_threshold = ?,
			trigger_status = ?,
			rate_limit_seconds = ?,
			enabled = ?
		WHERE id = ?
	`

	_, err := r.db.Writer.ExecContext(ctx, query,
		rule.AlertType,
		rule.HighThreshold,
		rule.LowThreshold,
		rule.TriggerStatus,
		rule.RateLimitSeconds,
		rule.Enabled,
		rule.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update alert rule: %w", err)
	}

	return nil
}

func (r *AlertRepositoryImpl) DeleteAlertRule(ctx context.Context, ruleID int) error {
	query := `DELETE FROM sensor_alert_rules WHERE id = ?`
	_, err := r.db.Writer.ExecContext(ctx, query, ruleID)
	if err != nil {
		return fmt.Errorf("failed to delete alert rule: %w", err)
	}
	return nil
}

func (r *AlertRepositoryImpl) GetAlertHistory(ctx context.Context, sensorID int, limit int) ([]gen.AlertHistoryEntry, error) {
	query := `
		SELECT 
			ash.id, 
			ash.sensor_id, 
			sar.alert_type, 
			ash.reading_value, 
			ash.sent_at
		FROM alert_sent_history ash
		JOIN sensor_alert_rules sar ON ash.alert_rule_id = sar.id
		WHERE ash.sensor_id = ?
		ORDER BY ash.sent_at DESC
		LIMIT ?
	`

	rows, err := r.db.Reader.QueryContext(ctx, query, sensorID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get alert history: %w", err)
	}
	defer rows.Close()

	var history []gen.AlertHistoryEntry
	for rows.Next() {
		var entry gen.AlertHistoryEntry
		var readingValue sql.NullFloat64
		var sentAt SQLiteTime
		err := rows.Scan(&entry.Id, &entry.SensorId, &entry.AlertType, &readingValue, &sentAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan alert history entry: %w", err)
		}
		entry.SentAt = sentAt.Time
		if err != nil {
			return nil, fmt.Errorf("failed to scan alert history entry: %w", err)
		}
		if readingValue.Valid {
			entry.ReadingValue = fmt.Sprintf("%.2f", readingValue.Float64)
		}
		history = append(history, entry)
	}

	return history, nil
}

func (r *AlertRepositoryImpl) DeleteAlertHistoryOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := r.db.Writer.ExecContext(ctx, "DELETE FROM alert_sent_history WHERE sent_at < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old alert history: %w", err)
	}
	return result.RowsAffected()
}
