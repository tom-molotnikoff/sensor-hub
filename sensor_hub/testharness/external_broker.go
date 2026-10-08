//go:build integration

package testharness

import (
	"fmt"
	"net"
	"strconv"
	"sync"

	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

// ExternalBroker is a plain MQTT broker that accepts anyone, standing in for
// an external broker such as Mosquitto on the home network. The hub reaches it
// over TCP like any external broker.
type ExternalBroker struct {
	Port     int
	server   *mochi.Server
	attempts *connectAttempts
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
	attempts := &connectAttempts{byPassword: map[string]int{}}
	if err := server.AddHook(attempts, nil); err != nil {
		return nil, fmt.Errorf("failed to add connect recording hook: %w", err)
	}
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
	return &ExternalBroker{Port: portNumber, server: server, attempts: attempts}, nil
}

func (b *ExternalBroker) Stop() error {
	return b.server.Close()
}

// ConnectAttempts counts the CONNECTs that presented the given password,
// accepted or not.
func (b *ExternalBroker) ConnectAttempts(password string) int {
	return b.attempts.count(password)
}

// Connected reports whether a client with the given ID holds an open
// connection to the broker.
func (b *ExternalBroker) Connected(clientID string) bool {
	client, ok := b.server.Clients.Get(clientID)
	return ok && !client.Closed()
}

// connectAttempts records the password of every CONNECT, before
// authentication.
type connectAttempts struct {
	mochi.HookBase
	mu         sync.Mutex
	byPassword map[string]int
}

func (h *connectAttempts) ID() string { return "connect-attempts" }

func (h *connectAttempts) Provides(b byte) bool { return b == mochi.OnConnect }

func (h *connectAttempts) OnConnect(_ *mochi.Client, pk packets.Packet) error {
	h.mu.Lock()
	h.byPassword[string(pk.Connect.Password)]++
	h.mu.Unlock()
	return nil
}

func (h *connectAttempts) count(password string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.byPassword[password]
}
