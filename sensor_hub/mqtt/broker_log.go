package mqtt

import (
	"context"
	"log/slog"

	mochi "github.com/mochi-mqtt/server/v2"
)

// brokerLogHandler passes mochi's log records to the hub's logger, demoting to
// debug the warnings mochi writes for each connection it does not establish.
// A stranger can cause one of those per connection without authenticating, so
// at warn a CONNECT flood would flood the journal, and journald's rate limit
// would then drop the hub's own logs. The hub's hooks still count every
// refusal and log it at debug.
type brokerLogHandler struct {
	slog.Handler
}

func newBrokerLogger(logger *slog.Logger) *slog.Logger {
	return slog.New(brokerLogHandler{logger.Handler()})
}

func (h brokerLogHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level == slog.LevelWarn && isPerConnectionWarning(r.Message) {
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

// isPerConnectionWarning recognises mochi's warnings about a single incoming
// connection: its listeners log a connection that ends before it is
// established, refused ones included, with an empty message, and it warns of
// a CONNECT's short keepalive before any hook has run.
func isPerConnectionWarning(message string) bool {
	return message == "" || message == mochi.ErrMinimumKeepalive.Error()
}
