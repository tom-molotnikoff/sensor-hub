package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sync/atomic"
	"testing"
	"time"

	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	"example/sensorHub/drivers"
	gen "example/sensorHub/gen"
	"example/sensorHub/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const testWindow = 2 * time.Hour

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

func testHTTPMocks(t *testing.T) []httpMock {
	t.Helper()
	mocks := make([]httpMock, 0, len(composeHTTPMocks))
	for _, mock := range composeHTTPMocks {
		mocks = append(mocks, httpMock{name: mock.name, url: startTemperatureServer(t, 0).URL})
	}
	return mocks
}

func startTemperatureServer(t *testing.T, unavailableFor int) *httptest.Server {
	t.Helper()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if int(calls.Add(1)) <= unavailableFor {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintf(w, `{"temperature": 20.5, "time": %q}`, time.Now().UTC().Format(time.DateTime))
	}))
	t.Cleanup(server.Close)
	return server
}

func runSeed(t *testing.T, db *database.Handles) string {
	t.Helper()
	return runSeedOver(t, db, testWindow)
}

func runSeedOver(t *testing.T, db *database.Handles, window time.Duration) string {
	t.Helper()
	apiKey, err := seed(context.Background(), db, discardLogger(), testHTTPMocks(t), window)
	require.NoError(t, err)
	return apiKey
}

func newSensorService(db *database.Handles) *service.SensorService {
	logger := discardLogger()
	sensors := database.NewSensorRepository(db, logger)
	types := database.NewMeasurementTypeRepository(db, logger)
	return service.NewSensorService(sensors, database.NewReadingsRepository(db, sensors, types, logger), types, nil, nil, nil, logger)
}

type entityCounts struct {
	users, apiKeys, sensors, subscriptions, alertRules, notifications, markers int
}

var fullySeeded = entityCounts{
	users:         len(devUsers),
	apiKeys:       1,
	sensors:       len(mqttDevices) + len(composeHTTPMocks),
	subscriptions: 1,
	alertRules:    len(seededRules),
	notifications: len(seededNotifications),
	markers:       1,
}

func countEntities(t *testing.T, db *database.Handles) entityCounts {
	t.Helper()
	var counts entityCounts
	require.NoError(t, db.Reader.QueryRow("SELECT COUNT(*) FROM users").Scan(&counts.users))
	require.NoError(t, db.Reader.QueryRow("SELECT COUNT(*) FROM api_keys").Scan(&counts.apiKeys))
	require.NoError(t, db.Reader.QueryRow("SELECT COUNT(*) FROM sensors").Scan(&counts.sensors))
	require.NoError(t, db.Reader.QueryRow("SELECT COUNT(*) FROM mqtt_subscriptions").Scan(&counts.subscriptions))
	require.NoError(t, db.Reader.QueryRow("SELECT COUNT(*) FROM sensor_alert_rules").Scan(&counts.alertRules))
	require.NoError(t, db.Reader.QueryRow("SELECT COUNT(*) FROM notifications").Scan(&counts.notifications))
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

	assert.Equal(t, fullySeeded, countEntities(t, db))
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

	_, err = seed(context.Background(), db, discardLogger(), testHTTPMocks(t), testWindow)

	var failed *stepError
	require.ErrorAs(t, err, &failed)
	assert.Equal(t, "write the marker", failed.step)
	assert.ErrorContains(t, failed.err, "injected failure")
	unmarked := fullySeeded
	unmarked.markers = 0
	assert.Equal(t, unmarked, countEntities(t, db))

	_, err = db.Writer.Exec("DROP TRIGGER fail_marker")
	require.NoError(t, err)
	apiKey := runSeed(t, db)

	assert.Equal(t, fullySeeded, countEntities(t, db))
	assertSeededUsersCanLogIn(t, db)
	assertAuthenticatesAsAdmin(t, db, apiKey)
}

func TestSeed_RerunPrintsNoKeyOnceTheSeededKeyIsRevoked(t *testing.T) {
	db := openTempDatabase(t)
	runSeed(t, db)
	_, err := db.Writer.Exec("UPDATE api_keys SET revoked = 1")
	require.NoError(t, err)

	assert.Empty(t, runSeed(t, db), "a key that no longer authenticates is not printed")
}

func TestSeed_RerunFinishesAUserAnUnfinishedRunLeftBehind(t *testing.T) {
	db := openTempDatabase(t)
	users := database.NewUserRepository(db, discardLogger())
	hash, err := bcrypt.GenerateFromPassword([]byte("adminpassword"), bcrypt.MinCost)
	require.NoError(t, err)
	_, err = users.CreateUser(context.Background(), gen.User{Username: "admin", MustChangePassword: true}, string(hash))
	require.NoError(t, err)

	runSeed(t, db)

	assert.Equal(t, fullySeeded, countEntities(t, db))
	assertSeededUsersCanLogIn(t, db)
}

func TestSeed_FirstRunApprovesEveryMockSensor(t *testing.T) {
	db := openTempDatabase(t)
	mocks := testHTTPMocks(t)

	_, err := seed(context.Background(), db, discardLogger(), mocks, testWindow)

	require.NoError(t, err)
	sensors := newSensorService(db)
	for _, want := range (&seeder{httpMocks: mocks}).devices() {
		sensor, err := sensors.ServiceGetSensorByName(context.Background(), want.Name)
		require.NoError(t, err)
		require.NotNil(t, sensor, want.Name)
		assert.Equal(t, gen.SensorStatusActive, sensor.Status, want.Name)
		assert.True(t, sensor.Enabled, want.Name)
		assert.Equal(t, want.Driver, sensor.SensorDriver, want.Name)
		assert.Equal(t, want.Config, sensor.Config, want.Name)
	}
}

func TestSeed_FirstRunSubscribesTheEmbeddedBrokerToZigbee2MQTT(t *testing.T) {
	db := openTempDatabase(t)

	runSeed(t, db)

	subscriptions, err := database.NewMQTTSubscriptionRepository(db, discardLogger()).GetAll(context.Background())
	require.NoError(t, err)
	require.Len(t, subscriptions, 1)
	broker, err := database.NewMQTTBrokerRepository(db, discardLogger()).GetByID(context.Background(), subscriptions[0].BrokerId)
	require.NoError(t, err)
	assert.Equal(t, "embedded", broker.Type)
	assert.Equal(t, "zigbee2mqtt/#", subscriptions[0].TopicPattern)
	assert.Equal(t, "mqtt-zigbee2mqtt", subscriptions[0].DriverType)
	assert.True(t, subscriptions[0].Enabled)
}

func mockDeviceNames(t *testing.T) []string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "mocks", "mqtt_devices.py"))
	require.NoError(t, err)
	var names []string
	for _, match := range regexp.MustCompile(`Device\(\s*"([^"]+)"`).FindAllStringSubmatch(string(source), -1) {
		names = append(names, match[1])
	}
	require.NotEmpty(t, names, "found no devices in mqtt_devices.py")
	return names
}

func TestSeed_EveryMockDeviceLandsOnTheSensorAutoDiscoveryLooksUp(t *testing.T) {
	db := openTempDatabase(t)
	runSeed(t, db)
	subscriptions, err := database.NewMQTTSubscriptionRepository(db, discardLogger()).GetAll(context.Background())
	require.NoError(t, err)
	require.Len(t, subscriptions, 1)
	driver, ok := drivers.Get(subscriptions[0].DriverType)
	require.True(t, ok)
	pushDriver := driver.(drivers.PushDriver)
	sensors := newSensorService(db)

	names := mockDeviceNames(t)
	for _, name := range names {
		deviceName, err := pushDriver.IdentifyDevice("zigbee2mqtt/"+name, nil)
		require.NoError(t, err, name)
		sensor, err := sensors.ServiceGetSensorByExternalId(context.Background(), deviceName)
		require.NoError(t, err, "mock device %s has no seeded sensor, so auto-discovery would add it as pending", name)
		require.NotNil(t, sensor, name)
		assert.Equal(t, gen.SensorStatusActive, sensor.Status, name)
		assert.Equal(t, subscriptions[0].DriverType, sensor.SensorDriver, name)
	}
	for _, device := range mqttDevices {
		assert.Contains(t, names, device.Name, "seeded sensor %s has no mock device feeding it", device.Name)
	}
}

func TestSeed_RerunKeepsADeletedSeededSensorDeleted(t *testing.T) {
	db := openTempDatabase(t)
	runSeed(t, db)
	sensors := newSensorService(db)
	require.NoError(t, sensors.ServiceDeleteSensorByName(context.Background(), "office-plug"))

	runSeed(t, db)

	deleted, err := sensors.ServiceGetSensorByName(context.Background(), "office-plug")
	require.NoError(t, err)
	assert.Nil(t, deleted, "a deleted seeded sensor stays deleted")
}

func TestSeed_RerunApprovesASensorAnUnfinishedRunLeftPending(t *testing.T) {
	db := openTempDatabase(t)
	sensors := newSensorService(db)
	name := mqttDevices[0].Name
	require.NoError(t, sensors.ServiceAddSensor(context.Background(), gen.Sensor{
		Name: name, ExternalId: &name, SensorDriver: mqttDevices[0].Driver, Status: gen.SensorStatusPending,
	}))

	runSeed(t, db)

	assert.Equal(t, fullySeeded, countEntities(t, db))
	sensor, err := sensors.ServiceGetSensorByName(context.Background(), name)
	require.NoError(t, err)
	assert.Equal(t, gen.SensorStatusActive, sensor.Status)
}

func TestSeed_WaitsForAnHTTPMockThatIsStillStarting(t *testing.T) {
	db := openTempDatabase(t)
	mocks := testHTTPMocks(t)
	mocks[0].url = startTemperatureServer(t, 3).URL

	_, err := seed(context.Background(), db, discardLogger(), mocks, testWindow)

	require.NoError(t, err)
	assert.Equal(t, fullySeeded, countEntities(t, db))
}
