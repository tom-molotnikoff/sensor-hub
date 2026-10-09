package service

import (
	"context"
	"log/slog"
	"testing"

	gen "example/sensorHub/gen"
	"example/sensorHub/notifications"
	"example/sensorHub/secrets"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type namedBrokers map[int]string

func (b namedBrokers) GetByID(_ context.Context, id int) (*gen.MQTTBroker, error) {
	name, ok := b[id]
	if !ok {
		return nil, nil
	}
	return &gen.MQTTBroker{Name: name}, nil
}

type recordedNotification struct {
	notification notifications.Notification
	permission   string
}

type notificationRecorder struct{ created []recordedNotification }

func (r *notificationRecorder) CreateNotification(_ context.Context, n notifications.Notification, permission string) (int, error) {
	r.created = append(r.created, recordedNotification{n, permission})
	return len(r.created), nil
}

func TestNotifySecretFailures_RaisesOneNotificationNamingEachOwner(t *testing.T) {
	recorder := &notificationRecorder{}
	failed := []secrets.Ref{
		{Owner: "mqtt_broker:1", Name: "password"},
		{Owner: "mqtt_broker:2", Name: "password"},
		{Owner: "mqtt_broker:9", Name: "password"}, // deleted, so it has no name
	}

	err := NotifySecretFailures(context.Background(), failed, nil, namedBrokers{1: "Home Mosquitto", 2: "Garage"}, recorder, slog.Default())

	require.NoError(t, err)
	require.Len(t, recorder.created, 1)
	got := recorder.created[0]
	assert.Equal(t, "view_notifications_config", got.permission)
	assert.Equal(t, notifications.CategorySecretFailure, got.notification.Category)
	assert.Equal(t, notifications.SeverityError, got.notification.Severity)
	assert.Equal(t, "Stored secrets need re-entry", got.notification.Title)
	assert.Contains(t, got.notification.Message, `MQTT broker "Home Mosquitto", MQTT broker "Garage", mqtt_broker:9.`)
	assert.NoError(t, got.notification.Validate())
}

func TestNotifySecretFailures_NamesTheEmailSettings(t *testing.T) {
	recorder := &notificationRecorder{}
	failed := []secrets.Ref{
		{Owner: "mqtt_broker:1", Name: "password"},
		{Owner: "smtp", Name: "password"},
	}

	err := NotifySecretFailures(context.Background(), failed, nil, namedBrokers{1: "Home Mosquitto"}, recorder, slog.Default())

	require.NoError(t, err)
	require.Len(t, recorder.created, 1)
	assert.Contains(t, recorder.created[0].notification.Message, `MQTT broker "Home Mosquitto", the email (SMTP) settings.`)
}

func TestNotifySecretFailures_RaisesNothingWhenEverySecretDecrypts(t *testing.T) {
	recorder := &notificationRecorder{}

	require.NoError(t, NotifySecretFailures(context.Background(), nil, nil, namedBrokers{}, recorder, slog.Default()))

	assert.Empty(t, recorder.created)
}

func TestNotifySecretFailures_SaysWhenTheKeyIsNowProtectedByTheHostNotTheTPM(t *testing.T) {
	recorder := &notificationRecorder{}
	replaced := &secrets.KeyReplacement{SealedWith: "host", SetAside: "/etc/sensor-hub/secrets.key.cred.unsealable-20261008T215500Z"}

	err := NotifySecretFailures(context.Background(), []secrets.Ref{{Owner: "mqtt_broker:1", Name: "password"}}, replaced,
		namedBrokers{1: "Home Mosquitto"}, recorder, slog.Default())

	require.NoError(t, err)
	require.Len(t, recorder.created, 1)
	message := recorder.created[0].notification.Message
	assert.Contains(t, message, "kept as /etc/sensor-hub/secrets.key.cred.unsealable-20261008T215500Z")
	assert.Contains(t, message, "protected by this host's credential secret, not the TPM")
}
