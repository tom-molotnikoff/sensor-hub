package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Telemetry holds all OTel providers and the structured logger.
type Telemetry struct {
	Logger            *slog.Logger
	TracerProvider    *sdktrace.TracerProvider
	MeterProvider     *sdkmetric.MeterProvider
	LogProvider       *sdklog.LoggerProvider
	PrometheusHandler http.Handler
	logFile           *os.File
}

// Config holds telemetry configuration.
type Config struct {
	ServiceName string
	Version     string
	LogFilePath string
}

// Init initialises all telemetry providers.
func Init(ctx context.Context, cfg Config) (*Telemetry, error) {
	exportEnabled := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != ""

	serviceName := cfg.ServiceName
	if envName := os.Getenv("OTEL_SERVICE_NAME"); envName != "" {
		serviceName = envName
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(cfg.Version),
		),
		resource.WithHost(),
		resource.WithOS(),
		resource.WithProcess(),
	)
	if err != nil {
		return nil, err
	}

	// Log provider (for OTel log bridge)
	var logProvider *sdklog.LoggerProvider
	if exportEnabled {
		logExporter, err := otlploggrpc.New(ctx)
		if err != nil {
			return nil, err
		}
		logProvider = sdklog.NewLoggerProvider(
			sdklog.WithResource(res),
			sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		)
	}

	// Logger
	writer, logFile, err := LogWriter(cfg.LogFilePath)
	if err != nil {
		return nil, err
	}
	logger := NewLogger(writer, logProvider)

	// Tracer
	tp, err := initTracerProvider(ctx, res, exportEnabled)
	if err != nil {
		return nil, err
	}

	// Metrics
	mp, promHandler, err := initMeterProvider(ctx, res, exportEnabled)
	if err != nil {
		return nil, err
	}

	// Go runtime metrics (goroutines, memory, GC) via OTel SDK
	if err := runtime.Start(); err != nil {
		return nil, fmt.Errorf("failed to start runtime instrumentation: %w", err)
	}

	logger.Info("telemetry initialised",
		"service", serviceName,
		"version", cfg.Version,
		"log_level", logLevel.Level().String(),
		"otel_export", exportEnabled,
	)

	return &Telemetry{
		Logger:            logger,
		TracerProvider:    tp,
		MeterProvider:     mp,
		LogProvider:       logProvider,
		PrometheusHandler: promHandler,
		logFile:           logFile,
	}, nil
}

// Shutdown flushes all providers with a 5-second timeout.
func (t *Telemetry) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if t.TracerProvider != nil {
		_ = t.TracerProvider.Shutdown(ctx)
	}
	if t.MeterProvider != nil {
		_ = t.MeterProvider.Shutdown(ctx)
	}
	if t.LogProvider != nil {
		_ = t.LogProvider.Shutdown(ctx)
	}
	if t.logFile != nil {
		_ = t.logFile.Close()
	}
}
