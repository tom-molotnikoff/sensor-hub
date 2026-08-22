package database

import (
	"database/sql"
	appProps "example/sensorHub/application_properties"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/XSAM/otelsql"
	"github.com/golang-migrate/migrate/v4"
	sqlite_migrate "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	_ "modernc.org/sqlite"
)

func InitialiseDatabase(logger *slog.Logger) (*sql.DB, error) {
	if appProps.AppConfig() == nil {
		return nil, fmt.Errorf("application configuration not loaded")
	}

	dbPath := appProps.AppConfig().DatabasePath

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("could not create database directory: %w", err)
	}

	driverName, err := otelsql.Register("sqlite",
		otelsql.WithAttributes(semconv.DBSystemSqlite),
	)
	if err != nil {
		return nil, fmt.Errorf("could not register instrumented driver: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", dbPath)
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("could not open database: %w", err)
	}

	// SQLite performs best with a single writer connection
	db.SetMaxOpenConns(1)

	if _, err := otelsql.RegisterDBStatsMetrics(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("could not register DB stats metrics: %w", err)
	}

	if err := runMigrations(db, logger); err != nil {
		db.Close()
		return nil, fmt.Errorf("could not run migrations: %w", err)
	}

	logger.Info("connected to database")
	return db, nil
}

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

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration failed: %w", err)
	}

	version, dirty, _ := m.Version()
	if dirty {
		return fmt.Errorf("database migration state is dirty at version %d", version)
	}

	logger.Info("database schema version", "version", version)
	return nil
}
