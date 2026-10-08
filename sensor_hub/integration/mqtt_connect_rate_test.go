//go:build integration

package integration

import (
	"fmt"
	"sync"
	"testing"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The harness runs the broker with the default limit of 20 CONNECTs a second.
const defaultConnectRateLimit = 20

func TestEmbeddedBroker_ShedsAConnectBurstButNotANormalRate(t *testing.T) {
	// Three times the limit is refused in part even if the burst straddles
	// two seconds. Each CONNECT carries the fixture's valid credentials, so
	// any refusal is the rate cap's.
	const burst = 3 * defaultConnectRateLimit
	startOfNextSecond()
	errs := make([]error, burst)
	var wg sync.WaitGroup
	for i := range burst {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = connectAndDisconnect(fmt.Sprintf("burst-%d-%d", i, time.Now().UnixNano()))
		}()
	}
	wg.Wait()

	refused := 0
	for _, err := range errs {
		if err != nil {
			assert.ErrorIs(t, err, packets.ErrorRefusedServerUnavailable)
			refused++
		}
	}
	assert.Positive(t, refused, "a burst of %d CONNECTs sees refusals", burst)
	assert.GreaterOrEqual(t, burst-refused, defaultConnectRateLimit, "the burst's first %d CONNECTs in a second are let in", defaultConnectRateLimit)

	// From the next second, a device connecting a few times a second is let
	// in every time: the burst's refusals did not outlast their second.
	startOfNextSecond()
	for i := range 8 {
		require.NoError(t, connectAndDisconnect(fmt.Sprintf("steady-%d-%d", i, time.Now().UnixNano())), "CONNECT %d at a normal rate", i)
		time.Sleep(250 * time.Millisecond)
	}
}

func TestEmbeddedBroker_FailedConnectsDoNotLockOutTheUsername(t *testing.T) {
	// Fewer CONNECTs than the rate limit, so the rate cap plays no part.
	startOfNextSecond()
	for range defaultConnectRateLimit / 2 {
		_, err := connectDeviceWith(t, mqtt311Device(env.MQTTClient.Name, "not-the-password"))
		require.ErrorIs(t, err, packets.ErrorRefusedNotAuthorised)
	}

	started := time.Now()
	_, err := connectDeviceWith(t, mqtt311Device(env.MQTTClient.Name, env.MQTTClient.Password))
	require.NoError(t, err, "the right password straight after failed ones is let in")
	assert.Less(t, time.Since(started), time.Second, "and is not held back")
}

// connectAndDisconnect connects to the embedded broker as the fixture device
// and disconnects straight away, returning the CONNECT's error. It is safe to
// call from several goroutines.
func connectAndDisconnect(clientID string) error {
	device := pahomqtt.NewClient(mqtt311Device(env.MQTTClient.Name, env.MQTTClient.Password).
		SetClientID(clientID).
		AddBroker("tcp://" + env.MQTTBrokerAddress).
		SetAutoReconnect(false))
	token := device.Connect()
	if !token.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("CONNECT %s timed out", clientID)
	}
	if err := token.Error(); err != nil {
		return err
	}
	device.Disconnect(100)
	return nil
}

// mqtt311Device is a device's client options pinned to MQTT 3.1.1. Paho
// otherwise retries a refused CONNECT as MQTT 3.1, sending two for each.
func mqtt311Device(username, password string) *pahomqtt.ClientOptions {
	return pahomqtt.NewClientOptions().
		SetClientID(fmt.Sprintf("device-%s-%d", username, time.Now().UnixNano())).
		SetUsername(username).
		SetPassword(password).
		SetProtocolVersion(4)
}

func startOfNextSecond() {
	time.Sleep(time.Until(time.Now().Truncate(time.Second).Add(time.Second)))
}
