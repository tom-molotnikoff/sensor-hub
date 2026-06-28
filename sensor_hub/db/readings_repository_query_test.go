package database

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"testing"

	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// migratedReadingsRepo returns a repository backed by a fully-migrated in-memory
// database with one sensor present, so name->id resolution works in queries.
func migratedReadingsRepo(t *testing.T) (*ReadingsRepositoryImpl, *sql.DB) {
	t.Helper()
	db := newInMemoryDB(t)
	require.NoError(t, newTestMigrator(t, db).Migrate(19))

	ctx := context.Background()
	require.NoError(t, NewSensorRepository(db, slog.Default()).AddSensor(ctx, gen.Sensor{
		Name:         "Office",
		SensorDriver: "sensor-hub-http-temperature",
	}))

	repo := NewReadingsRepository(db, slog.Default()).(*ReadingsRepositoryImpl)
	return repo, db
}

// queryPlan runs EXPLAIN QUERY PLAN and returns the concatenated detail lines.
func queryPlan(t *testing.T, db *sql.DB, query string, args ...any) string {
	t.Helper()
	rows, err := db.Query("EXPLAIN QUERY PLAN "+query, args...)
	require.NoError(t, err)
	defer rows.Close()

	var sb strings.Builder
	for rows.Next() {
		var id, parent, notused int
		var detail string
		require.NoError(t, rows.Scan(&id, &parent, &notused, &detail))
		sb.WriteString(detail)
		sb.WriteString("\n")
	}
	require.NoError(t, rows.Err())
	return sb.String()
}

func TestRawBetweenQuery_UsesCompositeIndex(t *testing.T) {
	repo, db := migratedReadingsRepo(t)
	ctx := context.Background()

	clause, filterArgs, resolved, err := repo.seriesFilter(ctx, "Office", "temperature")
	require.NoError(t, err)
	require.True(t, resolved)

	args := append([]any{"2025-01-01 00:00:00", "2025-02-01 00:00:00"}, filterArgs...)
	plan := queryPlan(t, db, rawBetweenQuery(clause), args...)

	assert.Contains(t, plan, "idx_readings_sensor_type_time", "should use the composite index")
	assert.NotContains(t, plan, "SCAN readings", "should not full-scan the readings table")
}

func TestAggregatedBetweenQuery_UsesCompositeIndex(t *testing.T) {
	repo, db := migratedReadingsRepo(t)
	ctx := context.Background()

	clause, filterArgs, resolved, err := repo.seriesFilter(ctx, "Office", "temperature")
	require.NoError(t, err)
	require.True(t, resolved)

	bucket, err := timeBucketExpression(AggregationPT1H)
	require.NoError(t, err)

	args := append([]any{"2025-01-01 00:00:00", "2025-02-01 00:00:00"}, filterArgs...)
	plan := queryPlan(t, db, aggregatedBetweenQuery("ROUND(AVG(r.numeric_value), 2)", bucket, clause), args...)

	assert.Contains(t, plan, "idx_readings_sensor_type_time", "should use the composite index")
	assert.NotContains(t, plan, "SCAN readings", "should not full-scan the readings table")
}

func TestLastBetweenQuery_UsesCompositeIndex(t *testing.T) {
	repo, db := migratedReadingsRepo(t)
	ctx := context.Background()

	clause, filterArgs, resolved, err := repo.seriesFilter(ctx, "Office", "temperature")
	require.NoError(t, err)
	require.True(t, resolved)

	bucket, err := timeBucketExpression(AggregationPT1H)
	require.NoError(t, err)

	args := append([]any{"2025-01-01 00:00:00", "2025-02-01 00:00:00"}, filterArgs...)
	plan := queryPlan(t, db, lastBetweenQuery(bucket, clause), args...)

	assert.Contains(t, plan, "idx_readings_sensor_type_time", "should use the composite index")
	assert.NotContains(t, plan, "SCAN readings", "should not full-scan the readings table")
}

func TestGetBetweenDates_Raw_FiltersBySensorCaseInsensitively(t *testing.T) {
	repo, _ := migratedReadingsRepo(t) // seeds sensor "Office"
	ctx := context.Background()
	require.NoError(t, NewSensorRepository(db(repo), slog.Default()).AddSensor(ctx, gen.Sensor{
		Name:         "Attic",
		SensorDriver: "sensor-hub-http-temperature",
	}))

	v := 21.0
	require.NoError(t, repo.Add(ctx, []gen.Reading{
		{SensorName: "Office", MeasurementType: "temperature", NumericValue: &v, Time: "2025-01-15 12:00:00"},
		{SensorName: "Attic", MeasurementType: "temperature", NumericValue: &v, Time: "2025-01-15 12:00:00"},
	}))

	got, err := repo.GetBetweenDates(ctx, "2025-01-01 00:00:00", "2025-02-01 00:00:00", "office", "temperature", AggregationRaw, "")
	require.NoError(t, err)
	require.Len(t, got, 1, "filters to the one matching sensor")
	assert.Equal(t, "Office", got[0].SensorName, "case-insensitive name match")
}

func TestGetBetweenDates_Raw_UnknownSensorReturnsEmpty(t *testing.T) {
	repo, _ := migratedReadingsRepo(t)
	ctx := context.Background()

	got, err := repo.GetBetweenDates(ctx, "2025-01-01 00:00:00", "2025-02-01 00:00:00", "does-not-exist", "temperature", AggregationRaw, "")
	require.NoError(t, err, "unknown sensor is not an error")
	assert.Empty(t, got, "unknown sensor yields no readings")
}

// db exposes the repository's underlying *sql.DB for test setup.
func db(r *ReadingsRepositoryImpl) *sql.DB { return r.db }
