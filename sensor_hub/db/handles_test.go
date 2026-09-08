package database

import (
	"context"
	appProps "example/sensorHub/application_properties"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func openHandles(t *testing.T, readerConnections int) *Handles {
	t.Helper()
	h, err := Open(&appProps.ApplicationConfiguration{
		DatabasePath:              filepath.Join(t.TempDir(), "test.db"),
		DatabaseReaderConnections: readerConnections,
	}, slog.Default())
	require.NoError(t, err)
	t.Cleanup(func() { h.Close() })
	return h
}

func TestOpen_ReadDoesNotWaitForAnOpenWriteTransaction(t *testing.T) {
	h := openHandles(t, 4)
	ctx := context.Background()

	_, err := h.Writer.ExecContext(ctx, "INSERT INTO sensors (name, sensor_driver, config) VALUES ('office', 'sensor-hub-http-temperature', '{}')")
	require.NoError(t, err)

	tx, err := h.Writer.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, "INSERT INTO sensors (name, sensor_driver, config) VALUES ('attic', 'sensor-hub-http-temperature', '{}')")
	require.NoError(t, err)

	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var count int
	start := time.Now()
	err = h.Reader.QueryRowContext(readCtx, "SELECT COUNT(*) FROM sensors").Scan(&count)

	require.NoError(t, err, "the reader must not wait on the open write transaction")
	assert.Equal(t, 1, count, "the reader sees the committed state, not the open transaction")
	assert.Less(t, time.Since(start), time.Second, "the read returns immediately rather than blocking on the writer")
}

func TestOpen_WriteThroughTheReaderFailsReadOnly(t *testing.T) {
	h := openHandles(t, 4)

	_, err := h.Reader.ExecContext(context.Background(),
		"INSERT INTO sensors (name, sensor_driver, config) VALUES ('office', 'sensor-hub-http-temperature', '{}')")

	require.Error(t, err)
	var sqliteErr *sqlite.Error
	require.ErrorAs(t, err, &sqliteErr, "the failure is SQLite's own error")
	assert.Equal(t, sqlite3.SQLITE_READONLY, sqliteErr.Code(), "the reader pool refuses writes at the SQLite level")
}

func TestOpen_BothHandlesWaitFiveSecondsForALock(t *testing.T) {
	h := openHandles(t, 4)
	ctx := context.Background()

	var readerTimeout, writerTimeout int
	require.NoError(t, h.Reader.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&readerTimeout))
	require.NoError(t, h.Writer.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&writerTimeout))

	assert.Equal(t, 5000, readerTimeout)
	assert.Equal(t, 5000, writerTimeout)
}

func TestOpen_ReaderPoolUsesTheConfiguredConnectionCount(t *testing.T) {
	assert.Equal(t, 4, openHandles(t, 4).Reader.Stats().MaxOpenConnections)
	assert.Equal(t, 2, openHandles(t, 2).Reader.Stats().MaxOpenConnections)
}

func TestOpen_WriterPoolHasASingleConnection(t *testing.T) {
	assert.Equal(t, 1, openHandles(t, 4).Writer.Stats().MaxOpenConnections)
}

func TestOpen_RejectsANonPositiveReaderConnectionCount(t *testing.T) {
	for _, connections := range []int{0, -1} {
		_, err := Open(&appProps.ApplicationConfiguration{
			DatabasePath:              filepath.Join(t.TempDir(), "test.db"),
			DatabaseReaderConnections: connections,
		}, slog.Default())
		assert.Error(t, err, "reader connections of %d is rejected", connections)
	}
}

func TestOpen_ConnectionMaxOpenIsReportedPerPool(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() { otel.SetMeterProvider(previous) })

	openHandles(t, 3)

	var collected metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &collected))

	byPool := map[string]int64{}
	for _, scope := range collected.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name != "db.sql.connection.max_open" {
				continue
			}
			gauge, ok := metric.Data.(metricdata.Gauge[int64])
			require.True(t, ok)
			for _, point := range gauge.DataPoints {
				pool, found := point.Attributes.Value(attribute.Key("pool"))
				require.True(t, found, "every pool series carries a pool attribute")
				byPool[pool.AsString()] = point.Value
			}
		}
	}

	assert.Equal(t, map[string]int64{PoolWriter: 1, PoolReader: 3}, byPool)
}
