package database

import (
	"database/sql"
	"testing"
	"time"

	"example/sensorHub/alerting"
	gen "example/sensorHub/gen"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// newMockDB creates a new sqlmock database connection for testing.
// Returns the db, mock, and a cleanup function.
func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

func handles(db *sql.DB) *Handles {
	return &Handles{Reader: db, Writer: db}
}

// newMockDBWithQueryMatcher creates a mock DB with custom query matching.
func newMockDBWithQueryMatcher(t *testing.T, matcher sqlmock.QueryMatcher) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

// Test data factories

func testSensor() gen.Sensor {
	return gen.Sensor{
		Id:           1,
		Name:         "test-sensor",
		SensorDriver: "sensor-hub-http-temperature",
		Config:       map[string]string{"url": "http://localhost:8080"},
		HealthStatus: gen.Good,
		HealthReason: "ok",
		Enabled:      true,
	}
}

func testSensorWithID(id int, name string) gen.Sensor {
	return gen.Sensor{
		Id:           id,
		Name:         name,
		SensorDriver: "sensor-hub-http-temperature",
		Config:       map[string]string{"url": "http://localhost:8080"},
		HealthStatus: gen.Good,
		HealthReason: "ok",
		Enabled:      true,
	}
}

func testUser() gen.User {
	return gen.User{
		Id:                 1,
		Username:           "testuser",
		Email:              "test@example.com",
		Disabled:           false,
		MustChangePassword: false,
		Roles:              []string{"user"},
		CreatedAt:          time.Now(),
	}
}

func testUserWithID(id int, username string) gen.User {
	return gen.User{
		Id:                 id,
		Username:           username,
		Email:              username + "@example.com",
		Disabled:           false,
		MustChangePassword: false,
		Roles:              []string{"user"},
		CreatedAt:          time.Now(),
	}
}

func testAlertRule() alerting.AlertRule {
	return alerting.AlertRule{
		ID:                1,
		SensorID:          1,
		MeasurementTypeId: 1,
		SensorName:        "test-sensor",
		AlertType:         alerting.AlertTypeNumericRange,
		HighThreshold:     30.0,
		LowThreshold:      10.0,
		TriggerStatus:     "",
		Enabled:           true,
		RateLimitSeconds:  1,
	}
}

func testReading() gen.Reading {
	val := 22.5
	return gen.Reading{
		Id:              1,
		SensorName:      "test-sensor",
		MeasurementType: "temperature",
		Unit:            "°C",
		NumericValue:    &val,
		Time:            "2026-01-16 12:00:00",
	}
}

func testSessionInfo() SessionInfo {
	now := time.Now()
	return SessionInfo{
		Id:             1,
		UserId:         1,
		CreatedAt:      now,
		ExpiresAt:      now.Add(24 * time.Hour),
		LastAccessedAt: now,
		IpAddress:      "192.168.1.1",
		UserAgent:      "Mozilla/5.0",
	}
}

func testSensorHealthHistory() gen.SensorHealthHistory {
	return gen.SensorHealthHistory{
		Id:           1,
		SensorId:     "1",
		HealthStatus: gen.Good,
		RecordedAt:   time.Now(),
	}
}

// Column definitions for sqlmock rows

var sensorColumns = []string{"id", "name", "external_id", "sensor_driver", "config", "health_status", "health_reason", "enabled", "status", "retention_hours", "metadata"}

var userColumns = []string{"id", "username", "email", "must_change_password", "disabled", "created_at", "updated_at"}

var userColumnsWithHash = []string{"id", "username", "email", "must_change_password", "disabled", "created_at", "updated_at", "password_hash"}

var sessionColumns = []string{"id", "user_id", "created_at", "expires_at", "last_accessed_at", "ip_address", "user_agent"}

var readingColumns = []string{"id", "sensor_id", "sensor_name", "measurement_type", "unit", "numeric_value", "text_state", "time"}

var sensorHealthHistoryColumns = []string{"id", "sensor_id", "health_status", "recorded_at"}

var alertRuleColumns = []string{"id", "sensor_id", "name", "measurement_type_id", "measurement_type", "alert_type", "high_threshold", "low_threshold", "trigger_status", "enabled", "rate_limit_seconds", "sent_at"}

var alertRuleColumnsNoID = []string{"sensor_id", "name", "measurement_type_id", "measurement_type", "alert_type", "high_threshold", "low_threshold", "trigger_status", "enabled", "rate_limit_seconds", "sent_at"}

var alertHistoryColumns = []string{"id", "sensor_id", "alert_type", "reading_value", "sent_at"}

var roleColumns = []string{"id", "name"}

var permissionColumns = []string{"id", "name", "description"}
