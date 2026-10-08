package mqtt

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"sync"

	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

// A stranger can make mochi log once for each connection it opens, without
// authenticating: the listener warns when a connection it was given ends in
// an error, refused CONNECTs included, and the server warns of a CONNECT's
// short keepalive before any hook has run. At warn, a CONNECT flood would
// flood the journal, and journald's rate limit would then drop the hub's own
// logs. The types here log those at debug instead. A connection that
// authenticated and then ends in an error, such as a home device dropping,
// is still logged at warn. The hub's hooks count every refusal and log it at
// debug either way.

// authenticatedConns records the connections that passed authentication, so
// the listener can tell how one that ended in an error should be logged.
type authenticatedConns struct {
	mochi.HookBase
	conns sync.Map // net.Conn → struct{}
}

func (h *authenticatedConns) ID() string {
	return "sensor-hub-authenticated-conns"
}

func (h *authenticatedConns) Provides(b byte) bool {
	return bytes.Contains([]byte{mochi.OnSessionEstablish}, []byte{b})
}

// OnSessionEstablish runs once a CONNECT has been authenticated, before the
// broker lists the client and acknowledges it.
func (h *authenticatedConns) OnSessionEstablish(cl *mochi.Client, pk packets.Packet) {
	if cl.Net.Conn != nil {
		h.conns.Store(cl.Net.Conn, struct{}{})
	}
}

// forget reports whether the connection authenticated, and stops tracking it.
func (h *authenticatedConns) forget(conn net.Conn) bool {
	_, ok := h.conns.LoadAndDelete(conn)
	return ok
}

// quietRefusalsListener wraps a mochi listener so that a connection which
// ends in an error before it authenticated is logged at debug here, rather
// than at warn by the listener. Errors from authenticated connections reach
// the listener as before.
type quietRefusalsListener struct {
	listeners.Listener
	authenticated *authenticatedConns
	logger        *slog.Logger
}

func (l *quietRefusalsListener) Serve(establish listeners.EstablishFn) {
	l.Listener.Serve(func(id string, conn net.Conn) error {
		err := establish(id, conn)
		authenticated := l.authenticated.forget(conn)
		if err != nil && !authenticated {
			l.logger.Debug("MQTT connection ended before it authenticated", "remote", conn.RemoteAddr().String(), "error", err)
			return nil
		}
		return err
	})
}

// brokerLogHandler passes mochi's log records to the hub's logger, demoting
// the short-keepalive warning, which mochi writes for every such CONNECT
// before any hook has run, to debug.
type brokerLogHandler struct {
	slog.Handler
}

func newBrokerLogger(logger *slog.Logger) *slog.Logger {
	return slog.New(brokerLogHandler{logger.Handler()})
}

func (h brokerLogHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level == slog.LevelWarn && r.Message == mochi.ErrMinimumKeepalive.Error() {
		if !h.Handler.Enabled(ctx, slog.LevelDebug) {
			return nil
		}
		r.Level = slog.LevelDebug
	}
	return h.Handler.Handle(ctx, r)
}

func (h brokerLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return brokerLogHandler{h.Handler.WithAttrs(attrs)}
}

func (h brokerLogHandler) WithGroup(name string) slog.Handler {
	return brokerLogHandler{h.Handler.WithGroup(name)}
}
