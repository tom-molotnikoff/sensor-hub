package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

var (
	ErrMQTTClientNotFound  = errors.New("no MQTT client found")
	ErrMQTTClientNameTaken = errors.New("an MQTT client with that name already exists")
)

// MQTTClient is a credential a device uses to dial in to the embedded broker.
// Only the password's hash is stored.
type MQTTClient struct {
	ID              int
	Name            string
	TopicPrefix     string
	PasswordHash    string
	Enabled         bool
	LastConnectedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type MQTTClientRepository struct {
	db     *Handles
	logger *slog.Logger
}

func NewMQTTClientRepository(db *Handles, logger *slog.Logger) *MQTTClientRepository {
	return &MQTTClientRepository{db: db, logger: logger.With("component", "mqtt_client_repository")}
}

const mqttClientColumns = `id, name, topic_prefix, password_hash, enabled, last_connected_at, created_at, updated_at`

func (r *MQTTClientRepository) Add(ctx context.Context, name, topicPrefix, passwordHash string, enabled bool) (int, error) {
	result, err := r.db.Writer.ExecContext(ctx,
		`INSERT INTO mqtt_clients (name, topic_prefix, password_hash, enabled) VALUES (?, ?, ?, ?)`,
		name, topicPrefix, passwordHash, enabled)
	if err != nil {
		return 0, mqttClientWriteError("adding", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("error getting last insert id for MQTT client: %w", err)
	}
	return int(id), nil
}

func (r *MQTTClientRepository) GetByID(ctx context.Context, id int) (MQTTClient, error) {
	return r.getOne(ctx, `SELECT `+mqttClientColumns+` FROM mqtt_clients WHERE id = ?`, id)
}

func (r *MQTTClientRepository) GetByName(ctx context.Context, name string) (MQTTClient, error) {
	return r.getOne(ctx, `SELECT `+mqttClientColumns+` FROM mqtt_clients WHERE name = ?`, name)
}

func (r *MQTTClientRepository) getOne(ctx context.Context, query string, arg any) (MQTTClient, error) {
	client, err := scanMQTTClientRow(r.db.Reader.QueryRowContext(ctx, query, arg))
	if errors.Is(err, sql.ErrNoRows) {
		return MQTTClient{}, ErrMQTTClientNotFound
	}
	if err != nil {
		return MQTTClient{}, fmt.Errorf("error querying MQTT client: %w", err)
	}
	return client, nil
}

func (r *MQTTClientRepository) GetAll(ctx context.Context) ([]MQTTClient, error) {
	rows, err := r.db.Reader.QueryContext(ctx, `SELECT `+mqttClientColumns+` FROM mqtt_clients ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("error querying MQTT clients: %w", err)
	}
	defer rows.Close()

	clients := []MQTTClient{}
	for rows.Next() {
		client, err := scanMQTTClientRow(rows)
		if err != nil {
			return nil, fmt.Errorf("error scanning MQTT client row: %w", err)
		}
		clients = append(clients, client)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over MQTT client rows: %w", err)
	}
	return clients, nil
}

func (r *MQTTClientRepository) Update(ctx context.Context, id int, name, topicPrefix string, enabled bool) error {
	result, err := r.db.Writer.ExecContext(ctx,
		`UPDATE mqtt_clients SET name = ?, topic_prefix = ?, enabled = ?, updated_at = datetime('now') WHERE id = ?`,
		name, topicPrefix, enabled, id)
	return requireMQTTClientRow(result, mqttClientWriteError("updating", err))
}

func (r *MQTTClientRepository) SetPasswordHash(ctx context.Context, id int, passwordHash string) error {
	result, err := r.db.Writer.ExecContext(ctx,
		`UPDATE mqtt_clients SET password_hash = ?, updated_at = datetime('now') WHERE id = ?`, passwordHash, id)
	return requireMQTTClientRow(result, mqttClientWriteError("rotating the password of", err))
}

func (r *MQTTClientRepository) RecordConnected(ctx context.Context, id int) error {
	_, err := r.db.Writer.ExecContext(ctx, `UPDATE mqtt_clients SET last_connected_at = datetime('now') WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("error recording MQTT client connection: %w", err)
	}
	return nil
}

func (r *MQTTClientRepository) Delete(ctx context.Context, id int) error {
	result, err := r.db.Writer.ExecContext(ctx, `DELETE FROM mqtt_clients WHERE id = ?`, id)
	return requireMQTTClientRow(result, mqttClientWriteError("deleting", err))
}

func mqttClientWriteError(action string, err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return ErrMQTTClientNameTaken
	}
	return fmt.Errorf("error %s MQTT client: %w", action, err)
}

func requireMQTTClientRow(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error fetching rows affected for MQTT client: %w", err)
	}
	if affected == 0 {
		return ErrMQTTClientNotFound
	}
	return nil
}

func scanMQTTClientRow(row scannable) (MQTTClient, error) {
	var c MQTTClient
	var lastConnectedAt, createdAt, updatedAt NullSQLiteTime
	err := row.Scan(&c.ID, &c.Name, &c.TopicPrefix, &c.PasswordHash, &c.Enabled, &lastConnectedAt, &createdAt, &updatedAt)
	if err != nil {
		return c, err
	}
	c.LastConnectedAt = nullableTime(lastConnectedAt)
	c.CreatedAt = createdAt.Time
	c.UpdatedAt = updatedAt.Time
	return c, nil
}
