package database

import (
	"database/sql"
	"errors"
	appProps "example/sensorHub/application_properties"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/XSAM/otelsql"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	_ "modernc.org/sqlite"
)

const (
	PoolReader = "reader"
	PoolWriter = "writer"

	readerDSNParams = "_pragma=busy_timeout(5000)&_pragma=query_only(1)"
	writerDSNParams = "_txlock=immediate&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
)

var poolAttributeKey = attribute.Key("pool")

type Handles struct {
	Reader *sql.DB
	Writer *sql.DB
}

func Open(cfg *appProps.ApplicationConfiguration, logger *slog.Logger) (*Handles, error) {
	return OpenWithDriver("sqlite", cfg, logger)
}

func OpenWithDriver(driverName string, cfg *appProps.ApplicationConfiguration, logger *slog.Logger) (*Handles, error) {
	if cfg == nil {
		return nil, fmt.Errorf("application configuration not loaded")
	}
	if cfg.DatabaseReaderConnections <= 0 {
		return nil, fmt.Errorf("database.reader.connections must be positive, got %d", cfg.DatabaseReaderConnections)
	}

	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0755); err != nil {
		return nil, fmt.Errorf("could not create database directory: %w", err)
	}

	writer, err := openPool(driverName, cfg.DatabasePath, PoolWriter, writerDSNParams, 1)
	if err != nil {
		return nil, err
	}

	if err := runMigrations(writer, logger); err != nil {
		writer.Close()
		return nil, fmt.Errorf("could not run migrations: %w", err)
	}

	reader, err := openPool(driverName, cfg.DatabasePath, PoolReader, readerDSNParams, cfg.DatabaseReaderConnections)
	if err != nil {
		writer.Close()
		return nil, err
	}

	logger.Info("connected to database", "reader_connections", cfg.DatabaseReaderConnections)
	return &Handles{Reader: reader, Writer: writer}, nil
}

func (h *Handles) Close() error {
	return errors.Join(h.Reader.Close(), h.Writer.Close())
}

func openPool(driverName, dbPath, pool, dsnParams string, maxOpenConns int) (*sql.DB, error) {
	attributes := otelsql.WithAttributes(semconv.DBSystemSqlite, poolAttributeKey.String(pool))

	instrumented, err := otelsql.Register(driverName, attributes)
	if err != nil {
		return nil, fmt.Errorf("could not register instrumented driver for %s pool: %w", pool, err)
	}

	db, err := sql.Open(instrumented, fmt.Sprintf("file:%s?%s", dbPath, dsnParams))
	if err != nil {
		return nil, fmt.Errorf("could not open %s pool: %w", pool, err)
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxOpenConns)

	if _, err := otelsql.RegisterDBStatsMetrics(db, attributes); err != nil {
		db.Close()
		return nil, fmt.Errorf("could not register DB stats metrics for %s pool: %w", pool, err)
	}

	return db, nil
}
