package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// BrokerPasswordSecret is the name an outbound broker's password is stored
// under, with BrokerSecretOwner as its owner.
const BrokerPasswordSecret = "password"

// BrokerSecretOwner is the secrets owner for the outbound MQTT broker with the
// given id.
func BrokerSecretOwner(brokerID int) string {
	return brokerSecretOwnerPrefix + strconv.Itoa(brokerID)
}

const brokerSecretOwnerPrefix = "mqtt_broker:"

// SMTPSecretOwner and SMTPPasswordSecret name the password the hub logs in to
// its SMTP server with.
const (
	SMTPSecretOwner    = "smtp"
	SMTPPasswordSecret = "password"
)

// BrokerIDOfSecretOwner gives the broker id a BrokerSecretOwner names, and
// false for an owner that is not a broker.
func BrokerIDOfSecretOwner(owner string) (int, bool) {
	id, found := strings.CutPrefix(owner, brokerSecretOwnerPrefix)
	if !found {
		return 0, false
	}
	brokerID, err := strconv.Atoi(id)
	return brokerID, err == nil
}

// SealedSecret is a secret as the secrets table holds it: the ciphertext and
// what it takes to decrypt it with the right key.
type SealedSecret struct {
	Owner      string
	Name       string
	KeyID      int
	Nonce      []byte
	Ciphertext []byte
}

// Seal encrypts a secret's value for its owner and name.
type Seal func(owner, name string, value []byte) (SealedSecret, error)

const upsertSecretSQL = `INSERT INTO secrets (owner, name, key_id, nonce, ciphertext)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT (owner, name) DO UPDATE SET
		key_id = excluded.key_id,
		nonce = excluded.nonce,
		ciphertext = excluded.ciphertext,
		updated_at = datetime('now')`

type SecretRepository struct {
	db *Handles
}

func NewSecretRepository(db *Handles) *SecretRepository {
	return &SecretRepository{db: db}
}

// Put stores the secret, replacing any held under the same owner and name.
func (r *SecretRepository) Put(ctx context.Context, s SealedSecret) error {
	if _, err := r.db.Writer.ExecContext(ctx, upsertSecretSQL, s.Owner, s.Name, s.KeyID, s.Nonce, s.Ciphertext); err != nil {
		return fmt.Errorf("error storing secret %s/%s: %w", s.Owner, s.Name, err)
	}
	return nil
}

// Get returns the secret, or nil when none is stored.
func (r *SecretRepository) Get(ctx context.Context, owner, name string) (*SealedSecret, error) {
	s := SealedSecret{Owner: owner, Name: name}
	err := r.db.Reader.QueryRowContext(ctx,
		"SELECT key_id, nonce, ciphertext FROM secrets WHERE owner = ? AND name = ?", owner, name,
	).Scan(&s.KeyID, &s.Nonce, &s.Ciphertext)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error reading secret %s/%s: %w", owner, name, err)
	}
	return &s, nil
}

func (r *SecretRepository) Delete(ctx context.Context, owner, name string) error {
	if _, err := r.db.Writer.ExecContext(ctx, "DELETE FROM secrets WHERE owner = ? AND name = ?", owner, name); err != nil {
		return fmt.Errorf("error deleting secret %s/%s: %w", owner, name, err)
	}
	return nil
}

func (r *SecretRepository) All(ctx context.Context) ([]SealedSecret, error) {
	rows, err := r.db.Reader.QueryContext(ctx, "SELECT owner, name, key_id, nonce, ciphertext FROM secrets ORDER BY owner, name")
	if err != nil {
		return nil, fmt.Errorf("error listing secrets: %w", err)
	}
	defer rows.Close()

	var secrets []SealedSecret
	for rows.Next() {
		var s SealedSecret
		if err := rows.Scan(&s.Owner, &s.Name, &s.KeyID, &s.Nonce, &s.Ciphertext); err != nil {
			return nil, fmt.Errorf("error scanning secret: %w", err)
		}
		secrets = append(secrets, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error listing secrets: %w", err)
	}
	return secrets, nil
}
