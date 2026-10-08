package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// EmailSettings is the one email_settings row: the SMTP server the hub sends
// through and how its last send went. The password is in the secret store.
type EmailSettings struct {
	Host        string
	Port        int
	Security    string
	Username    string
	FromAddress string
	// LastError is the error the last send failed with, nil when it
	// succeeded or none was tried.
	LastError *string
	// LastSentAt is when a send last succeeded, nil if none has.
	LastSentAt *time.Time
}

type EmailSettingsRepository struct {
	db *Handles
}

func NewEmailSettingsRepository(db *Handles) *EmailSettingsRepository {
	return &EmailSettingsRepository{db: db}
}

func (r *EmailSettingsRepository) Get(ctx context.Context) (EmailSettings, error) {
	var (
		s                                    EmailSettings
		host, username, fromAddress, lastErr sql.NullString
		port                                 sql.NullInt64
		lastSentAt                           NullSQLiteTime
	)
	err := r.db.Reader.QueryRowContext(ctx,
		`SELECT host, port, security, username, from_address, last_error, last_sent_at FROM email_settings WHERE id = 1`,
	).Scan(&host, &port, &s.Security, &username, &fromAddress, &lastErr, &lastSentAt)
	if err != nil {
		return EmailSettings{}, fmt.Errorf("error reading the email settings: %w", err)
	}
	s.Host, s.Port, s.Username, s.FromAddress = host.String, int(port.Int64), username.String, fromAddress.String
	if lastErr.Valid {
		s.LastError = &lastErr.String
	}
	if lastSentAt.Valid {
		s.LastSentAt = &lastSentAt.Time
	}
	return s, nil
}

// Save replaces the settings. It leaves the record of the last send alone.
func (r *EmailSettingsRepository) Save(ctx context.Context, s EmailSettings) error {
	_, err := r.db.Writer.ExecContext(ctx,
		`UPDATE email_settings SET host = ?, port = ?, security = ?, username = ?, from_address = ?, updated_at = datetime('now')
		 WHERE id = 1`,
		s.Host, s.Port, s.Security, s.Username, s.FromAddress)
	if err != nil {
		return fmt.Errorf("error saving the email settings: %w", err)
	}
	return nil
}

// RecordSent records a send that succeeded at the given time, clearing the
// last error.
func (r *EmailSettingsRepository) RecordSent(ctx context.Context, at time.Time) error {
	_, err := r.db.Writer.ExecContext(ctx,
		`UPDATE email_settings SET last_error = NULL, last_sent_at = ? WHERE id = 1`, SQLiteTime{Time: at.UTC()})
	if err != nil {
		return fmt.Errorf("error recording a sent email: %w", err)
	}
	return nil
}

// RecordFailure records the error a send failed with, keeping when the last
// one succeeded.
func (r *EmailSettingsRepository) RecordFailure(ctx context.Context, sendErr string) error {
	_, err := r.db.Writer.ExecContext(ctx, `UPDATE email_settings SET last_error = ? WHERE id = 1`, sendErr)
	if err != nil {
		return fmt.Errorf("error recording a failed email: %w", err)
	}
	return nil
}

// SeedFromSMTPUser finishes the email settings a 1.5.x upgrade created. The
// old smtp.user property was both the Gmail login and the sender, so when it
// is non-empty it becomes the username and the from address. It runs once, on
// a row nothing has written since the migration created it, and reports
// whether it did.
func (r *EmailSettingsRepository) SeedFromSMTPUser(ctx context.Context, smtpUser string) (bool, error) {
	var result sql.Result
	var err error
	if smtpUser == "" {
		result, err = r.db.Writer.ExecContext(ctx,
			`UPDATE email_settings SET updated_at = datetime('now') WHERE id = 1 AND updated_at IS NULL`)
	} else {
		result, err = r.db.Writer.ExecContext(ctx,
			`UPDATE email_settings SET username = ?, from_address = ?, updated_at = datetime('now') WHERE id = 1 AND updated_at IS NULL`,
			smtpUser, smtpUser)
	}
	if err != nil {
		return false, fmt.Errorf("error seeding the email settings: %w", err)
	}
	seeded, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("error seeding the email settings: %w", err)
	}
	return seeded > 0, nil
}
