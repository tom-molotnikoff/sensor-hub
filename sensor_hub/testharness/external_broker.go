//go:build integration

package testharness

import (
	"fmt"
	"net"
	"strconv"

	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
)

// ExternalBroker is a plain MQTT broker that accepts anyone, standing in for
// an external broker such as Mosquitto on the home network. The hub reaches it
// over TCP like any external broker.
type ExternalBroker struct {
	Port   int
	server *mochi.Server
}

func StartExternalBroker() (*ExternalBroker, error) {
	server := mochi.New(nil)
	if err := server.AddHook(new(auth.AllowHook), nil); err != nil {
		return nil, fmt.Errorf("failed to add allow-all hook: %w", err)
	}
	return serveExternalBroker(server)
}

// StartExternalBrokerWithLogin starts an external broker that accepts only
// the given username and password, as a Mosquitto with a password file does.
func StartExternalBrokerWithLogin(username, password string) (*ExternalBroker, error) {
	server := mochi.New(nil)
	ledger := &auth.Ledger{
		Auth: auth.AuthRules{{Username: auth.RString(username), Password: auth.RString(password), Allow: true}},
		ACL:  auth.ACLRules{{Username: auth.RString(username), Filters: auth.Filters{"#": auth.ReadWrite}}},
	}
	if err := server.AddHook(new(auth.Hook), &auth.Options{Ledger: ledger}); err != nil {
		return nil, fmt.Errorf("failed to add auth hook: %w", err)
	}
	return serveExternalBroker(server)
}

func serveExternalBroker(server *mochi.Server) (*ExternalBroker, error) {
	tcp := listeners.NewTCP(listeners.Config{ID: "external-broker", Address: "127.0.0.1:0"})
	if err := server.AddListener(tcp); err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}
	go func() { _ = server.Serve() }()
	_, port, err := net.SplitHostPort(tcp.Address())
	if err != nil {
		_ = server.Close()
		return nil, err
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil {
		_ = server.Close()
		return nil, err
	}
	return &ExternalBroker{Port: portNumber, server: server}, nil
}

func (b *ExternalBroker) Stop() error {
	return b.server.Close()
}
