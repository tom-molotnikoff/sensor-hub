// Package mqtt provides an embedded MQTT broker powered by mochi-mqtt.
// The broker runs inside the Sensor Hub process and handles local MQTT
// traffic for push-based sensor drivers (Zigbee2MQTT, rtl_433, etc.).
package mqtt

import (
	"fmt"
	"log/slog"
	"sync"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

// BrokerConfig holds the configuration for the embedded MQTT broker.
type BrokerConfig struct {
	TCPAddress string // e.g. "127.0.0.1:1883"
	// ConnectRateLimit is the most CONNECTs accepted in one second across all
	// clients. 0 turns the limit off.
	ConnectRateLimit int
}

// EmbeddedBroker wraps a mochi-mqtt server instance with lifecycle management.
// Devices connect to it with an MQTT client's credentials; the hub itself
// uses the inline client and needs none.
type EmbeddedBroker struct {
	server        *mqtt.Server
	config        BrokerConfig
	authenticator Authenticator
	logger        *slog.Logger
	running       bool
	mu            sync.Mutex
}

// NewEmbeddedBroker creates a new embedded broker but does not start it.
func NewEmbeddedBroker(config BrokerConfig, authenticator Authenticator, logger *slog.Logger) *EmbeddedBroker {
	return &EmbeddedBroker{
		config:        config,
		authenticator: authenticator,
		logger:        logger.With("component", "embedded_broker"),
	}
}

// Start initialises the mochi-mqtt server, adds a TCP listener, and begins
// serving. The server runs in a background goroutine; call Stop to shut it down.
func (b *EmbeddedBroker) Start() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return fmt.Errorf("embedded broker is already running")
	}

	b.server = mqtt.New(&mqtt.Options{
		InlineClient: true,
		Logger:       newBrokerLogger(b.logger),
	})

	// The rate hook goes first, so a CONNECT it refuses is never authenticated.
	if b.config.ConnectRateLimit > 0 {
		if err := b.server.AddHook(newConnectRateHook(b.server, b.config.ConnectRateLimit, b.logger), nil); err != nil {
			return fmt.Errorf("failed to add connect rate hook: %w", err)
		}
	}
	if err := b.server.AddHook(newClientAuthHook(b.server, b.authenticator, b.logger), nil); err != nil {
		return fmt.Errorf("failed to add auth hook: %w", err)
	}

	tcp := listeners.NewTCP(listeners.Config{
		ID:      "sensor-hub-tcp",
		Address: b.config.TCPAddress,
	})
	if err := b.server.AddListener(tcp); err != nil {
		return fmt.Errorf("failed to add TCP listener on %s: %w", b.config.TCPAddress, err)
	}

	go func() {
		if err := b.server.Serve(); err != nil {
			b.logger.Error("embedded MQTT broker error", "error", err)
		}
	}()

	b.running = true
	b.logger.Info("embedded MQTT broker started", "address", b.config.TCPAddress)
	return nil
}

// Stop gracefully shuts down the embedded broker.
func (b *EmbeddedBroker) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return nil
	}

	if err := b.server.Close(); err != nil {
		return fmt.Errorf("failed to stop embedded broker: %w", err)
	}

	b.running = false
	b.logger.Info("embedded MQTT broker stopped")
	return nil
}

// IsRunning returns whether the broker is currently running.
func (b *EmbeddedBroker) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

// Server returns the underlying mochi-mqtt server instance, through whose
// inline client the connection manager publishes and subscribes.
func (b *EmbeddedBroker) Server() *mqtt.Server {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.server
}

// ConnectedUsernames returns the username of every client connected now.
func (b *EmbeddedBroker) ConnectedUsernames() map[string]bool {
	connected := make(map[string]bool)
	for _, cl := range b.liveClients() {
		connected[string(cl.Properties.Username)] = true
	}
	return connected
}

// Disconnect closes every live connection made with the username.
func (b *EmbeddedBroker) Disconnect(username string) {
	b.mu.Lock()
	server := b.server
	b.mu.Unlock()
	for _, cl := range b.liveClients() {
		if string(cl.Properties.Username) != username {
			continue
		}
		_ = server.DisconnectClient(cl, packets.ErrAdministrativeAction)
		b.logger.Info("disconnected MQTT client", "username", username, "remote", cl.Net.Remote)
	}
}

// liveClients returns the open connections of authenticated clients, leaving
// out the inline client and sessions kept for clients that have gone.
func (b *EmbeddedBroker) liveClients() []*mqtt.Client {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.running {
		return nil
	}
	var clients []*mqtt.Client
	for _, cl := range b.server.Clients.GetAll() {
		if !cl.Net.Inline && !cl.Closed() {
			clients = append(clients, cl)
		}
	}
	return clients
}
