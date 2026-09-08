package database

import (
	"context"
	"database/sql"
	"log/slog"
	"testing"
	"time"

	gen "example/sensorHub/gen"

	"github.com/golang-migrate/migrate/v4"
	sqlite_migrate "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthHistoryMigration_RemovesConsecutiveDuplicates(t *testing.T) {
	db := newInMemoryDB(t)
	m := newTestMigrator(t, db)

	require.NoError(t, m.Migrate(18))

	repo := NewSensorRepository(handles(db), slog.Default())
	ctx := context.Background()
	require.NoError(t, repo.AddSensor(ctx, gen.Sensor{
		Name:         "migration-sensor",
		SensorDriver: "sensor-hub-http-temperature",
	}))

	sensorID, err := repo.GetSensorIdByName(ctx, "migration-sensor")
	require.NoError(t, err)

	const sqliteDateTime = "2006-01-02 15:04:05"
	t1 := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	t2 := t1.Add(1 * time.Hour)
	t3 := t2.Add(1 * time.Hour)
	t4 := t3.Add(1 * time.Hour)
	t5 := t4.Add(1 * time.Hour)

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO sensor_health_history (sensor_id, health_status, recorded_at)
		 VALUES (?, ?, ?), (?, ?, ?), (?, ?, ?), (?, ?, ?), (?, ?, ?)`,
		sensorID, gen.Good, t1.Format(sqliteDateTime),
		sensorID, gen.Good, t2.Format(sqliteDateTime),
		sensorID, gen.Bad, t3.Format(sqliteDateTime),
		sensorID, gen.Bad, t4.Format(sqliteDateTime),
		sensorID, gen.Good, t5.Format(sqliteDateTime),
	)
	require.NoError(t, err)

	require.NoError(t, m.Up())

	history, err := repo.GetSensorHealthHistoryById(ctx, sensorID, t1)
	require.NoError(t, err)
	require.Len(t, history, 3)

	assert.Equal(t, []gen.SensorHealthStatus{gen.Good, gen.Bad, gen.Good}, []gen.SensorHealthStatus{
		history[0].HealthStatus,
		history[1].HealthStatus,
		history[2].HealthStatus,
	})
	assert.Equal(t, []time.Time{t5, t3, t1}, []time.Time{
		history[0].RecordedAt.UTC(),
		history[1].RecordedAt.UTC(),
		history[2].RecordedAt.UTC(),
	})
}

func newTestMigrator(t *testing.T, db *sql.DB) *migrate.Migrate {
	t.Helper()

	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	require.NoError(t, err)

	dbDriver, err := sqlite_migrate.WithInstance(db, &sqlite_migrate.Config{
		NoTxWrap: true,
	})
	require.NoError(t, err)

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite", dbDriver)
	require.NoError(t, err)

	t.Cleanup(func() {
		sourceErr, dbErr := m.Close()
		require.NoError(t, sourceErr)
		require.NoError(t, dbErr)
	})

	return m
}
