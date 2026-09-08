package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-migrate/migrate/v4"
	sqlite_migrate "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// RunMigrations applies all pending migrations to the given database.
// Exported so test packages can set up in-memory SQLite databases with the correct schema.
func RunMigrations(db *sql.DB, logger *slog.Logger) error {
	return runMigrations(db, logger)
}

func runMigrations(db *sql.DB, logger *slog.Logger) error {
	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("could not create migration source: %w", err)
	}

	dbDriver, err := sqlite_migrate.WithInstance(db, &sqlite_migrate.Config{
		NoTxWrap: true,
	})
	if err != nil {
		return fmt.Errorf("could not create migration db driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite", dbDriver)
	if err != nil {
		return fmt.Errorf("could not create migrator: %w", err)
	}

	started := time.Now()
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration failed: %w", err)
	}
	elapsed := time.Since(started)

	version, dirty, _ := m.Version()
	if dirty {
		return fmt.Errorf("database migration state is dirty at version %d", version)
	}

	logger.Info("database schema version", "version", version, "migration_duration_ms", elapsed.Milliseconds())
	return nil
}
