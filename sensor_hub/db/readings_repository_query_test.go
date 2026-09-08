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
	require.NoError(t, newTestMigrator(t, db).Migrate(22))

	ctx := context.Background()
	sensorRepo := NewSensorRepository(handles(db), slog.Default())
	require.NoError(t, sensorRepo.AddSensor(ctx, gen.Sensor{
		Name:         "Office",
		SensorDriver: "sensor-hub-http-temperature",
	}))

	mtRepo := NewMeasurementTypeRepository(handles(db), slog.Default())
	repo := NewReadingsRepository(handles(db), sensorRepo, mtRepo, slog.Default()).(*ReadingsRepositoryImpl)
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

var readingsAliases = []string{"readings", "r", "r2", "latest", "counted"}

func assertNoReadingsScan(t *testing.T, plan string) {
	t.Helper()
	for line := range strings.SplitSeq(plan, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "SCAN" {
			continue
		}
		assert.NotContains(t, readingsAliases, fields[1], "plan step scans readings: "+line)
	}
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
	assertNoReadingsScan(t, plan)
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
	assertNoReadingsScan(t, plan)
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
	assertNoReadingsScan(t, plan)
}

func TestGetBetweenDates_Raw_FiltersBySensorCaseInsensitively(t *testing.T) {
	repo, _ := migratedReadingsRepo(t) // seeds sensor "Office"
	ctx := context.Background()
	require.NoError(t, NewSensorRepository(repoHandles(repo), slog.Default()).AddSensor(ctx, gen.Sensor{
		Name:         "Attic",
		SensorDriver: "sensor-hub-http-temperature",
	}))

	v := 21.0
	require.NoError(t, repo.Ingest(ctx, ReadingBatch{SensorName: "Office", Readings: []gen.Reading{
		{SensorName: "Office", MeasurementType: "temperature", NumericValue: &v, Time: "2025-01-15 12:00:00"},
	}}))
	require.NoError(t, repo.Ingest(ctx, ReadingBatch{SensorName: "Attic", Readings: []gen.Reading{
		{SensorName: "Attic", MeasurementType: "temperature", NumericValue: &v, Time: "2025-01-15 12:00:00"},
	}}))

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

// repoHandles exposes the repository's underlying handles for test setup.
func repoHandles(r *ReadingsRepositoryImpl) *Handles { return r.db }

func TestLatestPerSeriesQuery_ProbesTheCompositeIndexOncePerPair(t *testing.T) {
	_, db := migratedReadingsRepo(t)

	plan := queryPlan(t, db, latestPerSeriesQuery())

	assert.Contains(t, plan, "CORRELATED SCALAR SUBQUERY", "one probe per pair, not one pass over readings")
	assert.Contains(t, plan, "idx_readings_sensor_type_time", "the probe uses the composite index")
	assert.Contains(t, plan, "SCAN smt", "the pairs come from the join table")
	assertNoReadingsScan(t, plan)
}

func TestTypesWithReadingsQuery_ReadsTheJoinTable(t *testing.T) {
	_, db := migratedReadingsRepo(t)

	plan := queryPlan(t, db, typesWithReadingsQuery())

	assert.Contains(t, plan, "smt", "the answer comes from the join table")
	assertNoReadingsScan(t, plan)
}

func TestSensorTypesWithReadingsQuery_ReadsTheJoinTable(t *testing.T) {
	_, db := migratedReadingsRepo(t)

	plan := queryPlan(t, db, sensorTypesWithReadingsQuery(), 1)

	assert.Contains(t, plan, "smt", "the answer comes from the join table")
	assertNoReadingsScan(t, plan)
}

func TestRetentionDeletes_DoNotScanReadings(t *testing.T) {
	_, db := migratedReadingsRepo(t)
	cutoff := "2025-01-01 00:00:00"

	global := queryPlan(t, db, deleteOlderThanQuery(), cutoff)
	assert.Contains(t, global, "idx_readings_time", "the global delete walks the time index")
	assertNoReadingsScan(t, global)

	perSensor := queryPlan(t, db, deleteOlderThanForSensorQuery(), 1, cutoff)
	assert.Contains(t, perSensor, "idx_readings_sensor_type_time", "the per-sensor delete walks the composite index")
	assertNoReadingsScan(t, perSensor)

	excluding := queryPlan(t, db, deleteOlderThanExcludingSensorsQuery(2), cutoff, 1, 2)
	assert.Contains(t, excluding, "idx_readings_time", "the excluding delete walks the time index")
	assertNoReadingsScan(t, excluding)
}

func TestCountReadingsPerSensorQuery_IsTheOneQueryThatScansReadings(t *testing.T) {
	_, db := migratedReadingsRepo(t)

	plan := queryPlan(t, db, countReadingsPerSensorQuery())

	assert.Contains(t, plan, "SCAN readings", "the sampler counts by scanning readings")
}
