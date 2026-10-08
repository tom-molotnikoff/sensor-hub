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
	"github.com/stretchr/testify/require"
)

// undecryptablePassword is a secret store whose broker password was sealed
// under another key.
type undecryptablePassword struct{}

func (undecryptablePassword) Get(context.Context, string, string) (string, secrets.Status, error) {
	return "", secrets.StatusNeedsReentry, nil
}

func TestConnectionManager_DoesNotDialWithAPasswordThatNeedsReentry(t *testing.T) {
	host, portText, err := net.SplitHostPort(startAuthBroker(t))
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)
	id := 7
	username := "plug"
	broker := gen.MQTTBroker{Id: &id, Name: "protected", Type: "external", Host: &host, Port: &port, Enabled: true, Username: &username}
	cm := NewConnectionManager(&MockSensorService{}, &MockSubRepo{}, &MockBrokerRepo{}, undecryptablePassword{}, nil, slog.Default())
	t.Cleanup(cm.Stop)

	err = cm.ConnectBroker(context.Background(), broker)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "enter it again")
	assert.False(t, cm.IsConnected(7))
}
