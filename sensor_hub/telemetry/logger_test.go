package telemetry_test

import (
	"context"
	"io"
	"sync"
	"testing"

	sdklog "go.opentelemetry.io/otel/sdk/log"

	"example/sensorHub/telemetry"

	"github.com/stretchr/testify/assert"
)

// recordingExporter stands in for a collector, capturing the message of every
// record that reaches the export pipeline.
type recordingExporter struct {
	mu       sync.Mutex
	messages []string
}

func (r *recordingExporter) Export(_ context.Context, records []sdklog.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, record := range records {
		r.messages = append(r.messages, record.Body().AsString())
	}
	return nil
}

func (r *recordingExporter) Shutdown(context.Context) error { return nil }

func (r *recordingExporter) ForceFlush(context.Context) error { return nil }

func (r *recordingExporter) exported() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.messages...)
}

func newRecordingProvider(exporter *recordingExporter) *sdklog.LoggerProvider {
	return sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)))
}

func TestNewLogger_ExportsNothingBelowTheConfiguredLevel(t *testing.T) {
	telemetry.SetLogLevel("info")
	defer telemetry.SetLogLevel("info")

	exporter := &recordingExporter{}
	logger := telemetry.NewLogger(io.Discard, newRecordingProvider(exporter))

	logger.Debug("a line only debug emits")
	logger.Info("a line info emits")

	assert.Equal(t, []string{"a line info emits"}, exporter.exported())
}

func TestNewLogger_ExportsDebugOnceTheLevelAllowsIt(t *testing.T) {
	telemetry.SetLogLevel("info")
	defer telemetry.SetLogLevel("info")

	exporter := &recordingExporter{}
	logger := telemetry.NewLogger(io.Discard, newRecordingProvider(exporter))

	telemetry.SetLogLevel("debug")
	logger.Debug("a line only debug emits")

	assert.Equal(t, []string{"a line only debug emits"}, exporter.exported())
}
