package appProps

import (
	"os"
	"path/filepath"
	"testing"

	"example/sensorHub/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validAppPropsMap returns a complete valid application properties map
func validAppPropsMap() map[string]string {
	return map[string]string{
		"sensor.collection.interval":        "300",
		"health.history.retention.days":     "180",
		"sensor.data.retention.days":        "365",
		"data.cleanup.interval.hours":       "24",
		"failed.login.retention.days":       "2",
		"auth.bcrypt.cost":                  "12",
		"auth.session.ttl.minutes":          "43200",
		"auth.session.cookie.name":          "sensor_hub_session",
		"auth.login.backoff.window.minutes": "15",
		"auth.login.backoff.threshold":      "5",
		"auth.login.backoff.base.seconds":   "2",
		"auth.login.backoff.max.seconds":    "300",
		"mqtt.broker.enabled":               "true",
		"mqtt.broker.port":                  "1883",
		"hub.timezone":                      "Europe/London",
		"http.listen.address":               "127.0.0.1:8080",
		"mqtt.broker.listen.address":        "127.0.0.1",
		"automation.loop.max.chain":         "5",
		"actuator.command.timeout_seconds":  "10",
	}
}

func validDbPropsMap() map[string]string {
	return map[string]string{
		"database.path":               "test/sensor_hub.db",
		"database.reader.connections": "4",
	}
}

func TestLoadConfigurationFromMaps_Success(t *testing.T) {
	appProps := validAppPropsMap()
	dbProps := validDbPropsMap()

	cfg, err := LoadConfigurationFromMaps(appProps, dbProps)

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, 300, cfg.SensorCollectionInterval)
	assert.Equal(t, 180, cfg.HealthHistoryRetentionDays)
	assert.Equal(t, 365, cfg.SensorDataRetentionDays)
	assert.Equal(t, 24, cfg.DataCleanupIntervalHours)
	assert.Equal(t, 2, cfg.FailedLoginRetentionDays)
	assert.Equal(t, 12, cfg.AuthBcryptCost)
	assert.Equal(t, 43200, cfg.AuthSessionTTLMinutes)
	assert.Equal(t, "sensor_hub_session", cfg.AuthSessionCookieName)
	assert.Equal(t, 15, cfg.AuthLoginBackoffWindowMinutes)
	assert.Equal(t, 5, cfg.AuthLoginBackoffThreshold)
	assert.Equal(t, 2, cfg.AuthLoginBackoffBaseSeconds)
	assert.Equal(t, 300, cfg.AuthLoginBackoffMaxSeconds)
	assert.Equal(t, "test/sensor_hub.db", cfg.DatabasePath)
	assert.Equal(t, 10, cfg.ActuatorCommandTimeoutSeconds)
}

func TestLoadConfigurationFromMaps_EmptyMaps(t *testing.T) {
	cfg, err := LoadConfigurationFromMaps(map[string]string{}, map[string]string{})

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, 0, cfg.SensorCollectionInterval)
}

func TestLoadConfigurationFromMaps_RuleFailureNamesTheKey(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["sensor.collection.interval"] = "-5"

	_, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	var vErr *ValidationError
	assert.ErrorAs(t, err, &vErr)
	assert.Equal(t, "sensor.collection.interval", vErr.Key)
}

func TestLoadConfigurationFromMaps_ParseFailureNamesTheKey(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["sensor.collection.interval"] = "abc"

	_, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	var vErr *ValidationError
	assert.ErrorAs(t, err, &vErr)
	assert.Equal(t, "sensor.collection.interval", vErr.Key)
}

func TestLoadConfigurationFromMaps_EmptyStringRuleFailureNamesTheKey(t *testing.T) {
	dbProps := validDbPropsMap()
	dbProps["database.path"] = ""

	_, err := LoadConfigurationFromMaps(validAppPropsMap(), dbProps)

	var vErr *ValidationError
	assert.ErrorAs(t, err, &vErr)
	assert.Equal(t, "database.path", vErr.Key)
}

func TestLoadConfigurationFromMaps_InvalidSensorCollectionInterval(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["sensor.collection.interval"] = "abc"

	cfg, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidReadingsAggregationEnabled(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["readings.aggregation.enabled"] = "notabool"

	cfg, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidHealthHistoryRetentionDays(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["health.history.retention.days"] = "not-int"

	cfg, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidSensorDataRetentionDays(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["sensor.data.retention.days"] = "xxx"

	cfg, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidDataCleanupIntervalHours(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["data.cleanup.interval.hours"] = "bad"

	cfg, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_LegacyHealthHistoryDefaultResponseNumberIgnored(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["health.history.default.response.number"] = "nope"

	cfg, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidFailedLoginRetentionDays(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["failed.login.retention.days"] = "invalid"

	cfg, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidAuthBcryptCost(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["auth.bcrypt.cost"] = "high"

	cfg, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_BcryptCostOutsideTenToThirtyOneNamesTheKey(t *testing.T) {
	for value, accepted := range map[string]bool{"4": false, "9": false, "10": true, "31": true, "32": false} {
		t.Run(value, func(t *testing.T) {
			appProps := validAppPropsMap()
			appProps["auth.bcrypt.cost"] = value

			_, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

			if accepted {
				assert.NoError(t, err)
				return
			}
			var vErr *ValidationError
			require.ErrorAs(t, err, &vErr)
			assert.Equal(t, "auth.bcrypt.cost", vErr.Key)
		})
	}
}

func TestLoadConfigurationFromMaps_InvalidAuthSessionTTLMinutes(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["auth.session.ttl.minutes"] = "forever"

	cfg, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidAuthLoginBackoffWindowMinutes(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["auth.login.backoff.window.minutes"] = "bad"

	cfg, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidAuthLoginBackoffThreshold(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["auth.login.backoff.threshold"] = "x"

	cfg, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidAuthLoginBackoffBaseSeconds(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["auth.login.backoff.base.seconds"] = "slow"

	cfg, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidAuthLoginBackoffMaxSeconds(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["auth.login.backoff.max.seconds"] = "max"

	cfg, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_ZeroSensorCollectionInterval(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["sensor.collection.interval"] = "0"

	cfg, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_NegativeSensorCollectionInterval(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["sensor.collection.interval"] = "-5"

	cfg, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_ZeroRetentionDays_NonNegative_OK(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["health.history.retention.days"] = "0"

	cfg, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	assert.NoError(t, err)
	assert.Equal(t, 0, cfg.HealthHistoryRetentionDays)
}

func TestLoadConfigurationFromMaps_NegativeRetentionDays(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["sensor.data.retention.days"] = "-1"

	cfg, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_ZeroCleanupInterval(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["data.cleanup.interval.hours"] = "0"

	cfg, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestConvertConfigurationToMaps_Success(t *testing.T) {
	cfg := &ApplicationConfiguration{
		SensorCollectionInterval:      300,
		ReadingsAggregationEnabled:    true,
		HealthHistoryRetentionDays:    180,
		SensorDataRetentionDays:       365,
		DataCleanupIntervalHours:      24,
		FailedLoginRetentionDays:      2,
		AuthBcryptCost:                12,
		AuthSessionTTLMinutes:         43200,
		AuthSessionCookieName:         "my_session",
		AuthLoginBackoffWindowMinutes: 15,
		AuthLoginBackoffThreshold:     5,
		AuthLoginBackoffBaseSeconds:   2,
		AuthLoginBackoffMaxSeconds:    300,
		DatabasePath:                  "test/path.db",
		ActuatorCommandTimeoutSeconds: 12,
	}

	appProps, dbProps := ConvertConfigurationToMaps(cfg)

	assert.Equal(t, "300", appProps["sensor.collection.interval"])
	assert.Equal(t, "true", appProps["readings.aggregation.enabled"])
	assert.Equal(t, "180", appProps["health.history.retention.days"])
	assert.Equal(t, "365", appProps["sensor.data.retention.days"])
	assert.Equal(t, "24", appProps["data.cleanup.interval.hours"])
	assert.Equal(t, "2", appProps["failed.login.retention.days"])
	assert.Equal(t, "12", appProps["auth.bcrypt.cost"])
	assert.Equal(t, "43200", appProps["auth.session.ttl.minutes"])
	assert.Equal(t, "my_session", appProps["auth.session.cookie.name"])
	assert.Equal(t, "15", appProps["auth.login.backoff.window.minutes"])
	assert.Equal(t, "5", appProps["auth.login.backoff.threshold"])
	assert.Equal(t, "2", appProps["auth.login.backoff.base.seconds"])
	assert.Equal(t, "300", appProps["auth.login.backoff.max.seconds"])

	_, hasLegacyHealthHistoryLimit := appProps["health.history.default.response.number"]
	assert.False(t, hasLegacyHealthHistoryLimit)

	assert.Equal(t, "test/path.db", dbProps["database.path"])
	assert.Equal(t, "12", appProps["actuator.command.timeout_seconds"])
}

func TestConvertConfigurationToMaps_ZeroValues(t *testing.T) {
	cfg := &ApplicationConfiguration{}

	appProps, dbProps := ConvertConfigurationToMaps(cfg)

	assert.Equal(t, "0", appProps["sensor.collection.interval"])
	assert.Equal(t, "false", appProps["readings.aggregation.enabled"])
	assert.Equal(t, "", dbProps["database.path"])
}

func TestConvertConfigurationToMaps_RoundTrip(t *testing.T) {
	original := &ApplicationConfiguration{
		SensorCollectionInterval:      600,
		ReadingsAggregationEnabled:    true,
		HealthHistoryRetentionDays:    90,
		SensorDataRetentionDays:       180,
		DataCleanupIntervalHours:      12,
		FailedLoginRetentionDays:      7,
		AuthBcryptCost:                14,
		AuthSessionTTLMinutes:         60,
		AuthSessionCookieName:         "test_session",
		AuthLoginBackoffWindowMinutes: 30,
		AuthLoginBackoffThreshold:     10,
		AuthLoginBackoffBaseSeconds:   5,
		AuthLoginBackoffMaxSeconds:    600,
		DatabasePath:                  "test/roundtrip.db",
		DatabaseReaderConnections:     4,
		MQTTBrokerPort:                1883,
		HubTimezone:                   "Europe/London",
		AutomationLoopMaxChain:        3,
		ActuatorCommandTimeoutSeconds: 25,
		HTTPListenAddress:             "127.0.0.1:8080",
		MQTTBrokerListenAddress:       "0.0.0.0",
		MQTTBrokerConnectRateLimit:    7,
	}

	appProps, dbProps := ConvertConfigurationToMaps(original)
	restored, err := LoadConfigurationFromMaps(appProps, dbProps)

	assert.NoError(t, err)
	assert.Equal(t, original.SensorCollectionInterval, restored.SensorCollectionInterval)
	assert.Equal(t, original.ReadingsAggregationEnabled, restored.ReadingsAggregationEnabled)
	assert.Equal(t, original.AuthBcryptCost, restored.AuthBcryptCost)
	assert.Equal(t, original.DatabasePath, restored.DatabasePath)
	assert.Equal(t, original.DatabaseReaderConnections, restored.DatabaseReaderConnections)
	assert.Equal(t, original.HubTimezone, restored.HubTimezone)
	assert.Equal(t, original.ActuatorCommandTimeoutSeconds, restored.ActuatorCommandTimeoutSeconds)
}

// ============================================================================
// Validation function tests
// ============================================================================

// Tests for LoadConfigurationFromMaps with empty database path

func TestLoadConfigurationFromMaps_EmptyDatabasePath(t *testing.T) {
	appProps := validAppPropsMap()
	dbProps := map[string]string{"database.path": ""}

	cfg, err := LoadConfigurationFromMaps(appProps, dbProps)

	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "database.path must not be empty")
}

// ============================================================================
// File reading tests (with mocked utils.ReadPropertiesFile)
// ============================================================================

func TestReadApplicationPropertiesFile_Success(t *testing.T) {
	originalReadPropertiesFile := utils.ReadPropertiesFile
	defer func() { utils.ReadPropertiesFile = originalReadPropertiesFile }()

	utils.ReadPropertiesFile = func(path string) (map[string]string, error) {
		return map[string]string{
			"sensor.collection.interval": "600",
		}, nil
	}

	props, err := ReadApplicationPropertiesFile()

	assert.NoError(t, err)
	assert.NotNil(t, props)
	assert.Equal(t, "600", props["sensor.collection.interval"])
}

// A bound can be newer than the file: a cost an earlier release accepted
// moves to the nearest bound instead of stopping the hub starting.
func TestReadApplicationPropertiesFile_MovesAnOutOfRangeBcryptCostToTheNearestBound(t *testing.T) {
	originalReadPropertiesFile := utils.ReadPropertiesFile
	defer func() { utils.ReadPropertiesFile = originalReadPropertiesFile }()

	for fileValue, want := range map[string]string{"4": "10", "12": "12", "40": "31"} {
		utils.ReadPropertiesFile = func(path string) (map[string]string, error) {
			return map[string]string{"auth.bcrypt.cost": fileValue}, nil
		}

		props, err := ReadApplicationPropertiesFile()

		require.NoError(t, err)
		assert.Equal(t, want, props["auth.bcrypt.cost"], "file value %s", fileValue)
	}
}

func TestReadApplicationPropertiesFile_FileReadError(t *testing.T) {
	originalReadPropertiesFile := utils.ReadPropertiesFile
	defer func() { utils.ReadPropertiesFile = originalReadPropertiesFile }()

	utils.ReadPropertiesFile = func(path string) (map[string]string, error) {
		return nil, os.ErrNotExist
	}

	props, err := ReadApplicationPropertiesFile()

	assert.Error(t, err)
	assert.Nil(t, props)
	assert.Contains(t, err.Error(), "failed to read application properties file")
}

func TestReadDatabasePropertiesFile_Success(t *testing.T) {
	originalReadPropertiesFile := utils.ReadPropertiesFile
	defer func() { utils.ReadPropertiesFile = originalReadPropertiesFile }()

	utils.ReadPropertiesFile = func(path string) (map[string]string, error) {
		return map[string]string{
			"database.path": "data/my_hub.db",
		}, nil
	}

	props, err := ReadDatabasePropertiesFile()

	assert.NoError(t, err)
	assert.Equal(t, "data/my_hub.db", props["database.path"])
}

func TestReadDatabasePropertiesFile_FileReadError(t *testing.T) {
	originalReadPropertiesFile := utils.ReadPropertiesFile
	defer func() { utils.ReadPropertiesFile = originalReadPropertiesFile }()

	utils.ReadPropertiesFile = func(path string) (map[string]string, error) {
		return nil, os.ErrNotExist
	}

	props, err := ReadDatabasePropertiesFile()

	assert.Error(t, err)
	assert.Nil(t, props)
	assert.Contains(t, err.Error(), "failed to read database properties file")
}

// ============================================================================
// SaveConfigurationToFiles tests (with temp directory)
// ============================================================================

func TestSaveConfigurationToFiles_Success(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "app-props-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	origAppPath := applicationPropertiesFilePath
	origDbPath := databasePropertiesFilePath
	defer func() {
		applicationPropertiesFilePath = origAppPath
		databasePropertiesFilePath = origDbPath
	}()

	applicationPropertiesFilePath = filepath.Join(tempDir, "application.properties")
	databasePropertiesFilePath = filepath.Join(tempDir, "database.properties")

	origConfig := AppConfig()
	defer func() { SetAppConfig(origConfig) }()

	SetAppConfig(&ApplicationConfiguration{
		SensorCollectionInterval:      120,
		HealthHistoryRetentionDays:    90,
		SensorDataRetentionDays:       180,
		DataCleanupIntervalHours:      12,
		FailedLoginRetentionDays:      3,
		AuthBcryptCost:                10,
		AuthSessionTTLMinutes:         60,
		AuthSessionCookieName:         "test_cookie",
		AuthLoginBackoffWindowMinutes: 10,
		AuthLoginBackoffThreshold:     3,
		AuthLoginBackoffBaseSeconds:   1,
		AuthLoginBackoffMaxSeconds:    60,
		DatabasePath:                  "test/save.db",
		ActuatorCommandTimeoutSeconds: 9,
	})

	err = SaveConfigurationToFiles()

	assert.NoError(t, err)

	_, err = os.Stat(applicationPropertiesFilePath)
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(tempDir, "smtp.properties"))
	assert.ErrorIs(t, err, os.ErrNotExist, "smtp.properties is no longer written")
	_, err = os.Stat(databasePropertiesFilePath)
	assert.NoError(t, err)

	appContent, err := os.ReadFile(applicationPropertiesFilePath)
	assert.NoError(t, err)
	assert.Contains(t, string(appContent), "sensor.collection.interval=120")
	assert.Contains(t, string(appContent), "actuator.command.timeout_seconds=9")

	dbContent, err := os.ReadFile(databasePropertiesFilePath)
	assert.NoError(t, err)
	assert.Contains(t, string(dbContent), "database.path=test/save.db")
}

// Every save leaves the files at the mode the package installs them at, new
// or existing.
func TestSaveConfigurationToFiles_WritesFilesAtMode0640(t *testing.T) {
	tempDir := t.TempDir()

	origAppPath, origDbPath := applicationPropertiesFilePath, databasePropertiesFilePath
	defer func() {
		applicationPropertiesFilePath, databasePropertiesFilePath = origAppPath, origDbPath
	}()
	applicationPropertiesFilePath = filepath.Join(tempDir, "application.properties")
	databasePropertiesFilePath = filepath.Join(tempDir, "database.properties")

	require.NoError(t, os.WriteFile(applicationPropertiesFilePath, []byte("stale=1\n"), 0o644))
	require.NoError(t, os.Chmod(applicationPropertiesFilePath, 0o644))
	require.NoError(t, os.WriteFile(databasePropertiesFilePath, []byte("stale=1\n"), 0o600))

	origConfig := AppConfig()
	defer func() { SetAppConfig(origConfig) }()
	SetAppConfig(&ApplicationConfiguration{AuthBcryptCost: 12, DatabasePath: "test/save.db"})

	require.NoError(t, SaveConfigurationToFiles())

	modes := map[string]os.FileMode{
		applicationPropertiesFilePath: 0o640,
		databasePropertiesFilePath:    0o640,
	}
	for path, want := range modes {
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, want, info.Mode().Perm(), filepath.Base(path))
	}
	content, err := os.ReadFile(applicationPropertiesFilePath)
	require.NoError(t, err)
	assert.NotContains(t, string(content), "stale=1")
}

func TestSaveConfigurationToFiles_NilAppConfig(t *testing.T) {
	origConfig := AppConfig()
	defer func() { SetAppConfig(origConfig) }()

	SetAppConfig(nil)

	err := SaveConfigurationToFiles()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no application configuration loaded")
}

func TestSaveConfigurationToFiles_InvalidPath(t *testing.T) {
	origAppPath := applicationPropertiesFilePath
	defer func() { applicationPropertiesFilePath = origAppPath }()

	applicationPropertiesFilePath = "/nonexistent/dir/application.properties"

	origConfig := AppConfig()
	defer func() { SetAppConfig(origConfig) }()

	SetAppConfig(&ApplicationConfiguration{
		SensorCollectionInterval: 300,
	})

	err := SaveConfigurationToFiles()

	assert.Error(t, err)
}

// ============================================================================
// ReloadConfig and edge case tests
// ============================================================================

func TestReloadConfig_Success(t *testing.T) {
	origConfig := AppConfig()
	defer func() { SetAppConfig(origConfig) }()

	appProps := validAppPropsMap()
	dbProps := validDbPropsMap()

	ReloadConfig(appProps, dbProps)

	assert.NotNil(t, AppConfig())
	assert.Equal(t, 300, AppConfig().SensorCollectionInterval)
	assert.Equal(t, "test/sensor_hub.db", AppConfig().DatabasePath)
}

func TestReloadConfig_InvalidConfig(t *testing.T) {
	origConfig := AppConfig()
	defer func() { SetAppConfig(origConfig) }()

	SetAppConfig(&ApplicationConfiguration{SensorCollectionInterval: 100})

	appProps := validAppPropsMap()
	appProps["sensor.collection.interval"] = "invalid"

	ReloadConfig(appProps, validDbPropsMap())

	assert.Equal(t, 100, AppConfig().SensorCollectionInterval)
}

func TestApplicationPropertiesDefaults_HasExpectedKeys(t *testing.T) {
	appDefaults, _ := BuildDefaults()

	_, hasInterval := appDefaults["sensor.collection.interval"]
	_, hasBcryptCost := appDefaults["auth.bcrypt.cost"]
	_, hasCookieName := appDefaults["auth.session.cookie.name"]
	assert.True(t, hasInterval)
	assert.True(t, hasBcryptCost)
	assert.True(t, hasCookieName)
}

func TestDatabasePropertiesDefaults_Initial(t *testing.T) {
	_, dbDefaults := BuildDefaults()

	_, hasPath := dbDefaults["database.path"]
	assert.True(t, hasPath)
	assert.Equal(t, "4", dbDefaults["database.reader.connections"])
}

func TestLoadConfigurationFromMaps_ReaderConnectionsOverridesTheDefault(t *testing.T) {
	dbProps := validDbPropsMap()
	dbProps["database.reader.connections"] = "2"

	cfg, err := LoadConfigurationFromMaps(validAppPropsMap(), dbProps)

	assert.NoError(t, err)
	assert.Equal(t, 2, cfg.DatabaseReaderConnections)
}

func TestLoadConfigurationFromMaps_NonPositiveReaderConnections(t *testing.T) {
	for _, raw := range []string{"0", "-1"} {
		dbProps := validDbPropsMap()
		dbProps["database.reader.connections"] = raw

		cfg, err := LoadConfigurationFromMaps(validAppPropsMap(), dbProps)

		assert.Error(t, err, "reader connections of %s is rejected", raw)
		assert.Nil(t, cfg)
	}
}

// A 1.5.x install keeps oauth.* keys in application.properties, which its
// saver wrote, and smtp.user in smtp.properties. 2.0 has no such properties:
// it ignores the keys and its next save drops them.
func TestLoadConfigurationFromMaps_IgnoresTheKeys15xLeftBehind(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["oauth.credentials.file.path"] = "credentials.json"
	appProps["oauth.token.file.path"] = "token.json"
	appProps["oauth.token.refresh.interval.minutes"] = "not-a-number"
	appProps["smtp.user"] = "alerts@example.com"

	cfg, err := LoadConfigurationFromMaps(appProps, validDbPropsMap())

	require.NoError(t, err)
	saved, _ := ConvertConfigurationToMaps(cfg)
	for key := range saved {
		assert.NotContains(t, key, "oauth.")
		assert.NotContains(t, key, "smtp.")
	}
}

func TestInitialiseConfig_NeedsNoSMTPProperties(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "application.properties"),
		[]byte("sensor.collection.interval=300\noauth.token.file.path=token.json\n"), 0o640))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "database.properties"), []byte("database.path=data/test.db\n"), 0o640))
	oldDir, oldCfg := GetConfigDir(), AppConfig()
	t.Cleanup(func() {
		setConfigPaths(oldDir)
		SetAppConfig(oldCfg)
	})

	require.NoError(t, InitialiseConfig(dir))

	assert.Equal(t, 300, AppConfig().SensorCollectionInterval)
	assert.Equal(t, []string{filepath.Join(dir, "application.properties"), filepath.Join(dir, "database.properties")}, ConfigFilePaths())
}
