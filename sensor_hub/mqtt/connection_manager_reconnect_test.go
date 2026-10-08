package mqtt

import (
	"io"
	"log/slog"
	"testing"

	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestConnectionManager_TheEmbeddedRowFollowsItsEnabledFlag(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	embedded := NewEmbeddedBroker(BrokerConfig{TCPAddress: "127.0.0.1:0"}, nil, logger)
	require.NoError(t, embedded.Start())
	t.Cleanup(func() { _ = embedded.Stop() })

	id := 1
	enabled := gen.MQTTBroker{Id: &id, Name: "Embedded Broker", Type: "embedded", Enabled: true}
	disabled := enabled
	disabled.Enabled = false
	brokerRepo := &MockBrokerRepo{}
	brokerRepo.On("GetByID", mock.Anything, id).Return(&enabled, nil).Once()
	brokerRepo.On("GetByID", mock.Anything, id).Return(&disabled, nil).Once()
	subRepo := &MockSubRepo{}
	subRepo.On("GetEnabledByBrokerID", mock.Anything, id).Return([]gen.MQTTSubscription{}, nil)
	cm := NewConnectionManager(&MockSensorService{}, subRepo, brokerRepo, undecryptablePassword{}, embedded, logger)
	t.Cleanup(cm.Stop)

	cm.OnBrokerChanged(id)
	assert.True(t, cm.IsConnected(id), "enabling the row attaches the inline client")

	cm.OnBrokerChanged(id)
	assert.False(t, cm.IsConnected(id), "disabling the row detaches it")
	assert.Empty(t, cm.ConnectedBrokerIDs())
	_, ok := cm.Stats()[id]
	assert.True(t, ok, "a disabled broker keeps its statistics")

	cm.OnBrokerDeleted(id)
	_, ok = cm.Stats()[id]
	assert.False(t, ok, "a deleted broker's statistics go with it")
}
