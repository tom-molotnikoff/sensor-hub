//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	gen "example/sensorHub/gen"
	"example/sensorHub/testharness"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reconnectWithin is how soon a broker change must show in the connection.
const reconnectWithin = 15 * time.Second

const reconnectUsername = "hub"

// reconnectBroker is an external broker created through the API, pointed at
// a login-protected test broker.
type reconnectBroker struct {
	id       int
	name     string
	external *testharness.ExternalBroker
}

func startLoginBroker(t *testing.T, password string) *testharness.ExternalBroker {
	t.Helper()
	external, err := testharness.StartExternalBrokerWithLogin(reconnectUsername, password)
	require.NoError(t, err)
	t.Cleanup(func() { _ = external.Stop() })
	return external
}

func createReconnectBroker(t *testing.T, name string, external *testharness.ExternalBroker, password string) reconnectBroker {
	t.Helper()
	resp, status := client.CreateMQTTBroker(gen.MQTTBroker{
		Name: name, Type: "external", Host: ptrStr("127.0.0.1"), Port: ptrInt(external.Port),
		Username: ptrStr(reconnectUsername), Password: ptrStr(password), Enabled: true,
	})
	require.Equal(t, http.StatusCreated, status, "body: %s", resp)
	var created struct {
		ID int `json:"id"`
	}
	require.NoError(t, json.Unmarshal(resp, &created))
	t.Cleanup(func() { client.DeleteMQTTBroker(created.ID) })
	return reconnectBroker{id: created.ID, name: name, external: external}
}

// update writes the broker back with the given password and enabled flag. A
// nil password keeps the stored one.
func (b reconnectBroker) update(t *testing.T, password *string, enabled bool) {
	t.Helper()
	resp, status := client.UpdateMQTTBroker(b.id, gen.MQTTBroker{
		Name: b.name, Type: "external", Host: ptrStr("127.0.0.1"), Port: ptrInt(b.external.Port),
		Username: ptrStr(reconnectUsername), Password: password, Enabled: enabled,
	})
	require.Equal(t, http.StatusOK, status, "body: %s", resp)
}

// connected reports whether both the hub and the broker see the hub's client
// connected.
func (b reconnectBroker) connected() bool {
	return env.ConnectionManager.IsConnected(b.id) && b.external.Connected(fmt.Sprintf("sensor-hub-%d", b.id))
}

// disconnected reports whether neither side sees the hub's client connected.
func (b reconnectBroker) disconnected() bool {
	return !env.ConnectionManager.IsConnected(b.id) && !b.external.Connected(fmt.Sprintf("sensor-hub-%d", b.id))
}

func TestBrokerReconnect_ACreatedEnabledBrokerConnectsWithoutARestart(t *testing.T) {
	external := startLoginBroker(t, "created-right")

	broker := createReconnectBroker(t, "reconnect-created", external, "created-right")

	assert.Eventually(t, broker.connected, reconnectWithin, 100*time.Millisecond)
}

func TestBrokerReconnect_APasswordChangeIsFollowedAndSubscriptionsComeBack(t *testing.T) {
	external := startLoginBroker(t, "changed-right")
	broker := createReconnectBroker(t, "reconnect-password", external, "changed-right")
	require.Eventually(t, broker.connected, reconnectWithin, 100*time.Millisecond)
	resp, status := client.CreateMQTTSubscription(gen.MQTTSubscription{
		BrokerId: broker.id, TopicPattern: "reconnect/#", DriverType: "mqtt-zigbee2mqtt", Enabled: true,
	})
	require.Equal(t, http.StatusCreated, status, "body: %s", resp)

	broker.update(t, ptrStr("changed-wrong"), true)
	require.Eventually(t, broker.disconnected, reconnectWithin, 100*time.Millisecond)
	require.Eventually(t, func() bool { return external.ConnectAttempts("changed-wrong") > 0 },
		reconnectWithin, 100*time.Millisecond, "the new, wrong password is tried")
	assert.Never(t, broker.connected, time.Second, 100*time.Millisecond, "the broker refuses the wrong password")

	changed := time.Now()
	broker.update(t, ptrStr("changed-right"), true)
	require.Eventually(t, broker.connected, reconnectWithin, 100*time.Millisecond)
	t.Logf("reconnected %s after the change", time.Since(changed))

	publisher := pahomqtt.NewClient(pahomqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://127.0.0.1:%d", external.Port)).
		SetClientID("reconnect-publisher").
		SetUsername(reconnectUsername).
		SetPassword("changed-right"))
	token := publisher.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	t.Cleanup(func() { publisher.Disconnect(250) })

	received := env.ConnectionManager.Stats()[broker.id].MessagesReceived
	assert.Eventually(t, func() bool {
		publisher.Publish("reconnect/availability", 0, false, "online").WaitTimeout(time.Second)
		return env.ConnectionManager.Stats()[broker.id].MessagesReceived > received
	}, reconnectWithin, 200*time.Millisecond, "the subscription is in place on the new connection")
}

func TestBrokerReconnect_DisablingABrokerDisconnectsIt(t *testing.T) {
	external := startLoginBroker(t, "disabled-right")
	broker := createReconnectBroker(t, "reconnect-disabled", external, "disabled-right")
	require.Eventually(t, broker.connected, reconnectWithin, 100*time.Millisecond)

	broker.update(t, nil, false)

	assert.Eventually(t, broker.disconnected, reconnectWithin, 100*time.Millisecond)
}

func TestBrokerReconnect_DeletingABrokerDisconnectsIt(t *testing.T) {
	external := startLoginBroker(t, "deleted-right")
	broker := createReconnectBroker(t, "reconnect-deleted", external, "deleted-right")
	require.Eventually(t, broker.connected, reconnectWithin, 100*time.Millisecond)

	require.Equal(t, http.StatusNoContent, client.DeleteMQTTBroker(broker.id))

	assert.Eventually(t, broker.disconnected, reconnectWithin, 100*time.Millisecond)
	_, tracked := env.ConnectionManager.Stats()[broker.id]
	assert.False(t, tracked, "a deleted broker leaves no statistics behind")
}

func TestBrokerReconnect_ABrokerThatNeverConnectedIsReplacedCleanly(t *testing.T) {
	external := startLoginBroker(t, "never-right")
	broker := createReconnectBroker(t, "reconnect-never", external, "never-wrong")
	require.Eventually(t, func() bool { return external.ConnectAttempts("never-wrong") > 0 },
		reconnectWithin, 100*time.Millisecond, "the client retries with the wrong password")
	require.False(t, broker.connected())

	broker.update(t, ptrStr("never-right"), true)
	require.Eventually(t, broker.connected, reconnectWithin, 100*time.Millisecond)

	// The retry interval is 5 seconds: a client still retrying in the
	// background would present the old password again within this wait.
	stale := external.ConnectAttempts("never-wrong")
	time.Sleep(7 * time.Second)
	assert.Equal(t, stale, external.ConnectAttempts("never-wrong"), "the old password is never presented again")
	assert.True(t, broker.connected())
}
