package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	gen "example/sensorHub/gen"
)

type MQTTBrokerRepository struct {
	db     *Handles
	logger *slog.Logger
}

func NewMQTTBrokerRepository(db *Handles, logger *slog.Logger) *MQTTBrokerRepository {
	return &MQTTBrokerRepository{db: db, logger: logger.With("component", "mqtt_broker_repository")}
}

func (r *MQTTBrokerRepository) Add(ctx context.Context, broker gen.MQTTBroker) (int, error) {
	if broker.Name == "" {
		return 0, fmt.Errorf("broker name cannot be empty")
	}
	if broker.Type == "" {
		broker.Type = "external"
	}
	if broker.Type == "external" {
		if broker.Host == nil || *broker.Host == "" {
			return 0, fmt.Errorf("broker host cannot be empty")
		}
		if broker.Port == nil || *broker.Port <= 0 {
			port := 1883
			broker.Port = &port
		}
	}
	query := `INSERT INTO mqtt_brokers (name, type, host, port, username, client_id,
		tls, ca_cert_pem, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := r.db.Writer.ExecContext(ctx, query,
		broker.Name, broker.Type, nullStringPtr(broker.Host), broker.Port,
		nullStringPtr(broker.Username), nullStringPtr(broker.ClientId),
		brokerTLS(broker), nullStringPtr(broker.CaCertPem),
		broker.Enabled,
	)
	if err != nil {
		return 0, fmt.Errorf("error adding MQTT broker: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("error getting last insert id for MQTT broker: %w", err)
	}
	return int(id), nil
}

func (r *MQTTBrokerRepository) GetByID(ctx context.Context, id int) (*gen.MQTTBroker, error) {
	query := `SELECT id, name, type, host, port, username, client_id,
		tls, ca_cert_pem, enabled, created_at, updated_at
		FROM mqtt_brokers WHERE id = ?`
	broker, err := scanBrokerRow(r.db.Reader.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("error querying MQTT broker by id: %w", err)
	}
	return &broker, nil
}

func (r *MQTTBrokerRepository) GetByName(ctx context.Context, name string) (*gen.MQTTBroker, error) {
	query := `SELECT id, name, type, host, port, username, client_id,
		tls, ca_cert_pem, enabled, created_at, updated_at
		FROM mqtt_brokers WHERE LOWER(name) = LOWER(?)`
	broker, err := scanBrokerRow(r.db.Reader.QueryRowContext(ctx, query, name))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("error querying MQTT broker by name: %w", err)
	}
	return &broker, nil
}

func (r *MQTTBrokerRepository) GetAll(ctx context.Context) ([]gen.MQTTBroker, error) {
	query := `SELECT id, name, type, host, port, username, client_id,
		tls, ca_cert_pem, enabled, created_at, updated_at
		FROM mqtt_brokers ORDER BY name`
	rows, err := r.db.Reader.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying all MQTT brokers: %w", err)
	}
	defer rows.Close()

	var brokers []gen.MQTTBroker
	for rows.Next() {
		broker, err := scanBrokerRow(rows)
		if err != nil {
			return nil, fmt.Errorf("error scanning MQTT broker row: %w", err)
		}
		brokers = append(brokers, broker)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over MQTT broker rows: %w", err)
	}
	return brokers, nil
}

func (r *MQTTBrokerRepository) Update(ctx context.Context, broker gen.MQTTBroker) error {
	query := `UPDATE mqtt_brokers SET name = ?, type = ?, host = ?, port = ?,
		username = ?, client_id = ?,
		tls = ?, ca_cert_pem = ?,
		enabled = ?, updated_at = datetime('now') WHERE id = ?`
	result, err := r.db.Writer.ExecContext(ctx, query,
		broker.Name, broker.Type, nullStringPtr(broker.Host), broker.Port,
		nullStringPtr(broker.Username), nullStringPtr(broker.ClientId),
		brokerTLS(broker), nullStringPtr(broker.CaCertPem),
		broker.Enabled, *broker.Id,
	)
	if err != nil {
		return fmt.Errorf("error updating MQTT broker: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error fetching rows affected after MQTT broker update: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no MQTT broker found with id %d", *broker.Id)
	}
	return nil
}

// Delete removes the broker and its secrets in one transaction.
func (r *MQTTBrokerRepository) Delete(ctx context.Context, id int) error {
	tx, err := r.db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error deleting MQTT broker: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "DELETE FROM secrets WHERE owner = ?", BrokerSecretOwner(id)); err != nil {
		return fmt.Errorf("error deleting the secrets of MQTT broker %d: %w", id, err)
	}
	result, err := tx.ExecContext(ctx, "DELETE FROM mqtt_brokers WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("error deleting MQTT broker: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error fetching rows affected after MQTT broker delete: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no MQTT broker found with id %d", id)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error deleting MQTT broker: %w", err)
	}
	return nil
}

func (r *MQTTBrokerRepository) GetEnabled(ctx context.Context) ([]gen.MQTTBroker, error) {
	query := `SELECT id, name, type, host, port, username, client_id,
		tls, ca_cert_pem, enabled, created_at, updated_at
		FROM mqtt_brokers WHERE enabled = 1 ORDER BY name`
	rows, err := r.db.Reader.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying enabled MQTT brokers: %w", err)
	}
	defer rows.Close()

	var brokers []gen.MQTTBroker
	for rows.Next() {
		broker, err := scanBrokerRow(rows)
		if err != nil {
			return nil, fmt.Errorf("error scanning MQTT broker row: %w", err)
		}
		brokers = append(brokers, broker)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over MQTT broker rows: %w", err)
	}
	return brokers, nil
}

// brokerTLS reads the broker's TLS switch, which is off when omitted.
func brokerTLS(broker gen.MQTTBroker) bool {
	return broker.Tls != nil && *broker.Tls
}

// nullString converts an empty string to a sql.NullString for nullable TEXT columns.
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// nullStringPtr converts a *string to a sql.NullString for nullable TEXT columns.
func nullStringPtr(s *string) sql.NullString {
	if s == nil || *s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func scanBrokerRow(row scannable) (gen.MQTTBroker, error) {
	var b gen.MQTTBroker
	var id int
	var host, username, clientId sql.NullString
	var port sql.NullInt64
	var tls bool
	var caCert sql.NullString
	var createdAt, updatedAt NullSQLiteTime
	err := row.Scan(
		&id, &b.Name, &b.Type, &host, &port,
		&username, &clientId,
		&tls, &caCert,
		&b.Enabled, &createdAt, &updatedAt,
	)
	if err != nil {
		return b, err
	}
	b.Id = &id
	if host.Valid {
		b.Host = &host.String
	}
	b.Port = nullableInt(port)
	if username.Valid {
		b.Username = &username.String
	}
	if clientId.Valid {
		b.ClientId = &clientId.String
	}
	b.Tls = &tls
	if caCert.Valid {
		b.CaCertPem = &caCert.String
	}
	if createdAt.Valid {
		b.CreatedAt = &createdAt.Time
	}
	if updatedAt.Valid {
		b.UpdatedAt = &updatedAt.Time
	}
	return b, nil
}
