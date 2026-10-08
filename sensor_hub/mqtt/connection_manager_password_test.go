package mqtt

import (
	"context"
	"log/slog"
	"net"
	"strconv"
	"testing"

	gen "example/sensorHub/gen"
	"example/sensorHub/secrets"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fixedPassword struct {
	value  string
	status secrets.Status
	asked  []string
}

func (p *fixedPassword) Get(_ context.Context, owner, name string) (string, secrets.Status, error) {
	p.asked = append(p.asked, owner+"/"+name)
	return p.value, p.status, nil
}

// passwordProtectedBroker is a broker row for the authenticating broker that
// startAuthBroker runs, as user "plug", carrying a wrong password of its own.
func passwordProtectedBroker(t *testing.T) gen.MQTTBroker {
	t.Helper()
	host, portText, err := net.SplitHostPort(startAuthBroker(t))
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)
	id := 7
	return gen.MQTTBroker{
		Id: &id, Name: "protected", Type: "external", Host: &host, Port: &port, Enabled: true,
		Username: ptr("plug"), Password: ptr("wrong, and never used"),
	}
}

func ptr(s string) *string { return &s }

func TestConnectionManager_DialsWithThePasswordFromTheSecretStore(t *testing.T) {
	broker := passwordProtectedBroker(t)
	subs := &MockSubRepo{}
	subs.On("GetEnabledByBrokerID", mock.Anything, 7).Return([]gen.MQTTSubscription{}, nil)
	passwords := &fixedPassword{value: "right", status: secrets.StatusSet}
	cm := NewConnectionManager(&MockSensorService{}, subs, &MockBrokerRepo{}, passwords, nil, slog.Default())
	t.Cleanup(cm.Stop)

	require.NoError(t, cm.ConnectBroker(context.Background(), broker))

	assert.True(t, cm.IsConnected(7))
	assert.Equal(t, []string{"mqtt_broker:7/password"}, passwords.asked)
}

func TestConnectionManager_DoesNotDialWithAPasswordThatNeedsReentry(t *testing.T) {
	broker := passwordProtectedBroker(t)
	passwords := &fixedPassword{status: secrets.StatusNeedsReentry}
	cm := NewConnectionManager(&MockSensorService{}, &MockSubRepo{}, &MockBrokerRepo{}, passwords, nil, slog.Default())
	t.Cleanup(cm.Stop)

	err := cm.ConnectBroker(context.Background(), broker)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "enter it again")
	assert.False(t, cm.IsConnected(7))
}
