package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// Runs no migrations, so the copy holds the schema the database has rather
// than the one this binary would apply. VACUUM INTO reads one consistent
// snapshot, so the server can keep running.
func Backup(ctx context.Context, dbPath, dest string) error {
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s already exists; refusing to overwrite it", dest)
	}
	if err != nil {
		return fmt.Errorf("could not create backup file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("could not create backup file: %w", err)
	}

	if err := vacuumInto(ctx, dbPath, dest); err != nil {
		_ = os.Remove(dest)
		return err
	}
	return nil
}

func vacuumInto(ctx context.Context, dbPath, dest string) error {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(5000)", dbPath))
	if err != nil {
		return fmt.Errorf("could not open database: %w", err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", dest); err != nil {
		return fmt.Errorf("could not back up %s: %w", dbPath, err)
	}
	return nil
}
