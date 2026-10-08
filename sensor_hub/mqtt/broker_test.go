package mqtt

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedBroker_StartStop(t *testing.T) {
	broker := NewEmbeddedBroker(BrokerConfig{
		TCPAddress: ":0", // OS-assigned port
	}, nil, slog.Default())

	require.NoError(t, broker.Start())
	assert.True(t, broker.IsRunning())

	require.NoError(t, broker.Stop())
	assert.False(t, broker.IsRunning())
}

func TestEmbeddedBroker_DoubleStart(t *testing.T) {
	broker := NewEmbeddedBroker(BrokerConfig{
		TCPAddress: ":0",
	}, nil, slog.Default())

	require.NoError(t, broker.Start())
	defer broker.Stop()

	err := broker.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestEmbeddedBroker_StopWhenNotRunning(t *testing.T) {
	broker := NewEmbeddedBroker(BrokerConfig{
		TCPAddress: ":0",
	}, nil, slog.Default())

	assert.NoError(t, broker.Stop())
}

func TestEmbeddedBroker_Server(t *testing.T) {
	broker := NewEmbeddedBroker(BrokerConfig{
		TCPAddress: ":0",
	}, nil, slog.Default())

	require.NoError(t, broker.Start())
	defer broker.Stop()

	assert.NotNil(t, broker.Server())
}
