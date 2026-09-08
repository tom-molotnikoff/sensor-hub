package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	gen "example/sensorHub/gen"
)

type PendingCommandRecord struct {
	ID                int
	SensorID          int
	Property          string
	Value             string
	Status            string
	TimeoutSeconds    int
	SentAt            time.Time
	AcknowledgedAt    *time.Time
	AcknowledgedValue *string
}

type SensorCommandHistoryRepository struct {
	db     *Handles
	logger *slog.Logger
}

func NewSensorCommandHistoryRepository(db *Handles, logger *slog.Logger) *SensorCommandHistoryRepository {
	return &SensorCommandHistoryRepository{db: db, logger: logger.With("component", "sensor_command_history_repository")}
}

func (r *SensorCommandHistoryRepository) AddSentCommand(ctx context.Context, sensorID int, userID *int, property string, value string, mqttTopic string, mqttPayload string, timeoutSeconds int, sentAt time.Time) (int, error) {
	var userIDValue any
	if userID != nil {
		userIDValue = *userID
	}

	query := `INSERT INTO sensor_command_history
		(sensor_id, user_id, property, value, status, mqtt_topic, mqtt_payload, timeout_seconds, sent_at)
		VALUES (?, ?, ?, ?, 'sent', ?, ?, ?, ?)`
	result, err := r.db.Writer.ExecContext(ctx, query, sensorID, userIDValue, property, value, mqttTopic, mqttPayload, timeoutSeconds, sentAt)
	if err != nil {
		return 0, fmt.Errorf("error inserting sensor command history: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("error reading sensor command history insert id: %w", err)
	}

	return int(id), nil
}

func (r *SensorCommandHistoryRepository) HasPendingCommand(ctx context.Context, sensorID int, property string) (bool, error) {
	query := `SELECT COUNT(1) FROM sensor_command_history WHERE sensor_id = ? AND property = ? AND status = 'sent'`
	var count int
	if err := r.db.Reader.QueryRowContext(ctx, query, sensorID, property).Scan(&count); err != nil {
		return false, fmt.Errorf("error querying pending sensor commands: %w", err)
	}
	return count > 0, nil
}

func (r *SensorCommandHistoryRepository) MarkAcknowledged(ctx context.Context, id int, acknowledgedValue string, acknowledgedAt time.Time) (bool, error) {
	query := `UPDATE sensor_command_history
		SET status = 'acknowledged', acknowledged_at = ?, acknowledged_value = ?
		WHERE id = ? AND status = 'sent'`
	result, err := r.db.Writer.ExecContext(ctx, query, acknowledgedAt, acknowledgedValue, id)
	if err != nil {
		return false, fmt.Errorf("error updating acknowledged command status: %w", err)
	}
	return rowsAffected(result)
}

func (r *SensorCommandHistoryRepository) MarkTimedOut(ctx context.Context, id int) (bool, error) {
	query := `UPDATE sensor_command_history
		SET status = 'timed_out'
		WHERE id = ? AND status = 'sent'`
	result, err := r.db.Writer.ExecContext(ctx, query, id)
	if err != nil {
		return false, fmt.Errorf("error updating timed out command status: %w", err)
	}
	return rowsAffected(result)
}

func (r *SensorCommandHistoryRepository) MarkFailed(ctx context.Context, id int) (bool, error) {
	query := `UPDATE sensor_command_history
		SET status = 'failed'
		WHERE id = ? AND status = 'sent'`
	result, err := r.db.Writer.ExecContext(ctx, query, id)
	if err != nil {
		return false, fmt.Errorf("error updating failed command status: %w", err)
	}
	return rowsAffected(result)
}

func (r *SensorCommandHistoryRepository) ListPendingCommands(ctx context.Context) ([]PendingCommandRecord, error) {
	query := `SELECT id, sensor_id, property, value, status, timeout_seconds, sent_at, acknowledged_at, acknowledged_value
		FROM sensor_command_history
		WHERE status = 'sent'
		ORDER BY sent_at ASC, id ASC`
	rows, err := r.db.Reader.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying pending sensor commands: %w", err)
	}
	defer rows.Close()

	commands := make([]PendingCommandRecord, 0)
	for rows.Next() {
		var command PendingCommandRecord
		var acknowledgedAt sql.NullTime
		var acknowledgedValue sql.NullString
		if err := rows.Scan(
			&command.ID,
			&command.SensorID,
			&command.Property,
			&command.Value,
			&command.Status,
			&command.TimeoutSeconds,
			&command.SentAt,
			&acknowledgedAt,
			&acknowledgedValue,
		); err != nil {
			return nil, fmt.Errorf("error scanning pending sensor command: %w", err)
		}
		if acknowledgedAt.Valid {
			command.AcknowledgedAt = &acknowledgedAt.Time
		}
		if acknowledgedValue.Valid {
			value := acknowledgedValue.String
			command.AcknowledgedValue = &value
		}
		commands = append(commands, command)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pending sensor commands: %w", err)
	}
	return commands, nil
}

func (r *SensorCommandHistoryRepository) ListBySensorID(ctx context.Context, sensorID int, limit int) ([]gen.CommandHistoryEntry, error) {
	query := `SELECT h.id, h.property, h.value, h.status, h.sent_at, h.acknowledged_at, h.acknowledged_value,
			h.timeout_seconds, h.mqtt_topic, h.mqtt_payload, u.id, u.username
		FROM sensor_command_history h
		LEFT JOIN users u ON u.id = h.user_id
		WHERE h.sensor_id = ?
		ORDER BY h.sent_at DESC, h.id DESC
		LIMIT ?`
	rows, err := r.db.Reader.QueryContext(ctx, query, sensorID, limit)
	if err != nil {
		return nil, fmt.Errorf("error querying sensor command history: %w", err)
	}
	defer rows.Close()

	history := make([]gen.CommandHistoryEntry, 0)
	for rows.Next() {
		var entry gen.CommandHistoryEntry
		var status string
		var acknowledgedAt sql.NullTime
		var acknowledgedValue sql.NullString
		var userID sql.NullInt64
		var username sql.NullString
		if err := rows.Scan(
			&entry.Id,
			&entry.Property,
			&entry.Value,
			&status,
			&entry.SentAt,
			&acknowledgedAt,
			&acknowledgedValue,
			&entry.TimeoutSeconds,
			&entry.MqttTopic,
			&entry.MqttPayload,
			&userID,
			&username,
		); err != nil {
			return nil, fmt.Errorf("error scanning sensor command history row: %w", err)
		}
		entry.Status = gen.CommandHistoryEntryStatus(status)
		if acknowledgedAt.Valid {
			entry.AcknowledgedAt = &acknowledgedAt.Time
		}
		if acknowledgedValue.Valid {
			value := acknowledgedValue.String
			entry.AcknowledgedValue = &value
		}
		if userID.Valid {
			entry.User = &gen.CommandHistoryUser{
				Id: int(userID.Int64),
			}
			if username.Valid {
				entry.User.Username = username.String
			}
		}
		history = append(history, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sensor command history rows: %w", err)
	}
	return history, nil
}

func rowsAffected(result sql.Result) (bool, error) {
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("error reading affected rows: %w", err)
	}
	return affected > 0, nil
}
