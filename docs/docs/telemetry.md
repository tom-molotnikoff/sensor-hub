# Telemetry

Sensor Hub includes built-in observability powered by [OpenTelemetry](https://opentelemetry.io/). It produces structured logs, distributed traces, and Prometheus-compatible metrics — all with zero external dependencies by default.

## Overview

| Signal      | Local (default)                   | With OTel Collector              |
|-------------|-----------------------------------|----------------------------------|
| **Logs**    | JSON to stdout + log file         | Also exported via OTLP           |
| **Traces**  | Not exported                      | Exported via OTLP gRPC           |
| **Metrics** | Prometheus endpoint at `http://127.0.0.1:9464/metrics` | Also exported via OTLP           |

When no collector is configured, Sensor Hub operates in **local-only mode** — logs go to stdout and the configured log file, and metrics are available at the Prometheus endpoint. No data leaves the machine.

## Configuration

### Log Level

Set the log level in `application.properties`:

```properties
log.level=info
```

Supported values: `debug`, `info`, `warn`, `error`. Default: `info`. An
unrecognised value logs at `info`. The level applies to the running process as
soon as the change is saved, with no restart.

### OpenTelemetry Collector

To export telemetry to an [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/), set these environment variables (in `/etc/sensor-hub/environment` for systemd):

```bash
# Collector endpoint (enables OTLP export for logs, traces, and metrics)
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317

# Protocol (default: grpc)
OTEL_EXPORTER_OTLP_PROTOCOL=grpc

# Override service name (default: sensor-hub)
OTEL_SERVICE_NAME=sensor-hub
```

The OTel SDK reads these environment variables natively. See the [OpenTelemetry environment variable specification](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/) for the full list.

## Structured Logging

Sensor Hub uses Go's `log/slog` with JSON output. Every log line includes something similar to the following structure:

```json
{
  "time": "2025-03-26T14:30:00.000Z",
  "level": "INFO",
  "msg": "sensor added",
  "component": "sensor_service",
  "sensor": "living-room"
}
```

Key attributes:
- `component` — which service produced the log (e.g., `sensor_service`, `auth_service`, `cleanup_service`)
- `trace_id` — present on HTTP request logs when tracing is active
- Context-specific fields like `sensor`, `username`, `error`

### Log File

When running via systemd, logs are written to `/var/log/sensor-hub/sensor-hub.log` (configured via `--log-file`). The included logrotate configuration rotates logs daily, keeping 14 days with compression.

## Distributed Tracing

When an OTel Collector is configured, Sensor Hub produces traces for:

- **HTTP requests** — via `otelgin` middleware (automatic span per request)
- **Database queries** — via `otelsql` instrumentation (span per query)
- **Sensor collection** — custom spans for periodic data collection
- **Background tasks** — cleanup, hourly averages, alert processing

Traces use the W3C Trace Context propagation format. The trace ID appears in HTTP request logs for correlation.

## Prometheus Metrics

Metrics are served at `GET /metrics` by a listener of their own, apart from the API. The endpoint has no authentication, so it is never on the API port: `GET /metrics` there returns 404, and the example nginx configuration does not proxy it. It is compatible with Prometheus, Grafana Agent, or any OpenMetrics-compatible scraper.

The listener's address is the `metrics.listen.address` property. The default, `127.0.0.1:9464`, takes scrapes from the same machine only. To scrape from another host, set it to an address that host can reach, such as `0.0.0.0:9464`, and limit who can reach that port with a firewall. Set it to empty to turn the endpoint off. A change applies when the service restarts.

### Prometheus Scrape Config

```yaml
scrape_configs:
  - job_name: sensor-hub
    static_configs:
      - targets: ['localhost:9464']
    metrics_path: /metrics
    scrape_interval: 15s
```

Replace `localhost:9464` with the hub's address and port when Prometheus runs on another host.

## Grafana Integration

Sensor Hub telemetry is designed to work with the Grafana observability stack:

1. **Logs** → Grafana Loki (via OTel Collector with Loki exporter)
2. **Traces** → Grafana Tempo (via OTel Collector with OTLP exporter)
3. **Metrics** → Prometheus → Grafana (direct scrape or via OTel Collector)

### Example Collector Config

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

exporters:
  loki:
    endpoint: http://loki:3100/loki/api/v1/push
  otlp/tempo:
    endpoint: tempo:4317
    tls:
      insecure: true
  prometheus:
    endpoint: 0.0.0.0:8889

service:
  pipelines:
    logs:
      receivers: [otlp]
      exporters: [loki]
    traces:
      receivers: [otlp]
      exporters: [otlp/tempo]
    metrics:
      receivers: [otlp]
      exporters: [prometheus]
```
