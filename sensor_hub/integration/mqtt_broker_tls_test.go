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
	"example/sensorHub/testharness/testca"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tlsPassword = "tls-broker-password"

func newTestCA(t *testing.T, name string) *testca.CA {
	t.Helper()
	ca, err := testca.New(name)
	require.NoError(t, err)
	return ca
}

// startTLSBroker starts a login-protected broker serving TLS with a
// certificate the CA issued for the given hosts.
func startTLSBroker(t *testing.T, ca *testca.CA, hosts ...string) *testharness.ExternalBroker {
	t.Helper()
	certificate, err := ca.ServerCertificate(hosts...)
	require.NoError(t, err)
	external, err := testharness.StartExternalTLSBrokerWithLogin(reconnectUsername, tlsPassword, certificate)
	require.NoError(t, err)
	t.Cleanup(func() { _ = external.Stop() })
	return external
}

func createTLSBroker(t *testing.T, name string, external *testharness.ExternalBroker, tls bool, caPEM *string) reconnectBroker {
	t.Helper()
	resp, status := client.CreateMQTTBroker(gen.MQTTBroker{
		Name: name, Type: "external", Host: ptrStr("127.0.0.1"), Port: ptrInt(external.Port),
		Username: ptrStr(reconnectUsername), Password: ptrStr(tlsPassword), Enabled: true,
		Tls: &tls, CaCertPem: caPEM,
	})
	require.Equal(t, http.StatusCreated, status, "body: %s", resp)
	var created struct {
		ID int `json:"id"`
	}
	require.NoError(t, json.Unmarshal(resp, &created))
	t.Cleanup(func() { client.DeleteMQTTBroker(created.ID) })
	return reconnectBroker{id: created.ID, name: name, external: external}
}

func getBroker(t *testing.T, id int) gen.MQTTBroker {
	t.Helper()
	resp, status := client.GetMQTTBroker(id)
	require.Equal(t, http.StatusOK, status, "body: %s", resp)
	var broker gen.MQTTBroker
	require.NoError(t, json.Unmarshal(resp, &broker))
	return broker
}

func TestBrokerTLS_ConnectsToABrokerWhoseCertificateChainsToThePastedCA(t *testing.T) {
	ca := newTestCA(t, "home CA")
	external := startTLSBroker(t, ca, "127.0.0.1")

	broker := createTLSBroker(t, "tls-pasted-ca", external, true, &ca.PEM)

	assert.Eventually(t, broker.connected, reconnectWithin, 100*time.Millisecond)
	assert.Positive(t, external.ConnectAttempts(tlsPassword), "the password was sent, inside TLS")
	stored := getBroker(t, broker.id)
	require.NotNil(t, stored.Tls)
	assert.True(t, *stored.Tls)
	require.NotNil(t, stored.CaCertPem)
	assert.Equal(t, ca.PEM, *stored.CaCertPem)
}

func TestBrokerTLS_RefusesABrokerWhoseCertificateDoesNotVerify(t *testing.T) {
	ca := newTestCA(t, "home CA")
	other := newTestCA(t, "someone else's CA")
	cases := []struct {
		name  string
		hosts []string
		caPEM *string
	}{
		// A self-signed CA is not among the system roots.
		{name: "no CA pasted, so the system roots", hosts: []string{"127.0.0.1"}},
		{name: "a different CA pasted", hosts: []string{"127.0.0.1"}, caPEM: &other.PEM},
		{name: "a certificate for another host", hosts: []string{"mqtt.example.com"}, caPEM: &ca.PEM},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			external := startTLSBroker(t, ca, tc.hosts...)

			broker := createTLSBroker(t, fmt.Sprintf("tls-refused-%d", i), external, true, tc.caPEM)

			require.Eventually(t, func() bool { return external.TLSHellos() > 0 },
				reconnectWithin, 100*time.Millisecond, "the hub dials the broker over TLS")
			// A handshake that verified would complete, and the CONNECT follow,
			// within milliseconds of the hello.
			assert.Never(t, broker.connected, time.Second, 100*time.Millisecond)
			assert.Zero(t, external.ConnectAttempts(tlsPassword), "the password is never sent to an unverified broker")
		})
	}
}

func TestBrokerTLS_RefusesACACertificateThatIsNotPEM(t *testing.T) {
	notPEM := "MIIBszCCAVmgAwIBAgIBATAKBggqhkjOPQQDAjA"
	resp, status := client.CreateMQTTBroker(gen.MQTTBroker{
		Name: "tls-bad-ca", Type: "external", Host: ptrStr("127.0.0.1"), Port: ptrInt(18831), Enabled: false,
		Tls: ptrBool(true), CaCertPem: &notPEM,
	})
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Contains(t, string(resp), "broker CA certificate holds no PEM certificate")

	external := startLoginBroker(t, tlsPassword)
	broker := createTLSBroker(t, "tls-bad-ca-update", external, false, nil)
	resp, status = client.UpdateMQTTBroker(broker.id, gen.MQTTBroker{
		Name: broker.name, Type: "external", Host: ptrStr("127.0.0.1"), Port: ptrInt(external.Port),
		Username: ptrStr(reconnectUsername), Enabled: true, Tls: ptrBool(true), CaCertPem: &notPEM,
	})
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Contains(t, string(resp), "broker CA certificate holds no PEM certificate")
	assert.False(t, *getBroker(t, broker.id).Tls, "a refused update changes nothing")
}

func TestBrokerTLS_WithTLSOffTheBrokerIsReachedOverPlainTCP(t *testing.T) {
	external := startLoginBroker(t, tlsPassword)

	broker := createTLSBroker(t, "tls-off", external, false, nil)

	assert.Eventually(t, broker.connected, reconnectWithin, 100*time.Millisecond)
	stored := getBroker(t, broker.id)
	require.NotNil(t, stored.Tls)
	assert.False(t, *stored.Tls)
	assert.Nil(t, stored.CaCertPem)
}
