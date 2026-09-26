package main

import (
	"context"
	"testing"

	"example/sensorHub/alerting"
	database "example/sensorHub/db"
	"example/sensorHub/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAlertService(db *database.Handles) service.AlertManagementServiceInterface {
	return service.NewAlertManagementService(database.NewAlertRepository(db, discardLogger()), nil, discardLogger())
}

func seededRuleOn(t *testing.T, db *database.Handles, device, measurement string) *alerting.AlertRule {
	t.Helper()
	id, err := newSensorService(db).ServiceGetSensorIdByName(context.Background(), device)
	require.NoError(t, err)
	rules, err := newAlertService(db).ServiceGetAlertRulesBySensorID(context.Background(), id)
	require.NoError(t, err)
	for _, rule := range rules {
		if rule.MeasurementType == measurement {
			return &rule
		}
	}
	return nil
}

func TestAlerts_FirstRunCreatesRangeRulesAndADoorStatusRule(t *testing.T) {
	db := openTempDatabase(t)

	runSeed(t, db)

	temperature := seededRuleOn(t, db, "living-room-sensor", "temperature")
	require.NotNil(t, temperature)
	assert.Equal(t, alerting.AlertTypeNumericRange, temperature.AlertType)
	humidity := seededRuleOn(t, db, "kitchen-sensor", "humidity")
	require.NotNil(t, humidity)
	assert.Equal(t, alerting.AlertTypeNumericRange, humidity.AlertType)
	door := seededRuleOn(t, db, "front-door", "contact")
	require.NotNil(t, door)
	assert.Equal(t, alerting.AlertTypeStatusBased, door.AlertType)
	assert.Equal(t, stateOff, door.TriggerStatus)
	for _, rule := range []*alerting.AlertRule{temperature, humidity, door} {
		assert.True(t, rule.Enabled)
	}
}

func TestAlerts_RerunKeepsEditedAndDeletedRules(t *testing.T) {
	db := openTempDatabase(t)
	runSeed(t, db)
	alerts := newAlertService(db)
	ctx := context.Background()
	edited := seededRuleOn(t, db, "living-room-sensor", "temperature")
	edited.HighThreshold, edited.LowThreshold = 30, 10
	require.NoError(t, alerts.ServiceUpdateAlertRule(ctx, edited))
	deleted := seededRuleOn(t, db, "front-door", "contact")
	require.NoError(t, alerts.ServiceDeleteAlertRule(ctx, deleted.ID))

	runSeed(t, db)

	kept := seededRuleOn(t, db, "living-room-sensor", "temperature")
	require.NotNil(t, kept)
	assert.Equal(t, 30.0, kept.HighThreshold)
	assert.Equal(t, 10.0, kept.LowThreshold)
	assert.Nil(t, seededRuleOn(t, db, "front-door", "contact"), "a deleted seeded rule stays deleted")
}

func notificationStates(t *testing.T, db *database.Handles, username string) (read, unread int) {
	t.Helper()
	user, _, err := database.NewUserRepository(db, discardLogger()).GetUserByUsername(context.Background(), username)
	require.NoError(t, err)
	notifications := service.NewNotificationService(database.NewNotificationRepository(db, discardLogger()), nil, discardLogger())
	received, err := notifications.GetNotificationsForUser(context.Background(), user.Id, notificationPageSize, 0, true)
	require.NoError(t, err)
	for _, item := range received {
		if item.IsRead {
			read++
		} else {
			unread++
		}
	}
	return read, unread
}

func TestNotifications_SomeAreReadAndARerunAddsNone(t *testing.T) {
	db := openTempDatabase(t)
	runSeed(t, db)
	read, unread := notificationStates(t, db, adminUsername)
	assert.Positive(t, read)
	assert.Positive(t, unread)
	assert.Equal(t, len(seededNotifications), read+unread)

	runSeed(t, db)

	assert.Equal(t, len(seededNotifications), queryInt(t, db, "SELECT COUNT(*) FROM notifications"))
	rereadRead, rereadUnread := notificationStates(t, db, adminUsername)
	assert.Equal(t, read, rereadRead)
	assert.Equal(t, unread, rereadUnread)
}
