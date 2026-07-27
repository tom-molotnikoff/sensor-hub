package service

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"testing"

	appProps "example/sensorHub/application_properties"
	"example/sensorHub/telemetry"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// Test helpers
// ============================================================================

func setupPropertiesServiceTestConfig() func() {
	// Save original config
	originalConfig := appProps.AppConfig

	// Set up minimal test config with actual field names
	appProps.AppConfig = &appProps.ApplicationConfiguration{
		SensorCollectionInterval:      30,
		AuthSessionTTLMinutes:         60,
		AuthBcryptCost:                4,
		HealthHistoryRetentionDays:    30,
		SensorDataRetentionDays:       90,
		DataCleanupIntervalHours:      24,
		FailedLoginRetentionDays:      2,
		SMTPUser:                      "testuser",
		DatabasePath:                  "data/sensor_hub.db",
		MQTTBrokerPort:                1883,
		ActuatorCommandTimeoutSeconds: 10,
	}

	return func() {
		appProps.AppConfig = originalConfig
	}
}

// ============================================================================
// ServiceGetProperties tests
// ============================================================================

func TestPropertiesService_ServiceGetProperties_Success(t *testing.T) {
	cleanup := setupPropertiesServiceTestConfig()
	defer cleanup()

	service := NewPropertiesService(slog.Default())

	result, err := service.ServiceGetProperties(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check some expected keys exist
	assert.Contains(t, result, "sensor.collection.interval")
	assert.Contains(t, result, "auth.session.ttl.minutes")
}

func TestPropertiesService_ServiceGetProperties_ReturnsTheStoredValue(t *testing.T) {
	cleanup := setupPropertiesServiceTestConfig()
	defer cleanup()

	appProps.AppConfig.SMTPUser = "admin@example.com"

	service := NewPropertiesService(slog.Default())

	result, err := service.ServiceGetProperties(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, "admin@example.com", result["smtp.user"])
}

func TestPropertiesService_ServiceGetProperties_IncludesAllPropertyTypes(t *testing.T) {
	cleanup := setupPropertiesServiceTestConfig()
	defer cleanup()

	service := NewPropertiesService(slog.Default())

	result, err := service.ServiceGetProperties(context.Background())

	assert.NoError(t, err)

	// Should include app properties
	assert.Contains(t, result, "sensor.collection.interval")

	// Should include SMTP properties (if configured)
	assert.Contains(t, result, "smtp.user")

	// Should include database properties
	assert.Contains(t, result, "database.path")
}

// ============================================================================
// ServiceUpdateProperties tests
// ============================================================================

func TestPropertiesService_ServiceUpdateProperties_Success(t *testing.T) {
	cleanup := setupPropertiesServiceTestConfig()
	defer cleanup()

	service := NewPropertiesService(slog.Default())

	properties := map[string]string{
		"sensor.collection.interval": "60",
		"auth.session.ttl.minutes":   "120",
	}

	err := service.ServiceUpdateProperties(context.Background(), properties)

	assert.NoError(t, err)

	// Verify values were updated
	assert.Equal(t, 60, appProps.AppConfig.SensorCollectionInterval)
	assert.Equal(t, 120, appProps.AppConfig.AuthSessionTTLMinutes)

	service.waitForBackgroundWork()
}

func TestPropertiesService_ServiceUpdateProperties_StoresTheSuppliedValue(t *testing.T) {
	values := []string{"admin@example.com", "*****", "", "  spaced  "}

	for _, value := range values {
		t.Run(fmt.Sprintf("%q", value), func(t *testing.T) {
			cleanup := setupPropertiesServiceTestConfig()
			defer cleanup()

			service := NewPropertiesService(slog.Default())

			err := service.ServiceUpdateProperties(context.Background(), map[string]string{
				"smtp.user": value,
			})

			assert.NoError(t, err)
			assert.Equal(t, value, appProps.AppConfig.SMTPUser)

			service.waitForBackgroundWork()
		})
	}
}

func TestPropertiesService_ServiceUpdateProperties_UpdatesDatabasePath(t *testing.T) {
	cleanup := setupPropertiesServiceTestConfig()
	defer cleanup()

	service := NewPropertiesService(slog.Default())

	properties := map[string]string{
		"database.path": "new/path/sensor_hub.db",
	}

	err := service.ServiceUpdateProperties(context.Background(), properties)

	assert.NoError(t, err)
	assert.Equal(t, "new/path/sensor_hub.db", appProps.AppConfig.DatabasePath)

	service.waitForBackgroundWork()
}

func TestPropertiesService_ServiceUpdateProperties_InvalidValue(t *testing.T) {
	cleanup := setupPropertiesServiceTestConfig()
	defer cleanup()

	service := NewPropertiesService(slog.Default())

	// Invalid numeric value
	properties := map[string]string{
		"sensor.collection.interval": "not-a-number",
	}

	err := service.ServiceUpdateProperties(context.Background(), properties)

	// Should return error for invalid values
	assert.Error(t, err)

	service.waitForBackgroundWork()
}

func TestPropertiesService_ServiceUpdateProperties_PartialUpdate(t *testing.T) {
	cleanup := setupPropertiesServiceTestConfig()
	defer cleanup()

	originalSessionTTL := appProps.AppConfig.AuthSessionTTLMinutes

	service := NewPropertiesService(slog.Default())

	// Only update one property
	properties := map[string]string{
		"sensor.collection.interval": "45",
	}

	err := service.ServiceUpdateProperties(context.Background(), properties)

	assert.NoError(t, err)
	assert.Equal(t, 45, appProps.AppConfig.SensorCollectionInterval)
	// Other properties should remain unchanged
	assert.Equal(t, originalSessionTTL, appProps.AppConfig.AuthSessionTTLMinutes)

	service.waitForBackgroundWork()
}

func TestPropertiesService_ServiceUpdateProperties_EmptyMap(t *testing.T) {
	cleanup := setupPropertiesServiceTestConfig()
	defer cleanup()

	service := NewPropertiesService(slog.Default())

	properties := map[string]string{}

	err := service.ServiceUpdateProperties(context.Background(), properties)

	assert.NoError(t, err)

	service.waitForBackgroundWork()
}

func TestPropertiesService_ServiceUpdateProperties_UnknownKey(t *testing.T) {
	cleanup := setupPropertiesServiceTestConfig()
	defer cleanup()

	service := NewPropertiesService(slog.Default())

	// Unknown keys should be ignored
	properties := map[string]string{
		"unknownProperty":            "someValue",
		"sensor.collection.interval": "45",
	}

	err := service.ServiceUpdateProperties(context.Background(), properties)

	assert.NoError(t, err)
	// Known property should still be updated
	assert.Equal(t, 45, appProps.AppConfig.SensorCollectionInterval)

	service.waitForBackgroundWork()
}

// ============================================================================
// Log level tests
// ============================================================================

func TestPropertiesService_ServiceUpdateProperties_LogLevelTakesEffectOnSave(t *testing.T) {
	cleanup := setupPropertiesServiceTestConfig()
	defer cleanup()

	appProps.AppConfig.LogLevel = "info"
	telemetry.SetLogLevel("info")
	defer telemetry.SetLogLevel("info")

	var logs bytes.Buffer
	logger := telemetry.NewLogger(&logs, nil)

	service := NewPropertiesService(slog.Default())

	err := service.ServiceUpdateProperties(context.Background(), map[string]string{
		"log.level": "debug",
	})

	assert.NoError(t, err)

	logger.Debug("a line only debug emits")
	assert.Contains(t, logs.String(), "a line only debug emits")

	service.waitForBackgroundWork()
}

func TestPropertiesService_ServiceUpdateProperties_LogLevelStopsDebugAgainOnSave(t *testing.T) {
	cleanup := setupPropertiesServiceTestConfig()
	defer cleanup()

	appProps.AppConfig.LogLevel = "debug"
	telemetry.SetLogLevel("debug")
	defer telemetry.SetLogLevel("info")

	var logs bytes.Buffer
	logger := telemetry.NewLogger(&logs, nil)

	service := NewPropertiesService(slog.Default())

	err := service.ServiceUpdateProperties(context.Background(), map[string]string{
		"log.level": "info",
	})

	assert.NoError(t, err)

	logger.Debug("a line only debug emits")
	logger.Info("a line info emits")

	assert.NotContains(t, logs.String(), "a line only debug emits")
	assert.Contains(t, logs.String(), "a line info emits")

	service.waitForBackgroundWork()
}

func TestPropertiesService_ServiceUpdateProperties_UnrecognisedLogLevelSavesAndLogsAtInfo(t *testing.T) {
	cleanup := setupPropertiesServiceTestConfig()
	defer cleanup()

	appProps.AppConfig.LogLevel = "debug"
	telemetry.SetLogLevel("debug")
	defer telemetry.SetLogLevel("info")

	var logs bytes.Buffer
	logger := telemetry.NewLogger(&logs, nil)

	service := NewPropertiesService(slog.Default())

	err := service.ServiceUpdateProperties(context.Background(), map[string]string{
		"log.level": "loud",
	})

	assert.NoError(t, err)
	assert.Equal(t, "loud", appProps.AppConfig.LogLevel)

	logger.Debug("a line only debug emits")
	logger.Info("a line info emits")

	assert.NotContains(t, logs.String(), "a line only debug emits")
	assert.Contains(t, logs.String(), "a line info emits")

	service.waitForBackgroundWork()
}

// ============================================================================
// NewPropertiesService tests
// ============================================================================

func TestNewPropertiesService_ReturnsService(t *testing.T) {
	service := NewPropertiesService(slog.Default())

	assert.NotNil(t, service)
}
