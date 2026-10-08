package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
)

// MoveBrokerPasswordsToSecrets finishes the 2.0 upgrade of a database that
// held outbound broker passwords in plaintext in mqtt_brokers.password. In one
// transaction it seals every password into the secrets table and nulls the
// column, then drops the column and checkpoints the WAL into the database file
// and truncates it.
//
// It runs on the writer connection with secure_delete on, so the bytes the
// plaintext occupied are overwritten with zeros rather than left in free space.
// Once the column is gone it does nothing, so a hub that crashed partway
// through finishes the job on its next start. It returns how many passwords
// it moved.
func MoveBrokerPasswordsToSecrets(ctx context.Context, h *Handles, seal Seal, logger *slog.Logger) (int, error) {
	conn, err := h.Writer.Conn(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to acquire the writer to move broker passwords: %w", err)
	}
	defer func() { _ = conn.Close() }()

	present, err := brokerPasswordColumnPresent(ctx, conn)
	if err != nil || !present {
		return 0, err
	}

	restore, err := enableSecureDelete(ctx, conn)
	if err != nil {
		return 0, err
	}
	defer restore()

	moved, err := sealBrokerPasswords(ctx, conn, seal)
	if err != nil {
		return 0, err
	}
	if _, err := conn.ExecContext(ctx, "ALTER TABLE mqtt_brokers DROP COLUMN password"); err != nil {
		return 0, fmt.Errorf("failed to drop mqtt_brokers.password: %w", err)
	}

	var busy, logPages, checkpointed int
	if err := conn.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logPages, &checkpointed); err != nil {
		return 0, fmt.Errorf("failed to checkpoint the WAL after moving broker passwords: %w", err)
	}
	if busy != 0 {
		// Nothing should be reading this early in startup. The periodic
		// checkpoint truncates the WAL later.
		logger.Warn("could not truncate the WAL after moving broker passwords; it still holds their old pages until the next checkpoint")
	}
	return moved, nil
}

func brokerPasswordColumnPresent(ctx context.Context, conn *sql.Conn) (bool, error) {
	var n int
	err := conn.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM pragma_table_info('mqtt_brokers') WHERE name = 'password'").Scan(&n)
	if err != nil {
		return false, fmt.Errorf("failed to inspect mqtt_brokers: %w", err)
	}
	return n > 0, nil
}

// enableSecureDelete turns secure_delete on for the connection and returns a
// function that puts back the setting it had.
func enableSecureDelete(ctx context.Context, conn *sql.Conn) (func(), error) {
	var previous int
	if err := conn.QueryRowContext(ctx, "PRAGMA secure_delete").Scan(&previous); err != nil {
		return nil, fmt.Errorf("failed to read secure_delete: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA secure_delete = ON"); err != nil {
		return nil, fmt.Errorf("failed to turn on secure_delete: %w", err)
	}
	return func() {
		_, _ = conn.ExecContext(context.Background(), fmt.Sprintf("PRAGMA secure_delete = %d", previous))
	}, nil
}

func sealBrokerPasswords(ctx context.Context, conn *sql.Conn, seal Seal) (int, error) {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin moving broker passwords: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	passwords, err := readBrokerPasswords(ctx, tx)
	if err != nil {
		return 0, err
	}

	moved := 0
	for _, p := range passwords {
		// An empty password meant none, as it does through the API.
		if p.password == "" {
			continue
		}
		sealed, err := seal(BrokerSecretOwner(p.id), BrokerPasswordSecret, []byte(p.password))
		if err != nil {
			return 0, fmt.Errorf("failed to encrypt the password of broker %d: %w", p.id, err)
		}
		if _, err := tx.ExecContext(ctx, upsertSecretSQL, sealed.Owner, sealed.Name, sealed.KeyID, sealed.Nonce, sealed.Ciphertext); err != nil {
			return 0, fmt.Errorf("failed to store the password of broker %d: %w", p.id, err)
		}
		moved++
	}

	if _, err := tx.ExecContext(ctx, "UPDATE mqtt_brokers SET password = NULL WHERE password IS NOT NULL"); err != nil {
		return 0, fmt.Errorf("failed to clear broker passwords: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit moving broker passwords: %w", err)
	}
	return moved, nil
}

type brokerPassword struct {
	id       int
	password string
}

func readBrokerPasswords(ctx context.Context, tx *sql.Tx) ([]brokerPassword, error) {
	rows, err := tx.QueryContext(ctx, "SELECT id, password FROM mqtt_brokers WHERE password IS NOT NULL")
	if err != nil {
		return nil, fmt.Errorf("failed to read broker passwords: %w", err)
	}
	defer rows.Close()

	var passwords []brokerPassword
	for rows.Next() {
		var p brokerPassword
		if err := rows.Scan(&p.id, &p.password); err != nil {
			return nil, fmt.Errorf("failed to read broker passwords: %w", err)
		}
		passwords = append(passwords, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read broker passwords: %w", err)
	}
	return passwords, nil
}
