package appProps

import (
	"os"
	"path/filepath"
	"testing"

	"example/sensorHub/utils"

	"github.com/stretchr/testify/assert"
)

// validAppPropsMap returns a complete valid application properties map
func validAppPropsMap() map[string]string {
	return map[string]string{
		"sensor.collection.interval":           "300",
		"sensor.discovery.skip":                "true",
		"openapi.yaml.location":                "/path/to/openapi.yaml",
		"health.history.retention.days":        "180",
		"sensor.data.retention.days":           "365",
		"data.cleanup.interval.hours":          "24",
		"failed.login.retention.days":          "2",
		"auth.bcrypt.cost":                     "12",
		"auth.session.ttl.minutes":             "43200",
		"auth.session.cookie.name":             "sensor_hub_session",
		"auth.login.backoff.window.minutes":    "15",
		"auth.login.backoff.threshold":         "5",
		"auth.login.backoff.base.seconds":      "2",
		"auth.login.backoff.max.seconds":       "300",
		"oauth.credentials.file.path":          "configuration/credentials.json",
		"oauth.token.file.path":                "configuration/token.json",
		"oauth.token.refresh.interval.minutes": "30",
		"mqtt.broker.enabled":                  "true",
		"mqtt.broker.port":                     "1883",
		"actuator.command.timeout_seconds":     "10",
	}
}

func validSmtpPropsMap() map[string]string {
	return map[string]string{
		"smtp.user": "user@example.com",
	}
}

func validDbPropsMap() map[string]string {
	return map[string]string{
		"database.path": "test/sensor_hub.db",
	}
}

func TestLoadConfigurationFromMaps_Success(t *testing.T) {
	appProps := validAppPropsMap()
	smtpProps := validSmtpPropsMap()
	dbProps := validDbPropsMap()

	cfg, err := LoadConfigurationFromMaps(appProps, smtpProps, dbProps)

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, 300, cfg.SensorCollectionInterval)
	assert.True(t, cfg.SensorDiscoverySkip)
	assert.Equal(t, "/path/to/openapi.yaml", cfg.OpenAPILocation)
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
	assert.Equal(t, "user@example.com", cfg.SMTPUser)
	assert.Equal(t, "test/sensor_hub.db", cfg.DatabasePath)
	assert.Equal(t, 10, cfg.ActuatorCommandTimeoutSeconds)
}

func TestLoadConfigurationFromMaps_EmptyMaps(t *testing.T) {
	cfg, err := LoadConfigurationFromMaps(map[string]string{}, map[string]string{}, map[string]string{})

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, 0, cfg.SensorCollectionInterval)
}

func TestLoadConfigurationFromMaps_InvalidSensorCollectionInterval(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["sensor.collection.interval"] = "abc"

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidSensorDiscoverySkip(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["sensor.discovery.skip"] = "maybe"

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidHealthHistoryRetentionDays(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["health.history.retention.days"] = "not-int"

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidSensorDataRetentionDays(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["sensor.data.retention.days"] = "xxx"

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidDataCleanupIntervalHours(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["data.cleanup.interval.hours"] = "bad"

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_LegacyHealthHistoryDefaultResponseNumberIgnored(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["health.history.default.response.number"] = "nope"

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidFailedLoginRetentionDays(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["failed.login.retention.days"] = "invalid"

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidAuthBcryptCost(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["auth.bcrypt.cost"] = "high"

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidAuthSessionTTLMinutes(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["auth.session.ttl.minutes"] = "forever"

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidAuthLoginBackoffWindowMinutes(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["auth.login.backoff.window.minutes"] = "bad"

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidAuthLoginBackoffThreshold(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["auth.login.backoff.threshold"] = "x"

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidAuthLoginBackoffBaseSeconds(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["auth.login.backoff.base.seconds"] = "slow"

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_InvalidAuthLoginBackoffMaxSeconds(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["auth.login.backoff.max.seconds"] = "max"

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_ZeroSensorCollectionInterval(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["sensor.collection.interval"] = "0"

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_NegativeSensorCollectionInterval(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["sensor.collection.interval"] = "-5"

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_ZeroRetentionDays_NonNegative_OK(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["health.history.retention.days"] = "0"

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

	assert.NoError(t, err)
	assert.Equal(t, 0, cfg.HealthHistoryRetentionDays)
}

func TestLoadConfigurationFromMaps_NegativeRetentionDays(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["sensor.data.retention.days"] = "-1"

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigurationFromMaps_ZeroCleanupInterval(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["data.cleanup.interval.hours"] = "0"

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestConvertConfigurationToMaps_Success(t *testing.T) {
	cfg := &ApplicationConfiguration{
		SensorCollectionInterval:      300,
		SensorDiscoverySkip:           true,
		OpenAPILocation:               "/path/to/openapi.yaml",
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
		SMTPUser:                      "user@test.com",
		DatabasePath:                  "test/path.db",
		ActuatorCommandTimeoutSeconds: 12,
	}

	appProps, smtpProps, dbProps := ConvertConfigurationToMaps(cfg)

	assert.Equal(t, "300", appProps["sensor.collection.interval"])
	assert.Equal(t, "true", appProps["sensor.discovery.skip"])
	assert.Equal(t, "/path/to/openapi.yaml", appProps["openapi.yaml.location"])
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

	assert.Equal(t, "user@test.com", smtpProps["smtp.user"])

	assert.Equal(t, "test/path.db", dbProps["database.path"])
	assert.Equal(t, "12", appProps["actuator.command.timeout_seconds"])
}

func TestConvertConfigurationToMaps_ZeroValues(t *testing.T) {
	cfg := &ApplicationConfiguration{}

	appProps, smtpProps, dbProps := ConvertConfigurationToMaps(cfg)

	assert.Equal(t, "0", appProps["sensor.collection.interval"])
	assert.Equal(t, "false", appProps["sensor.discovery.skip"])
	assert.Equal(t, "", smtpProps["smtp.user"])
	assert.Equal(t, "", dbProps["database.path"])
}

func TestConvertConfigurationToMaps_RoundTrip(t *testing.T) {
	original := &ApplicationConfiguration{
		SensorCollectionInterval:      600,
		SensorDiscoverySkip:           false,
		OpenAPILocation:               "/api/spec.yaml",
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
		SMTPUser:                      "smtp@test.com",
		DatabasePath:                  "test/roundtrip.db",
		MQTTBrokerPort:                1883,
		ActuatorCommandTimeoutSeconds: 25,
	}

	appProps, smtpProps, dbProps := ConvertConfigurationToMaps(original)
	restored, err := LoadConfigurationFromMaps(appProps, smtpProps, dbProps)

	assert.NoError(t, err)
	assert.Equal(t, original.SensorCollectionInterval, restored.SensorCollectionInterval)
	assert.Equal(t, original.SensorDiscoverySkip, restored.SensorDiscoverySkip)
	assert.Equal(t, original.AuthBcryptCost, restored.AuthBcryptCost)
	assert.Equal(t, original.SMTPUser, restored.SMTPUser)
	assert.Equal(t, original.DatabasePath, restored.DatabasePath)
	assert.Equal(t, original.ActuatorCommandTimeoutSeconds, restored.ActuatorCommandTimeoutSeconds)
}

// ============================================================================
// Validation function tests
// ============================================================================

func TestValidateApplicationProperties_ValidConfig(t *testing.T) {
	applicationProperties = validAppPropsMap()

	err := validateApplicationProperties()

	assert.NoError(t, err)
}

func TestValidateApplicationProperties_InvalidSensorDiscoverySkip(t *testing.T) {
	applicationProperties = validAppPropsMap()
	applicationProperties["sensor.discovery.skip"] = "yes"

	err := validateApplicationProperties()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid sensor discovery skip")
}

func TestValidateApplicationProperties_EmptyOpenAPIWhenDiscoveryEnabled(t *testing.T) {
	applicationProperties = validAppPropsMap()
	applicationProperties["sensor.discovery.skip"] = "false"
	applicationProperties["openapi.yaml.location"] = ""

	err := validateApplicationProperties()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "openapi.yaml.location cannot be empty")
}

func TestValidateApplicationProperties_EmptyOpenAPIWhenDiscoverySkipped(t *testing.T) {
	applicationProperties = validAppPropsMap()
	applicationProperties["sensor.discovery.skip"] = "true"
	applicationProperties["openapi.yaml.location"] = ""

	err := validateApplicationProperties()

	assert.NoError(t, err)
}

// Tests for validateSMTPProperties

func TestValidateSMTPProperties_ValidConfig(t *testing.T) {
	smtpProperties = validSmtpPropsMap()

	err := validateSMTPProperties()

	assert.NoError(t, err)
}

func TestValidateSMTPProperties_EmptyValues(t *testing.T) {
	smtpProperties = map[string]string{
		"smtp.user": "",
	}

	err := validateSMTPProperties()

	assert.NoError(t, err)
}

// Tests for LoadConfigurationFromMaps with empty database path

func TestLoadConfigurationFromMaps_EmptyDatabasePath(t *testing.T) {
	appProps := validAppPropsMap()
	dbProps := map[string]string{"database.path": ""}

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), dbProps)

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

func TestReadApplicationPropertiesFile_ValidationError(t *testing.T) {
	originalReadPropertiesFile := utils.ReadPropertiesFile
	defer func() { utils.ReadPropertiesFile = originalReadPropertiesFile }()

	utils.ReadPropertiesFile = func(path string) (map[string]string, error) {
		return map[string]string{
			"sensor.discovery.skip": "not_a_bool",
			"openapi.yaml.location": "/path",
		}, nil
	}

	props, err := ReadApplicationPropertiesFile()

	assert.Error(t, err)
	assert.Nil(t, props)
	assert.Contains(t, err.Error(), "validation failed")
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

func TestReadSMTPPropertiesFile_Success(t *testing.T) {
	originalReadPropertiesFile := utils.ReadPropertiesFile
	defer func() { utils.ReadPropertiesFile = originalReadPropertiesFile }()

	utils.ReadPropertiesFile = func(path string) (map[string]string, error) {
		return map[string]string{
			"smtp.user": "sender@test.com",
		}, nil
	}

	props, err := ReadSMTPPropertiesFile()

	assert.NoError(t, err)
	assert.Equal(t, "sender@test.com", props["smtp.user"])
}

func TestReadSMTPPropertiesFile_FileReadError(t *testing.T) {
	originalReadPropertiesFile := utils.ReadPropertiesFile
	defer func() { utils.ReadPropertiesFile = originalReadPropertiesFile }()

	utils.ReadPropertiesFile = func(path string) (map[string]string, error) {
		return nil, os.ErrNotExist
	}

	props, err := ReadSMTPPropertiesFile()

	assert.Error(t, err)
	assert.Nil(t, props)
	assert.Contains(t, err.Error(), "failed to read SMTP properties file")
}

// ============================================================================
// SaveConfigurationToFiles tests (with temp directory)
// ============================================================================

func TestSaveConfigurationToFiles_Success(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "app-props-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	origAppPath := applicationPropertiesFilePath
	origSmtpPath := smtpPropertiesFilePath
	origDbPath := databasePropertiesFilePath
	defer func() {
		applicationPropertiesFilePath = origAppPath
		smtpPropertiesFilePath = origSmtpPath
		databasePropertiesFilePath = origDbPath
	}()

	applicationPropertiesFilePath = filepath.Join(tempDir, "application.properties")
	smtpPropertiesFilePath = filepath.Join(tempDir, "smtp.properties")
	databasePropertiesFilePath = filepath.Join(tempDir, "database.properties")

	origConfig := AppConfig
	defer func() { AppConfig = origConfig }()

	AppConfig = &ApplicationConfiguration{
		SensorCollectionInterval:      120,
		SensorDiscoverySkip:           false,
		OpenAPILocation:               "/test/openapi.yaml",
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
		SMTPUser:                      "test@smtp.com",
		DatabasePath:                  "test/save.db",
		ActuatorCommandTimeoutSeconds: 9,
	}

	err = SaveConfigurationToFiles()

	assert.NoError(t, err)

	_, err = os.Stat(applicationPropertiesFilePath)
	assert.NoError(t, err)
	_, err = os.Stat(smtpPropertiesFilePath)
	assert.NoError(t, err)
	_, err = os.Stat(databasePropertiesFilePath)
	assert.NoError(t, err)

	appContent, err := os.ReadFile(applicationPropertiesFilePath)
	assert.NoError(t, err)
	assert.Contains(t, string(appContent), "sensor.collection.interval=120")
	assert.Contains(t, string(appContent), "actuator.command.timeout_seconds=9")

	smtpContent, err := os.ReadFile(smtpPropertiesFilePath)
	assert.NoError(t, err)
	assert.Contains(t, string(smtpContent), "smtp.user=test@smtp.com")

	dbContent, err := os.ReadFile(databasePropertiesFilePath)
	assert.NoError(t, err)
	assert.Contains(t, string(dbContent), "database.path=test/save.db")
}

func TestSaveConfigurationToFiles_NilAppConfig(t *testing.T) {
	origConfig := AppConfig
	defer func() { AppConfig = origConfig }()

	AppConfig = nil

	err := SaveConfigurationToFiles()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no application configuration loaded")
}

func TestSaveConfigurationToFiles_InvalidPath(t *testing.T) {
	origAppPath := applicationPropertiesFilePath
	defer func() { applicationPropertiesFilePath = origAppPath }()

	applicationPropertiesFilePath = "/nonexistent/dir/application.properties"

	origConfig := AppConfig
	defer func() { AppConfig = origConfig }()

	AppConfig = &ApplicationConfiguration{
		SensorCollectionInterval: 300,
	}

	err := SaveConfigurationToFiles()

	assert.Error(t, err)
}

// ============================================================================
// ReloadConfig and edge case tests
// ============================================================================

func TestReloadConfig_Success(t *testing.T) {
	origConfig := AppConfig
	defer func() { AppConfig = origConfig }()

	appProps := validAppPropsMap()
	smtpProps := validSmtpPropsMap()
	dbProps := validDbPropsMap()

	ReloadConfig(appProps, smtpProps, dbProps)

	assert.NotNil(t, AppConfig)
	assert.Equal(t, 300, AppConfig.SensorCollectionInterval)
	assert.Equal(t, "test/sensor_hub.db", AppConfig.DatabasePath)
}

func TestReloadConfig_InvalidConfig(t *testing.T) {
	origConfig := AppConfig
	defer func() { AppConfig = origConfig }()

	AppConfig = &ApplicationConfiguration{SensorCollectionInterval: 100}

	appProps := validAppPropsMap()
	appProps["sensor.collection.interval"] = "invalid"

	ReloadConfig(appProps, validSmtpPropsMap(), validDbPropsMap())

	assert.Equal(t, 100, AppConfig.SensorCollectionInterval)
}

func TestApplicationPropertiesDefaults_HasExpectedKeys(t *testing.T) {
	appDefaults, _, _ := BuildDefaults()

	_, hasInterval := appDefaults["sensor.collection.interval"]
	_, hasBcryptCost := appDefaults["auth.bcrypt.cost"]
	_, hasCookieName := appDefaults["auth.session.cookie.name"]
	assert.True(t, hasInterval)
	assert.True(t, hasBcryptCost)
	assert.True(t, hasCookieName)
}

func TestSmtpPropertiesDefaults_Initial(t *testing.T) {
	_, smtpDefaults, _ := BuildDefaults()

	_, hasUser := smtpDefaults["smtp.user"]
	assert.True(t, hasUser)
}

func TestDatabasePropertiesDefaults_Initial(t *testing.T) {
	_, _, dbDefaults := BuildDefaults()

	_, hasPath := dbDefaults["database.path"]
	assert.True(t, hasPath)
}

func TestLoadConfigurationFromMaps_OAuthConfig(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["oauth.credentials.file.path"] = "/custom/creds.json"
	appProps["oauth.token.file.path"] = "/custom/token.json"
	appProps["oauth.token.refresh.interval.minutes"] = "45"
	smtpProps := validSmtpPropsMap()
	dbProps := validDbPropsMap()

	cfg, err := LoadConfigurationFromMaps(appProps, smtpProps, dbProps)

	assert.NoError(t, err)
	assert.Equal(t, "/custom/creds.json", cfg.OAuthCredentialsFilePath)
	assert.Equal(t, "/custom/token.json", cfg.OAuthTokenFilePath)
	assert.Equal(t, 45, cfg.OAuthTokenRefreshIntervalMinutes)
}

func TestLoadConfigurationFromMaps_InvalidOAuthTokenRefreshInterval(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["oauth.token.refresh.interval.minutes"] = "not-a-number"
	smtpProps := validSmtpPropsMap()
	dbProps := validDbPropsMap()

	cfg, err := LoadConfigurationFromMaps(appProps, smtpProps, dbProps)

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestConvertConfigurationToMaps_OAuthConfig(t *testing.T) {
	cfg := &ApplicationConfiguration{
		SensorCollectionInterval:         300,
		SensorDiscoverySkip:              true,
		OpenAPILocation:                  "/path/to/openapi.yaml",
		HealthHistoryRetentionDays:       180,
		SensorDataRetentionDays:          365,
		DataCleanupIntervalHours:         24,
		FailedLoginRetentionDays:         2,
		AuthBcryptCost:                   12,
		AuthSessionTTLMinutes:            43200,
		AuthSessionCookieName:            "sensor_hub_session",
		AuthLoginBackoffWindowMinutes:    15,
		AuthLoginBackoffThreshold:        5,
		AuthLoginBackoffBaseSeconds:      2,
		AuthLoginBackoffMaxSeconds:       300,
		OAuthCredentialsFilePath:         "/my/creds.json",
		OAuthTokenFilePath:               "/my/token.json",
		OAuthTokenRefreshIntervalMinutes: 60,
		SMTPUser:                         "user@example.com",
		DatabasePath:                     "test/oauth.db",
	}

	appProps, _, _ := ConvertConfigurationToMaps(cfg)

	assert.Equal(t, "/my/creds.json", appProps["oauth.credentials.file.path"])
	assert.Equal(t, "/my/token.json", appProps["oauth.token.file.path"])
	assert.Equal(t, "60", appProps["oauth.token.refresh.interval.minutes"])
}

// Bug #44: LoadConfigurationFromMaps used to mutate relative OAuth paths in
// place by prepending the config directory, which made the operation
// non-idempotent — every reload added another configDir prefix. The struct
// must now retain the raw user-supplied value; resolution happens on demand.
func TestLoadConfigurationFromMaps_PreservesRawRelativeOAuthPaths(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["oauth.credentials.file.path"] = "credentials.json"
	appProps["oauth.token.file.path"] = "token.json"
	smtpProps := validSmtpPropsMap()
	dbProps := validDbPropsMap()

	cfg, err := LoadConfigurationFromMaps(appProps, smtpProps, dbProps)

	assert.NoError(t, err)
	assert.Equal(t, "credentials.json", cfg.OAuthCredentialsFilePath,
		"raw relative path must be preserved (issue #44)")
	assert.Equal(t, "token.json", cfg.OAuthTokenFilePath,
		"raw relative path must be preserved (issue #44)")
}

// Bug #44: a load → convert → load cycle simulates what happens when
// SaveConfigurationToFiles writes to disk and the watcher reloads. The path
// must remain stable across cycles (no accumulating "configuration/" prefix).
func TestLoadConfigurationFromMaps_OAuthPathsAreIdempotentAcrossReloads(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["oauth.credentials.file.path"] = "credentials.json"
	appProps["oauth.token.file.path"] = "token.json"
	smtpProps := validSmtpPropsMap()
	dbProps := validDbPropsMap()

	cfg1, err := LoadConfigurationFromMaps(appProps, smtpProps, dbProps)
	assert.NoError(t, err)

	app2, smtp2, db2 := ConvertConfigurationToMaps(cfg1)
	cfg2, err := LoadConfigurationFromMaps(app2, smtp2, db2)
	assert.NoError(t, err)

	app3, smtp3, db3 := ConvertConfigurationToMaps(cfg2)
	cfg3, err := LoadConfigurationFromMaps(app3, smtp3, db3)
	assert.NoError(t, err)

	assert.Equal(t, "credentials.json", cfg3.OAuthCredentialsFilePath,
		"OAuth credentials path must not accumulate configDir prefixes across reloads (issue #44)")
	assert.Equal(t, "token.json", cfg3.OAuthTokenFilePath,
		"OAuth token path must not accumulate configDir prefixes across reloads (issue #44)")
}

// Bug #44: relative OAuth paths are resolved against the config directory at
// consumption time, not by mutating the configuration struct.
func TestApplicationConfiguration_ResolvedOAuthPaths_RelativeJoinsConfigDir(t *testing.T) {
	cfg := &ApplicationConfiguration{
		OAuthCredentialsFilePath: "credentials.json",
		OAuthTokenFilePath:       "token.json",
	}

	assert.Equal(t, filepath.Join(GetConfigDir(), "credentials.json"),
		cfg.ResolvedOAuthCredentialsPath())
	assert.Equal(t, filepath.Join(GetConfigDir(), "token.json"),
		cfg.ResolvedOAuthTokenPath())
}

func TestApplicationConfiguration_ResolvedOAuthPaths_AbsolutePassesThrough(t *testing.T) {
	cfg := &ApplicationConfiguration{
		OAuthCredentialsFilePath: "/etc/sensor_hub/creds.json",
		OAuthTokenFilePath:       "/etc/sensor_hub/token.json",
	}

	assert.Equal(t, "/etc/sensor_hub/creds.json", cfg.ResolvedOAuthCredentialsPath())
	assert.Equal(t, "/etc/sensor_hub/token.json", cfg.ResolvedOAuthTokenPath())
}

func TestApplicationConfiguration_ResolvedOAuthPaths_EmptyReturnsEmpty(t *testing.T) {
	cfg := &ApplicationConfiguration{}

	assert.Equal(t, "", cfg.ResolvedOAuthCredentialsPath())
	assert.Equal(t, "", cfg.ResolvedOAuthTokenPath())
}
