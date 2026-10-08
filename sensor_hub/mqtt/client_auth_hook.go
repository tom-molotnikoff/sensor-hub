package mqtt

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"example/sensorHub/service"
	"example/sensorHub/telemetry"

	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Authenticator checks connections against the MQTT clients an admin created.
type Authenticator interface {
	// Authenticate checks a CONNECT's credentials. It returns the topic prefix
	// the connection is limited to, or why the CONNECT is refused.
	Authenticate(ctx context.Context, username string, password []byte) (string, service.MQTTConnectRefusal)
	// IsEnabled reports whether the client still exists and is enabled.
	IsEnabled(ctx context.Context, username string) bool
}

// clientAuthHook ends anonymous access to the embedded broker: every CONNECT
// must carry a known, enabled client's credentials, and the connection may
// then only publish and subscribe under that client's topic prefix. The hub's
// own inline client is not subject to it.
type clientAuthHook struct {
	mochi.HookBase
	server        *mochi.Server
	authenticator Authenticator
	refused       metric.Int64Counter
	logger        *slog.Logger
	prefixes      sync.Map // *mochi.Client → topic prefix
}

func newClientAuthHook(server *mochi.Server, authenticator Authenticator, logger *slog.Logger) *clientAuthHook {
	refused, _ := telemetry.Meter("mqtt").Int64Counter("sensor_hub.mqtt.connect.refused",
		metric.WithDescription("CONNECTs the embedded broker refused, by reason"),
		metric.WithUnit("{connection}"))
	return &clientAuthHook{server: server, authenticator: authenticator, refused: refused, logger: logger}
}

func (h *clientAuthHook) ID() string {
	return "sensor-hub-client-auth"
}

func (h *clientAuthHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mochi.OnConnect, mochi.OnConnectAuthenticate, mochi.OnSessionEstablished, mochi.OnACLCheck, mochi.OnDisconnect,
	}, []byte{b})
}

// OnConnect authenticates the CONNECT. It is done here rather than in
// OnConnectAuthenticate because mochi answers a failed OnConnectAuthenticate
// with "bad username or password" on v5, and the refusal is "not authorized".
func (h *clientAuthHook) OnConnect(cl *mochi.Client, pk packets.Packet) error {
	username := string(cl.Properties.Username)
	prefix, refusal := h.authenticator.Authenticate(context.Background(), username, pk.Connect.Password)
	if refusal == "" && !h.claimClientID(cl.ID, username) {
		refusal = service.MQTTConnectRefusedAuth
	}
	if refusal != "" {
		h.refused.Add(context.Background(), 1, metric.WithAttributes(attribute.String("reason", string(refusal))))
		h.logger.Debug("refused MQTT CONNECT", "username", username, "client_id", cl.ID, "reason", refusal, "remote", cl.Net.Remote)
		if err := h.server.SendConnack(cl, notAuthorized(cl), false, nil); err != nil {
			h.logger.Debug("failed to send MQTT CONNACK refusal", "username", username, "error", err)
		}
		return packets.ErrNotAuthorized
	}
	if atomic.LoadUint32(&cl.Properties.Will.Flag) == 1 && !strings.HasPrefix(cl.Properties.Will.TopicName, prefix) {
		// Mochi publishes a will without an ACL check, so one outside the
		// prefix is dropped here, as any other publish outside it would be.
		atomic.StoreUint32(&cl.Properties.Will.Flag, 0)
		h.logger.Debug("dropped MQTT will outside the client's prefix", "username", username, "topic", cl.Properties.Will.TopicName)
	}
	h.prefixes.Store(cl, prefix)
	return nil
}

// claimClientID stops one credential reaching another's session through a
// shared client ID: mochi would disconnect that session's holder and hand over
// its subscriptions and queued messages without a prefix check. It refuses
// the ID while another username's session, or the hub's inline client, holds
// it live, and discards another username's kept session so nothing is
// inherited from it.
func (h *clientAuthHook) claimClientID(clientID, username string) bool {
	existing, ok := h.server.Clients.Get(clientID)
	if !ok || string(existing.Properties.Username) == username {
		return true
	}
	if existing.Net.Inline || !existing.Closed() {
		return false
	}
	h.server.UnsubscribeClient(existing)
	existing.ClearInflights()
	h.server.Clients.Delete(clientID)
	return true
}

// notAuthorized is the CONNACK code for a refused CONNECT: 0x87 on v5. Mochi
// rewrites "bad username or password" to 0x05, Not authorized, on v3.1.1.
func notAuthorized(cl *mochi.Client) packets.Code {
	if cl.Properties.ProtocolVersion < 5 {
		return packets.ErrBadUsernameOrPassword
	}
	return packets.ErrNotAuthorized
}

// OnConnectAuthenticate accepts every CONNECT that reaches it, because
// OnConnect has already refused the rest.
func (h *clientAuthHook) OnConnectAuthenticate(cl *mochi.Client, pk packets.Packet) bool {
	return true
}

// OnSessionEstablished checks the client again once the broker lists it. A
// client disabled or deleted while its CONNECT was being authenticated was
// missed by the disconnect, which only sees listed clients.
func (h *clientAuthHook) OnSessionEstablished(cl *mochi.Client, pk packets.Packet) {
	username := string(cl.Properties.Username)
	if h.authenticator.IsEnabled(context.Background(), username) {
		return
	}
	h.logger.Info("disconnecting MQTT client disabled while connecting", "username", username)
	_ = h.server.DisconnectClient(cl, packets.ErrAdministrativeAction)
}

// OnACLCheck allows a publish, subscription or delivery only when the topic or
// filter starts with the client's prefix literally, so "p/#" is allowed for
// "p/" and "#" is not.
func (h *clientAuthHook) OnACLCheck(cl *mochi.Client, topic string, write bool) bool {
	prefix, ok := h.prefixes.Load(cl)
	if ok && strings.HasPrefix(topic, prefix.(string)) {
		return true
	}
	h.logger.Debug("refused MQTT topic outside the client's prefix",
		"username", string(cl.Properties.Username), "topic", topic, "publish", write)
	return false
}

func (h *clientAuthHook) OnDisconnect(cl *mochi.Client, err error, expire bool) {
	h.prefixes.Delete(cl)
}
