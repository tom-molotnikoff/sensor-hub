package telemetry

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

// multiHandler fans out log records to multiple slog handlers, filtering on
// the process log level first. The filter belongs here rather than in each
// handler because the OTel bridge accepts every level it is offered, so
// without it a record below the configured level would still be built and
// shipped to the collector.
type multiHandler struct {
	level    slog.Leveler
	handlers []slog.Handler
}

func newMultiHandler(level slog.Leveler, handlers ...slog.Handler) *multiHandler {
	return &multiHandler{level: level, handlers: handlers}
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if level < m.level.Level() {
		return false
	}
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, record.Level) {
			if err := h.Handle(ctx, record); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return newMultiHandler(m.level, handlers...)
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return newMultiHandler(m.level, handlers...)
}

// logLevel is the level every logger built by [NewLogger] consults per record,
// so a configuration reload can change what the running process logs at
// without a restart. It reads as info until something sets it.
var logLevel = new(slog.LevelVar)

// SetLogLevel points every logger in the process at the named level.
// An unrecognised name falls back to info.
func SetLogLevel(level string) {
	logLevel.Set(parseLogLevel(level))
}

// parseLogLevel converts a string log level to slog.Level.
func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// NewLogger creates a structured slog.Logger.
// It writes JSON to the provided writer and optionally bridges to an OTel LoggerProvider.
// The logger filters on the process log level, which [SetLogLevel] can change
// at any point in its life.
func NewLogger(writer io.Writer, logProvider *sdklog.LoggerProvider) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: true,
	}

	jsonHandler := slog.NewJSONHandler(writer, opts)

	var handler slog.Handler
	if logProvider != nil {
		otelHandler := otelslog.NewHandler("sensor-hub", otelslog.WithLoggerProvider(logProvider))
		handler = newMultiHandler(logLevel, jsonHandler, otelHandler)
	} else {
		handler = jsonHandler
	}

	return slog.New(handler)
}

// LogWriter returns an io.Writer that writes to stdout and optionally a log file.
func LogWriter(logFilePath string) (io.Writer, *os.File, error) {
	if logFilePath == "" {
		return os.Stdout, nil, nil
	}
	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, nil, err
	}
	return io.MultiWriter(os.Stdout, f), f, nil
}
