//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/testharness"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// connectDevice dials the hub's embedded broker over TCP as a home device
// would, and returns the CONNECT's error when it is refused.
func connectDevice(t *testing.T, username, password string) (pahomqtt.Client, error) {
	t.Helper()
	return connectDeviceWith(t, pahomqtt.NewClientOptions().
		SetClientID(fmt.Sprintf("device-%s-%d", username, time.Now().UnixNano())).
		SetUsername(username).
		SetPassword(password))
}

func connectDeviceWith(t *testing.T, options *pahomqtt.ClientOptions) (pahomqtt.Client, error) {
	t.Helper()
	device := pahomqtt.NewClient(options.AddBroker("tcp://" + env.MQTTBrokerAddress).SetAutoReconnect(false))
	token := device.Connect()
	require.True(t, token.WaitTimeout(5*time.Second), "CONNECT timed out")
	if err := token.Error(); err != nil {
		return nil, err
	}
	t.Cleanup(func() { device.Disconnect(250) })
	return device, nil
}

func connectFixtureDevice(t *testing.T) pahomqtt.Client {
	t.Helper()
	device, err := connectDevice(t, env.MQTTClient.Name, env.MQTTClient.Password)
	require.NoError(t, err)
	return device
}

func createMQTTClient(t *testing.T, name, topicPrefix string) gen.MQTTClientCreated {
	t.Helper()
	created, status := client.CreateMQTTClient(gen.MQTTClientInput{Name: name, TopicPrefix: topicPrefix})
	require.Equal(t, http.StatusCreated, status)
	require.NotNil(t, created.Password)
	t.Cleanup(func() { client.DeleteMQTTClient(created.Id) })
	return created
}

func findMQTTClient(t *testing.T, id int) gen.MQTTClient {
	t.Helper()
	clients, status := client.ListMQTTClients()
	require.Equal(t, http.StatusOK, status)
	for _, c := range clients {
		if c.Id == id {
			return c
		}
	}
	t.Fatalf("MQTT client %d is not listed", id)
	return gen.MQTTClient{}
}

func subscribeDevice(t *testing.T, device pahomqtt.Client, filter string, received chan<- pahomqtt.Message) byte {
	t.Helper()
	token := device.Subscribe(filter, 0, func(_ pahomqtt.Client, msg pahomqtt.Message) {
		if received != nil {
			received <- msg
		}
	})
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	return token.(*pahomqtt.SubscribeToken).Result()[filter]
}

func publishFromDevice(t *testing.T, device pahomqtt.Client, topic, payload string) {
	t.Helper()
	token := device.Publish(topic, 0, false, payload)
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
}

func embeddedBrokerID(t *testing.T) int {
	t.Helper()
	raw, status := client.ListMQTTBrokers()
	require.Equal(t, http.StatusOK, status)
	var brokers []gen.MQTTBroker
	require.NoError(t, json.Unmarshal(raw, &brokers))
	for _, broker := range brokers {
		if broker.Type == "embedded" {
			assert.Nil(t, broker.Host, "the embedded broker has no host")
			assert.Nil(t, broker.Port, "the embedded broker has no port")
			return *broker.Id
		}
	}
	t.Fatal("no embedded broker is listed")
	return 0
}

func TestMQTTClients_CreateReturnsThePasswordOnce(t *testing.T) {
	created := createMQTTClient(t, "garage-bridge", "garage/")
	assert.Regexp(t, `^[0-9a-f]{64}$`, *created.Password)
	assert.Equal(t, "garage/", created.TopicPrefix)
	assert.True(t, created.Enabled)

	raw, status := client.GetMQTTClientRaw(created.Id)
	require.Equal(t, http.StatusOK, status)
	var fetched map[string]any
	require.NoError(t, json.Unmarshal(raw, &fetched))
	assert.NotContains(t, fetched, "password")
	assert.Equal(t, "garage-bridge", fetched["name"])

	_, err := connectDevice(t, "garage-bridge", *created.Password)
	assert.NoError(t, err, "the generated password lets the device connect")
}

func TestMQTTClients_RejectInvalidTopicPrefixes(t *testing.T) {
	for _, prefix := range []string{"", "garage", "garage/+/", "garage/#/", "garage\x00/", "$SYS/"} {
		_, status := client.CreateMQTTClient(gen.MQTTClientInput{Name: "invalid-prefix", TopicPrefix: prefix})
		assert.Equal(t, http.StatusBadRequest, status, "prefix %q", prefix)
	}
}

func TestMQTTClients_ViewerCanListButNotManage(t *testing.T) {
	created := createMQTTClient(t, "viewer-visible", "viewer/")

	_, status := client.CreateUser(gen.CreateUserRequest{
		Username: "mqtt-clients-viewer",
		Password: "viewerpass123",
		Email:    ptrStr("mqtt-clients-viewer@test.com"),
		Roles:    &[]string{"viewer"},
	})
	require.Equal(t, http.StatusCreated, status)
	viewer := testharness.NewClient(t, env.ServerURL)
	require.Equal(t, http.StatusOK, viewer.Login("mqtt-clients-viewer", "viewerpass123"))
	require.Equal(t, http.StatusOK, viewer.ChangePassword("viewerpass123"))

	_, status = viewer.ListMQTTClients()
	assert.Equal(t, http.StatusOK, status)
	_, status = viewer.CreateMQTTClient(gen.MQTTClientInput{Name: "viewer-made", TopicPrefix: "viewer/"})
	assert.Equal(t, http.StatusForbidden, status)
	_, status = viewer.RotateMQTTClientPassword(created.Id)
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, http.StatusForbidden, viewer.DeleteMQTTClient(created.Id))
}

func TestEmbeddedBroker_RefusesUnauthenticatedConnects(t *testing.T) {
	cases := map[string][2]string{
		"anonymous":        {"", ""},
		"unknown username": {"stranger", env.MQTTClient.Password},
		"wrong password":   {env.MQTTClient.Name, "not-the-password"},
	}
	for name, credentials := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := connectDevice(t, credentials[0], credentials[1])
			assert.ErrorIs(t, err, packets.ErrorRefusedNotAuthorised)
		})
	}
}

func TestEmbeddedBroker_HoldsClientsToTheirPrefix(t *testing.T) {
	device := connectFixtureDevice(t)
	other := createMQTTClient(t, "prefix-other", "other/")
	otherDevice, err := connectDevice(t, "prefix-other", *other.Password)
	require.NoError(t, err)

	const refused = 0x80
	assert.NotEqual(t, byte(refused), subscribeDevice(t, device, "zigbee2mqtt/+/state", nil))
	assert.Equal(t, byte(refused), subscribeDevice(t, device, "#", nil))
	assert.Equal(t, byte(refused), subscribeDevice(t, device, "+/x", nil))

	received := make(chan pahomqtt.Message, 10)
	assert.NotEqual(t, byte(refused), subscribeDevice(t, device, "zigbee2mqtt/inbox/#", received))

	publishFromDevice(t, otherDevice, "zigbee2mqtt/inbox/from-other", "outside other/")
	publishFromDevice(t, device, "zigbee2mqtt/inbox/from-self", "inside zigbee2mqtt/")

	select {
	case msg := <-received:
		assert.Equal(t, "zigbee2mqtt/inbox/from-self", msg.Topic(), "a publish outside the client's prefix is dropped")
	case <-time.After(5 * time.Second):
		t.Fatal("a publish inside the client's prefix was not delivered")
	}
	select {
	case msg := <-received:
		t.Fatalf("unexpected delivery on %s", msg.Topic())
	case <-time.After(300 * time.Millisecond):
	}
}

func TestEmbeddedBroker_DropsAWillOutsideThePrefix(t *testing.T) {
	watcher := createMQTTClient(t, "will-watcher", "will/")
	watcherDevice, err := connectDevice(t, "will-watcher", *watcher.Password)
	require.NoError(t, err)
	wills := make(chan pahomqtt.Message, 2)
	subscribeDevice(t, watcherDevice, "will/#", wills)

	inside := createMQTTClient(t, "will-inside", "will/")
	_, err = connectDeviceWith(t, pahomqtt.NewClientOptions().SetClientID("will-inside").
		SetUsername("will-inside").SetPassword(*inside.Password).SetWill("will/inside", "gone", 0, false))
	require.NoError(t, err)
	outside := createMQTTClient(t, "will-outside", "elsewhere/")
	_, err = connectDeviceWith(t, pahomqtt.NewClientOptions().SetClientID("will-outside").
		SetUsername("will-outside").SetPassword(*outside.Password).SetWill("will/outside", "gone", 0, false))
	require.NoError(t, err)

	// Disabling a client drops its connection, which publishes its will.
	disabled := false
	_, status := client.UpdateMQTTClient(outside.Id, gen.MQTTClientInput{Name: "will-outside", TopicPrefix: "elsewhere/", Enabled: &disabled})
	require.Equal(t, http.StatusOK, status)
	_, status = client.UpdateMQTTClient(inside.Id, gen.MQTTClientInput{Name: "will-inside", TopicPrefix: "will/", Enabled: &disabled})
	require.Equal(t, http.StatusOK, status)

	select {
	case msg := <-wills:
		assert.Equal(t, "will/inside", msg.Topic(), "a will outside the client's prefix is dropped")
	case <-time.After(5 * time.Second):
		t.Fatal("a will inside the client's prefix was not published")
	}
	select {
	case msg := <-wills:
		t.Fatalf("unexpected will on %s", msg.Topic())
	case <-time.After(300 * time.Millisecond):
	}
}

func TestEmbeddedBroker_RefusesAnotherClientsLiveClientID(t *testing.T) {
	owner := createMQTTClient(t, "id-owner", "owner/")
	ownerDevice, err := connectDeviceWith(t, pahomqtt.NewClientOptions().SetClientID("shared-client-id").
		SetUsername("id-owner").SetPassword(*owner.Password))
	require.NoError(t, err)

	_, err = connectDeviceWith(t, pahomqtt.NewClientOptions().SetClientID("shared-client-id").
		SetUsername(env.MQTTClient.Name).SetPassword(env.MQTTClient.Password))
	assert.ErrorIs(t, err, packets.ErrorRefusedNotAuthorised)
	_, err = connectDeviceWith(t, pahomqtt.NewClientOptions().SetClientID("inline").
		SetUsername(env.MQTTClient.Name).SetPassword(env.MQTTClient.Password))
	assert.ErrorIs(t, err, packets.ErrorRefusedNotAuthorised, "a device cannot take the hub's own client ID")

	time.Sleep(200 * time.Millisecond)
	assert.True(t, ownerDevice.IsConnected(), "the session's holder stays connected")
}

func TestEmbeddedBroker_HubIngestsAndCommandsThroughTheInlineClient(t *testing.T) {
	ctx := context.Background()
	sensorRepo := database.NewSensorRepository(env.DB, slog.Default())
	sensorName := fmt.Sprintf("inline-plug-%d", time.Now().UnixNano())
	metadata := map[string]interface{}{
		"exposes": []interface{}{map[string]interface{}{
			"type": "binary", "property": "state", "access": float64(7), "value_on": "ON", "value_off": "OFF",
		}},
	}
	require.NoError(t, sensorRepo.AddSensor(ctx, gen.Sensor{
		Name: sensorName, SensorDriver: "mqtt-zigbee2mqtt", Status: gen.SensorStatusActive,
		Enabled: true, Config: map[string]string{}, Metadata: &metadata,
	}))
	sensor, err := sensorRepo.GetSensorByName(ctx, sensorName)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sensorRepo.DeleteSensorByName(ctx, sensorName) })

	readingCount := func() int {
		var count int
		require.NoError(t, env.DB.Reader.QueryRow(`SELECT COUNT(*) FROM readings WHERE sensor_id = ?`, sensor.Id).Scan(&count))
		return count
	}

	device := connectFixtureDevice(t)
	commands := make(chan pahomqtt.Message, 1)
	subscribeDevice(t, device, fmt.Sprintf("zigbee2mqtt/%s/set", sensorName), commands)

	body, status := client.CreateMQTTSubscription(gen.MQTTSubscription{
		BrokerId: embeddedBrokerID(t), TopicPattern: "zigbee2mqtt/#", DriverType: "mqtt-zigbee2mqtt", Enabled: true,
	})
	require.Equal(t, http.StatusCreated, status, "body: %s", body)
	var subscription struct {
		ID int `json:"id"`
	}
	require.NoError(t, json.Unmarshal(body, &subscription))
	subscribed := true
	t.Cleanup(func() {
		if subscribed {
			client.DeleteMQTTSubscription(subscription.ID)
		}
	})

	publishFromDevice(t, device, "zigbee2mqtt/"+sensorName, `{"temperature":21.5}`)
	require.Eventually(t, func() bool { return readingCount() > 0 }, 5*time.Second, 100*time.Millisecond,
		"a reading published under the client's prefix is ingested once the subscription is added")

	_, status = client.SendSensorCommand(sensor.Id, "state", "OFF")
	require.Equal(t, http.StatusAccepted, status)
	select {
	case msg := <-commands:
		assert.JSONEq(t, `{"state":"OFF"}`, string(msg.Payload()))
	case <-time.After(5 * time.Second):
		t.Fatal("the command was not published through the embedded broker")
	}

	require.Equal(t, http.StatusNoContent, client.DeleteMQTTSubscription(subscription.ID))
	subscribed = false
	before := readingCount()
	publishFromDevice(t, device, "zigbee2mqtt/"+sensorName, `{"temperature":22.5}`)
	assert.Never(t, func() bool { return readingCount() > before }, time.Second, 100*time.Millisecond,
		"nothing is ingested once the subscription is removed")
}

func TestMQTTClients_RotatePasswordReplacesTheOldOne(t *testing.T) {
	created := createMQTTClient(t, "rotate-bridge", "rotate/")

	rotated, status := client.RotateMQTTClientPassword(created.Id)
	require.Equal(t, http.StatusOK, status)
	require.NotNil(t, rotated.Password)
	assert.Regexp(t, `^[0-9a-f]{64}$`, *rotated.Password)
	assert.NotEqual(t, *created.Password, *rotated.Password)

	_, err := connectDevice(t, "rotate-bridge", *created.Password)
	assert.ErrorIs(t, err, packets.ErrorRefusedNotAuthorised, "the old password stops working")
	_, err = connectDevice(t, "rotate-bridge", *rotated.Password)
	assert.NoError(t, err, "the new password works")
}

func TestMQTTClients_DisablingDisconnectsAndRefusesTheClient(t *testing.T) {
	created := createMQTTClient(t, "disable-bridge", "disable/")
	device, err := connectDevice(t, "disable-bridge", *created.Password)
	require.NoError(t, err)

	listed := findMQTTClient(t, created.Id)
	assert.True(t, listed.Connected)
	assert.NotNil(t, listed.LastConnectedAt)

	disabled := false
	_, status := client.UpdateMQTTClient(created.Id, gen.MQTTClientInput{Name: "disable-bridge", TopicPrefix: "disable/", Enabled: &disabled})
	require.Equal(t, http.StatusOK, status)

	assert.Eventually(t, func() bool { return !device.IsConnected() }, 5*time.Second, 50*time.Millisecond,
		"a disabled client is disconnected at once")
	assert.False(t, findMQTTClient(t, created.Id).Connected)
	_, err = connectDevice(t, "disable-bridge", *created.Password)
	assert.ErrorIs(t, err, packets.ErrorRefusedNotAuthorised, "a disabled client's next CONNECT is refused")
}

func TestMQTTClients_DeletingDisconnectsAndRemovesTheClient(t *testing.T) {
	created := createMQTTClient(t, "delete-bridge", "delete/")
	device, err := connectDevice(t, "delete-bridge", *created.Password)
	require.NoError(t, err)

	require.Equal(t, http.StatusNoContent, client.DeleteMQTTClient(created.Id))

	assert.Eventually(t, func() bool { return !device.IsConnected() }, 5*time.Second, 50*time.Millisecond,
		"a deleted client is disconnected at once")
	_, status := client.GetMQTTClientRaw(created.Id)
	assert.Equal(t, http.StatusNotFound, status)
	_, err = connectDevice(t, "delete-bridge", *created.Password)
	assert.ErrorIs(t, err, packets.ErrorRefusedNotAuthorised)
}
