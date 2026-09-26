package main

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	"example/sensorHub/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func openTempDatabase(t *testing.T) *database.Handles {
	t.Helper()
	db, err := database.Open(&appProps.ApplicationConfiguration{
		DatabasePath:              filepath.Join(t.TempDir(), "sensor_hub.db"),
		DatabaseReaderConnections: 1,
	}, discardLogger())
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func runSeed(t *testing.T, db *database.Handles) string {
	t.Helper()
	apiKey, err := seed(context.Background(), db, discardLogger())
	require.NoError(t, err)
	return apiKey
}

type entityCounts struct {
	users, apiKeys, markers int
}

func countEntities(t *testing.T, db *database.Handles) entityCounts {
	t.Helper()
	var counts entityCounts
	require.NoError(t, db.Reader.QueryRow("SELECT COUNT(*) FROM users").Scan(&counts.users))
	require.NoError(t, db.Reader.QueryRow("SELECT COUNT(*) FROM api_keys").Scan(&counts.apiKeys))
	require.NoError(t, db.Reader.QueryRow("SELECT COUNT(*) FROM devseed_metadata WHERE name = ?", markerSeededAt).Scan(&counts.markers))
	return counts
}

func assertSeededUsersCanLogIn(t *testing.T, db *database.Handles) {
	t.Helper()
	users := database.NewUserRepository(db, discardLogger())
	for _, want := range devUsers {
		user, hash, err := users.GetUserByUsername(context.Background(), want.Username)
		require.NoError(t, err)
		require.NotNil(t, user, want.Username)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte(want.Password)), want.Username)
		assert.Equal(t, []string{want.Role}, user.Roles, want.Username)
		assert.False(t, user.MustChangePassword, want.Username)
	}
}

func assertAuthenticatesAsAdmin(t *testing.T, db *database.Handles, apiKey string) {
	t.Helper()
	logger := discardLogger()
	userRepo := database.NewUserRepository(db, logger)
	keys := service.NewApiKeyService(database.NewApiKeyRepository(db, logger), userRepo, database.NewRoleRepository(db, logger), logger)
	user, err := keys.ValidateApiKey(context.Background(), apiKey)
	require.NoError(t, err)
	require.NotNil(t, user, "the printed API key authenticates")
	assert.Equal(t, "admin", user.Username)
}

func TestSeed_FirstRunCreatesEachEntityOnceAndWritesTheMarker(t *testing.T) {
	db := openTempDatabase(t)

	apiKey := runSeed(t, db)

	assert.Equal(t, entityCounts{users: len(devUsers), apiKeys: 1, markers: 1}, countEntities(t, db))
	assertSeededUsersCanLogIn(t, db)
	assertAuthenticatesAsAdmin(t, db, apiKey)
}

func TestSeed_RerunCreatesNothingAndReturnsTheSameKey(t *testing.T) {
	db := openTempDatabase(t)
	firstKey := runSeed(t, db)
	before := countEntities(t, db)

	secondKey := runSeed(t, db)

	assert.Equal(t, before, countEntities(t, db))
	assert.Equal(t, firstKey, secondKey, "every run prints the key the first run created")
}

func TestSeed_RerunKeepsEditsToSeededUsers(t *testing.T) {
	db := openTempDatabase(t)
	runSeed(t, db)
	users := database.NewUserRepository(db, discardLogger())
	ctx := context.Background()
	viewer, _, err := users.GetUserByUsername(ctx, "viewer")
	require.NoError(t, err)
	require.NoError(t, users.DeleteUserById(ctx, viewer.Id))
	user, _, err := users.GetUserByUsername(ctx, "user")
	require.NoError(t, err)
	require.NoError(t, users.SetRolesForUser(ctx, user.Id, []string{service.RoleViewer}))

	runSeed(t, db)

	deleted, _, err := users.GetUserByUsername(ctx, "viewer")
	require.NoError(t, err)
	assert.Nil(t, deleted, "a deleted seeded user stays deleted")
	edited, _, err := users.GetUserByUsername(ctx, "user")
	require.NoError(t, err)
	assert.Equal(t, []string{service.RoleViewer}, edited.Roles, "an edited seeded user keeps the edit")
}

func TestSeed_FailedStepIsNamedAndARerunFinishesWithoutDuplicates(t *testing.T) {
	db := openTempDatabase(t)
	_, err := db.Writer.Exec(`
		CREATE TABLE devseed_metadata (name TEXT PRIMARY KEY, value TEXT NOT NULL);
		CREATE TRIGGER fail_marker BEFORE INSERT ON devseed_metadata BEGIN SELECT RAISE(ABORT, 'injected failure'); END;`)
	require.NoError(t, err)

	_, err = seed(context.Background(), db, discardLogger())

	var failed *stepError
	require.ErrorAs(t, err, &failed)
	assert.Equal(t, "write the marker", failed.step)
	assert.ErrorContains(t, failed.err, "injected failure")
	assert.Equal(t, entityCounts{users: len(devUsers), apiKeys: 1, markers: 0}, countEntities(t, db))

	_, err = db.Writer.Exec("DROP TRIGGER fail_marker")
	require.NoError(t, err)
	apiKey := runSeed(t, db)

	assert.Equal(t, entityCounts{users: len(devUsers), apiKeys: 1, markers: 1}, countEntities(t, db))
	assertSeededUsersCanLogIn(t, db)
	assertAuthenticatesAsAdmin(t, db, apiKey)
}
