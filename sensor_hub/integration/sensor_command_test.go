//go:build integration

package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"example/sensorHub/actuation"
	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	mqttpkg "example/sensorHub/mqtt"
	"example/sensorHub/testharness"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type commandFixture struct {
	port     int
	brokerID int
	subID    int
	sensor   gen.Sensor
	stop     func()
}

func TestSendSensorCommand_PublishesAndPersistsSentCommand(t *testing.T) {
	fixture := setupCommandFixture(t, fmt.Sprintf("office-plug-%d", reserveTCPPort(t)))
	defer fixture.stop()

	subscriber := pahomqtt.NewClient(
		pahomqtt.NewClientOptions().
			AddBroker(fmt.Sprintf("tcp://127.0.0.1:%d", fixture.port)).
			SetClientID(fmt.Sprintf("integration-command-subscriber-%d", fixture.port)),
	)
	token := subscriber.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	defer subscriber.Disconnect(250)

	messageCh := make(chan pahomqtt.Message, 1)
	token = subscriber.Subscribe(fmt.Sprintf("zigbee2mqtt/%s/set", fixture.sensor.Name), 1, func(_ pahomqtt.Client, msg pahomqtt.Message) {
		messageCh <- msg
	})
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())

	result, status := client.SendSensorCommand(fixture.sensor.Id, "state", "ON")
	require.Equal(t, http.StatusAccepted, status)
	assert.Equal(t, "state", result.Property)
	assert.Equal(t, "ON", result.Value)
	assert.Equal(t, gen.SensorCommandAcceptedStatusSent, result.Status)
	assert.NotZero(t, result.Id)

	select {
	case msg := <-messageCh:
		assert.Equal(t, fmt.Sprintf("zigbee2mqtt/%s/set", fixture.sensor.Name), msg.Topic())
		assert.JSONEq(t, `{"state":"ON"}`, string(msg.Payload()))
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for published command")
	}

	adminUserID := lookupUserID(t, env.AdminUser)

	var userID int
	var property, value, statusValue, mqttTopic, mqttPayload string
	require.NoError(t, env.DB.QueryRow(`
		SELECT user_id, property, value, status, mqtt_topic, mqtt_payload
		FROM sensor_command_history
		WHERE id = ?
	`, result.Id).Scan(&userID, &property, &value, &statusValue, &mqttTopic, &mqttPayload))

	assert.Equal(t, adminUserID, userID)
	assert.Equal(t, "state", property)
	assert.Equal(t, "ON", value)
	assert.Equal(t, "sent", statusValue)
	assert.Equal(t, fmt.Sprintf("zigbee2mqtt/%s/set", fixture.sensor.Name), mqttTopic)
	assert.JSONEq(t, `{"state":"ON"}`, mqttPayload)
}

func TestSendSensorCommand_ViewerGetsForbidden(t *testing.T) {
	fixture := setupCommandFixture(t, fmt.Sprintf("viewer-plug-%d", reserveTCPPort(t)))
	defer fixture.stop()

	_, status := client.CreateUser(gen.CreateUserRequest{
		Username: fmt.Sprintf("command-viewer-%d", fixture.port),
		Password: "viewerpass123",
		Email:    ptrStr(fmt.Sprintf("command-viewer-%d@test.com", fixture.port)),
		Roles:    &[]string{"viewer"},
	})
	require.Equal(t, http.StatusCreated, status)

	viewer := testharness.NewClient(t, env.ServerURL)
	require.Equal(t, http.StatusOK, viewer.Login(fmt.Sprintf("command-viewer-%d", fixture.port), "viewerpass123"))
	require.Equal(t, http.StatusOK, viewer.ChangePassword("viewerpass123"))

	_, status = viewer.SendSensorCommand(fixture.sensor.Id, "state", "ON")
	assert.Equal(t, http.StatusForbidden, status)
}

func TestSendSensorCommand_BrokerDisconnectedReturnsServiceUnavailable(t *testing.T) {
	fixture := setupCommandFixture(t, fmt.Sprintf("disconnected-plug-%d", reserveTCPPort(t)))
	defer fixture.stop()

	env.ConnectionManager.DisconnectBroker(fixture.brokerID)
	require.Eventually(t, func() bool {
		return !env.ConnectionManager.IsConnected(fixture.brokerID)
	}, 5*time.Second, 100*time.Millisecond)

	_, status := client.SendSensorCommand(fixture.sensor.Id, "state", "ON")
	assert.Equal(t, http.StatusServiceUnavailable, status)
}

func TestSendSensorCommand_DisabledSensorReturnsConflict(t *testing.T) {
	fixture := setupCommandFixture(t, fmt.Sprintf("disabled-plug-%d", reserveTCPPort(t)))
	defer fixture.stop()

	require.Equal(t, http.StatusOK, client.DisableSensor(fixture.sensor.Name))

	_, status := client.SendSensorCommand(fixture.sensor.Id, "state", "ON")
	assert.Equal(t, http.StatusConflict, status)
}

func TestSendSensorCommand_AcknowledgesAndBroadcastsCommandStatus(t *testing.T) {
	fixture := setupCommandFixture(t, fmt.Sprintf("ack-plug-%d", reserveTCPPort(t)))
	defer fixture.stop()

	wsConn := connectCurrentReadingsWebSocket(t)
	defer wsConn.Close()

	subscriber := pahomqtt.NewClient(
		pahomqtt.NewClientOptions().
			AddBroker(fmt.Sprintf("tcp://127.0.0.1:%d", fixture.port)).
			SetClientID(fmt.Sprintf("integration-command-ack-%d", fixture.port)),
	)
	token := subscriber.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	defer subscriber.Disconnect(250)

	messageCh := make(chan pahomqtt.Message, 1)
	token = subscriber.Subscribe(fmt.Sprintf("zigbee2mqtt/%s/set", fixture.sensor.Name), 1, func(_ pahomqtt.Client, msg pahomqtt.Message) {
		messageCh <- msg
	})
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())

	result, status := client.SendSensorCommand(fixture.sensor.Id, "state", "ON")
	require.Equal(t, http.StatusAccepted, status)

	select {
	case <-messageCh:
		pub := subscriber.Publish(fmt.Sprintf("zigbee2mqtt/%s", fixture.sensor.Name), 1, false, `{"state":"ON"}`)
		require.True(t, pub.WaitTimeout(5*time.Second))
		require.NoError(t, pub.Error())
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for command publish")
	}

	require.Eventually(t, func() bool {
		var statusValue sql.NullString
		var acknowledgedValue sql.NullString
		var acknowledgedAt sql.NullTime
		err := env.DB.QueryRow(`
			SELECT status, acknowledged_value, acknowledged_at
			FROM sensor_command_history
			WHERE id = ?
		`, result.Id).Scan(&statusValue, &acknowledgedValue, &acknowledgedAt)
		require.NoError(t, err)
		return statusValue.String == "acknowledged" && acknowledgedValue.String == "true" && acknowledgedAt.Valid
	}, 5*time.Second, 100*time.Millisecond)

	message := readCommandStatusMessage(t, wsConn)
	assert.Equal(t, "command_status", message.Type)
	assert.Equal(t, result.Id, message.ID)
	assert.Equal(t, fixture.sensor.Id, message.SensorID)
	assert.Equal(t, "state", message.Property)
	assert.Equal(t, "ON", message.Value)
	assert.Equal(t, "acknowledged", message.Status)
	require.NotNil(t, message.AcknowledgedValue)
	assert.Equal(t, "true", *message.AcknowledgedValue)
	require.NotNil(t, message.AcknowledgedAt)
}

func TestGetSensorCommandHistory_ReturnsAcknowledgedCommandWithActor(t *testing.T) {
	fixture := setupCommandFixture(t, fmt.Sprintf("history-plug-%d", reserveTCPPort(t)))
	defer fixture.stop()

	subscriber := pahomqtt.NewClient(
		pahomqtt.NewClientOptions().
			AddBroker(fmt.Sprintf("tcp://127.0.0.1:%d", fixture.port)).
			SetClientID(fmt.Sprintf("integration-command-history-%d", fixture.port)),
	)
	token := subscriber.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	defer subscriber.Disconnect(250)

	messageCh := make(chan pahomqtt.Message, 1)
	token = subscriber.Subscribe(fmt.Sprintf("zigbee2mqtt/%s/set", fixture.sensor.Name), 1, func(_ pahomqtt.Client, msg pahomqtt.Message) {
		messageCh <- msg
	})
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())

	result, status := client.SendSensorCommand(fixture.sensor.Id, "state", "ON")
	require.Equal(t, http.StatusAccepted, status)

	select {
	case <-messageCh:
		pub := subscriber.Publish(fmt.Sprintf("zigbee2mqtt/%s", fixture.sensor.Name), 1, false, `{"state":"ON"}`)
		require.True(t, pub.WaitTimeout(5*time.Second))
		require.NoError(t, pub.Error())
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for command publish")
	}

	adminUserID := lookupUserID(t, env.AdminUser)

	var history []gen.CommandHistoryEntry
	require.Eventually(t, func() bool {
		var historyStatus int
		history, historyStatus = client.GetSensorCommandHistory(fixture.sensor.Id)
		if historyStatus != http.StatusOK || len(history) != 1 {
			return false
		}
		entry := history[0]
		return entry.Id == result.Id &&
			entry.Status == gen.CommandHistoryEntryStatusAcknowledged &&
			entry.AcknowledgedAt != nil &&
			entry.AcknowledgedValue != nil &&
			entry.User != nil &&
			entry.User.Username == env.AdminUser
	}, 5*time.Second, 100*time.Millisecond)

	entry := history[0]
	assert.Equal(t, result.Id, entry.Id)
	assert.Equal(t, "state", entry.Property)
	assert.Equal(t, "ON", entry.Value)
	assert.Equal(t, gen.CommandHistoryEntryStatusAcknowledged, entry.Status)
	assert.NotZero(t, entry.SentAt)
	require.NotNil(t, entry.AcknowledgedAt)
	require.NotNil(t, entry.AcknowledgedValue)
	assert.Equal(t, "true", *entry.AcknowledgedValue)
	assert.Positive(t, entry.TimeoutSeconds)
	assert.Equal(t, fmt.Sprintf("zigbee2mqtt/%s/set", fixture.sensor.Name), entry.MqttTopic)
	assert.JSONEq(t, `{"state":"ON"}`, entry.MqttPayload)
	require.NotNil(t, entry.User)
	assert.Equal(t, adminUserID, entry.User.Id)
	assert.Equal(t, env.AdminUser, entry.User.Username)
}

func TestSendSensorCommand_TimesOutAndBroadcastsCommandStatus(t *testing.T) {
	restore := overrideCommandTimeout(t, 1)
	defer restore()

	fixture := setupCommandFixture(t, fmt.Sprintf("timeout-plug-%d", reserveTCPPort(t)))
	defer fixture.stop()

	wsConn := connectCurrentReadingsWebSocket(t)
	defer wsConn.Close()

	result, status := client.SendSensorCommand(fixture.sensor.Id, "state", "ON")
	require.Equal(t, http.StatusAccepted, status)

	var timeoutSeconds int
	require.NoError(t, env.DB.QueryRow(`
		SELECT timeout_seconds
		FROM sensor_command_history
		WHERE id = ?
	`, result.Id).Scan(&timeoutSeconds))
	require.Equal(t, 1, timeoutSeconds)

	require.Eventually(t, func() bool {
		var statusValue string
		err := env.DB.QueryRow(`
			SELECT status
			FROM sensor_command_history
			WHERE id = ?
		`, result.Id).Scan(&statusValue)
		require.NoError(t, err)
		return statusValue == "timed_out"
	}, 5*time.Second, 100*time.Millisecond)

	message := readCommandStatusMessage(t, wsConn)
	assert.Equal(t, "command_status", message.Type)
	assert.Equal(t, result.Id, message.ID)
	assert.Equal(t, fixture.sensor.Id, message.SensorID)
	assert.Equal(t, "state", message.Property)
	assert.Equal(t, "ON", message.Value)
	assert.Equal(t, "timed_out", message.Status)
	assert.Nil(t, message.AcknowledgedValue)
	assert.Nil(t, message.AcknowledgedAt)
}

func setupCommandFixture(t *testing.T, sensorName string) commandFixture {
	t.Helper()

	ctx := context.Background()
	logger := slog.Default()
	port := reserveTCPPort(t)

	embeddedBroker := mqttpkg.NewEmbeddedBroker(mqttpkg.BrokerConfig{
		TCPAddress: fmt.Sprintf(":%d", port),
	}, logger)
	require.NoError(t, embeddedBroker.Start())

	brokerRepo := database.NewMQTTBrokerRepository(env.DB, logger)
	subRepo := database.NewMQTTSubscriptionRepository(env.DB, logger)
	sensorRepo := database.NewSensorRepository(env.DB, logger)

	brokerID, err := brokerRepo.Add(ctx, gen.MQTTBroker{
		Name:    fmt.Sprintf("integration-command-broker-%d", port),
		Host:    "127.0.0.1",
		Port:    port,
		Type:    "external",
		Enabled: true,
	})
	require.NoError(t, err)

	subID, err := subRepo.Add(ctx, gen.MQTTSubscription{
		BrokerId:     brokerID,
		TopicPattern: "zigbee2mqtt/#",
		DriverType:   "mqtt-zigbee2mqtt",
		Enabled:      true,
	})
	require.NoError(t, err)

	metadata := map[string]interface{}{
		"exposes": []interface{}{
			map[string]interface{}{
				"type":      "binary",
				"property":  "state",
				"access":    float64(7),
				"value_on":  "ON",
				"value_off": "OFF",
			},
		},
	}
	require.NoError(t, sensorRepo.AddSensor(ctx, gen.Sensor{
		Name:         sensorName,
		SensorDriver: "mqtt-zigbee2mqtt",
		Status:       gen.SensorStatusActive,
		Config:       map[string]string{},
		Metadata:     &metadata,
	}))

	sensor, err := sensorRepo.GetSensorByName(ctx, sensorName)
	require.NoError(t, err)
	require.NotNil(t, sensor)

	broker := gen.MQTTBroker{
		Id:      &brokerID,
		Name:    fmt.Sprintf("integration-command-broker-%d", port),
		Host:    "127.0.0.1",
		Port:    port,
		Type:    "external",
		Enabled: true,
	}
	require.NoError(t, env.ConnectionManager.ConnectBroker(ctx, broker))
	require.Eventually(t, func() bool {
		return env.ConnectionManager.IsConnected(brokerID)
	}, 5*time.Second, 100*time.Millisecond)

	stop := func() {
		env.ConnectionManager.DisconnectBroker(brokerID)
		_ = sensorRepo.DeleteSensorByName(ctx, sensorName)
		_ = subRepo.Delete(ctx, subID)
		_ = brokerRepo.Delete(ctx, brokerID)
		require.NoError(t, embeddedBroker.Stop())
	}

	return commandFixture{
		port:     port,
		brokerID: brokerID,
		subID:    subID,
		sensor:   *sensor,
		stop:     stop,
	}
}

func lookupUserID(t *testing.T, username string) int {
	t.Helper()

	var id int
	require.NoError(t, env.DB.QueryRow(`SELECT id FROM users WHERE username = ?`, username).Scan(&id))
	return id
}

func connectCurrentReadingsWebSocket(t *testing.T) *websocket.Conn {
	t.Helper()

	conn, resp, err := client.DialWebSocket("/api/readings/ws/current")
	if resp != nil {
		defer resp.Body.Close()
	}
	require.NoError(t, err)
	return conn
}

func readCommandStatusMessage(t *testing.T, conn *websocket.Conn) actuation.CommandStatusMessage {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		require.NoError(t, conn.SetReadDeadline(deadline))
		_, payload, err := conn.ReadMessage()
		require.NoError(t, err)

		if len(payload) == 0 || payload[0] != '{' {
			continue
		}

		var envelope struct {
			Type string `json:"type"`
		}
		require.NoError(t, json.Unmarshal(payload, &envelope))
		if envelope.Type != "command_status" {
			continue
		}

		var message actuation.CommandStatusMessage
		require.NoError(t, json.Unmarshal(payload, &message))
		return message
	}

	t.Fatal("timed out waiting for command_status websocket message")
	return actuation.CommandStatusMessage{}
}

func overrideCommandTimeout(t *testing.T, seconds int) func() {
	t.Helper()

	original := appProps.AppConfig()
	override := *original
	override.ActuatorCommandTimeoutSeconds = seconds
	appProps.SetAppConfig(&override)
	return func() {
		appProps.SetAppConfig(original)
	}
}
