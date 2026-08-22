package alerting

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAlertRule_ValidateNumericRange(t *testing.T) {
	rule := AlertRule{
		SensorID:          1,
		MeasurementTypeId: 1,
		SensorName:        "TestSensor",
		AlertType:         AlertTypeNumericRange,
		HighThreshold:     30.0,
		LowThreshold:      10.0,
		Enabled:           true,
		RateLimitSeconds:  1,
	}

	err := rule.Validate()
	assert.NoError(t, err)
}

func TestAlertRule_ValidateNumericRange_InvalidThresholds(t *testing.T) {
	rule := AlertRule{
		SensorID:          1,
		MeasurementTypeId: 1,
		AlertType:         AlertTypeNumericRange,
		HighThreshold:     10.0,
		LowThreshold:      30.0,
		Enabled:           true,
	}

	err := rule.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "high threshold must be greater than low threshold")
}

func TestAlertRule_ValidateStatusBased(t *testing.T) {
	rule := AlertRule{
		SensorID:          2,
		MeasurementTypeId: 1,
		SensorName:        "DoorSensor",
		AlertType:         AlertTypeStatusBased,
		TriggerStatus:     "open",
		Enabled:           true,
		RateLimitSeconds:  0,
	}

	err := rule.Validate()
	assert.NoError(t, err)
}

func TestAlertRule_ValidateNegativeRateLimit(t *testing.T) {
	rule := AlertRule{
		SensorID:          1,
		MeasurementTypeId: 1,
		AlertType:         AlertTypeNumericRange,
		HighThreshold:     30.0,
		LowThreshold:      10.0,
		RateLimitSeconds:  -1,
		Enabled:           true,
	}

	err := rule.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit seconds cannot be negative")
}

func TestAlertRule_ValidateZeroSensorID(t *testing.T) {
	rule := AlertRule{
		SensorID:         0,
		AlertType:        AlertTypeNumericRange,
		HighThreshold:    30.0,
		LowThreshold:     10.0,
		RateLimitSeconds: 1,
		Enabled:          true,
	}

	err := rule.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sensor ID must be a positive integer")
}

func TestAlertRule_ValidateNegativeSensorID(t *testing.T) {
	rule := AlertRule{
		SensorID:         -5,
		AlertType:        AlertTypeNumericRange,
		HighThreshold:    30.0,
		LowThreshold:     10.0,
		RateLimitSeconds: 1,
		Enabled:          true,
	}

	err := rule.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sensor ID must be a positive integer")
}

func TestAlertRule_ValidateInvalidAlertType(t *testing.T) {
	rule := AlertRule{
		SensorID:          1,
		MeasurementTypeId: 1,
		AlertType:         "invalid_type",
		HighThreshold:     30.0,
		LowThreshold:      10.0,
		RateLimitSeconds:  1,
		Enabled:           true,
	}

	err := rule.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid alert type")
}

func TestAlertRule_ShouldAlert_NumericHigh(t *testing.T) {
	rule := AlertRule{
		AlertType:     AlertTypeNumericRange,
		HighThreshold: 30.0,
		LowThreshold:  10.0,
		Enabled:       true,
	}

	shouldAlert, reason := rule.ShouldAlert(35.0, "")
	assert.True(t, shouldAlert)
	assert.Contains(t, reason, "above high threshold")
}

func TestAlertRule_ShouldAlert_NumericLow(t *testing.T) {
	rule := AlertRule{
		AlertType:     AlertTypeNumericRange,
		HighThreshold: 30.0,
		LowThreshold:  10.0,
		Enabled:       true,
	}

	shouldAlert, reason := rule.ShouldAlert(5.0, "")
	assert.True(t, shouldAlert)
	assert.Contains(t, reason, "below low threshold")
}

func TestAlertRule_ShouldAlert_NumericInRange(t *testing.T) {
	rule := AlertRule{
		AlertType:     AlertTypeNumericRange,
		HighThreshold: 30.0,
		LowThreshold:  10.0,
		Enabled:       true,
	}

	shouldAlert, _ := rule.ShouldAlert(20.0, "")
	assert.False(t, shouldAlert)
}

func TestAlertRule_ShouldAlert_StatusMatch(t *testing.T) {
	rule := AlertRule{
		AlertType:     AlertTypeStatusBased,
		TriggerStatus: "open",
		Enabled:       true,
	}

	shouldAlert, reason := rule.ShouldAlert(0, "open")
	assert.True(t, shouldAlert)
	assert.Contains(t, reason, "status is")
}

func TestAlertRule_ShouldAlert_StatusNoMatch(t *testing.T) {
	rule := AlertRule{
		AlertType:     AlertTypeStatusBased,
		TriggerStatus: "open",
		Enabled:       true,
	}

	shouldAlert, _ := rule.ShouldAlert(0, "closed")
	assert.False(t, shouldAlert)
}

func TestAlertRule_IsRateLimited_NoLimit(t *testing.T) {
	rule := AlertRule{
		RateLimitSeconds: 0,
	}

	assert.False(t, rule.IsRateLimited())
}

func TestAlertRule_IsRateLimited_NeverSent(t *testing.T) {
	rule := AlertRule{
		RateLimitSeconds: 1,
		LastAlertSentAt:  nil,
	}

	assert.False(t, rule.IsRateLimited())
}

func TestAlertRule_IsRateLimited_RecentlySent(t *testing.T) {
	thirtyMinutesAgo := time.Now().Add(-30 * time.Minute)

	rule := AlertRule{
		RateLimitSeconds: 3600, // 1 hour
		LastAlertSentAt:  &thirtyMinutesAgo,
	}

	assert.True(t, rule.IsRateLimited())
}

func TestAlertRule_IsRateLimited_OldEnough(t *testing.T) {
	twoHoursAgo := time.Now().Add(-2 * time.Hour)

	rule := AlertRule{
		RateLimitSeconds: 3600, // 1 hour
		LastAlertSentAt:  &twoHoursAgo,
	}

	assert.False(t, rule.IsRateLimited())
}
