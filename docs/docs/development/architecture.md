# Architecture

This guide explains how the system is structured, how components interact, and
how data flows through the application.

## System Overview

The system consists of sensors and a central hub that aggregates data, stores it, 
and serves a web UI.

```
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│  Pi + Sensor  │  │  Pi + Sensor  │  │  Pi + Sensor  │
│  (Flask API)  │  │  (Flask API)  │  │  (Flask API)  │
└──────┬───────┘  └──────┬───────┘  └──────┬───────┘
       │ HTTP GET /temperature        │                │
       └──────────────┬───────────────┘                │
                      │                                │
              ┌───────▼────────┐                       │
              │   Sensor Hub   │◄──────────────────────┘
              │   (Go binary)  │
              │                │◄───── MQTT ─────┐
              │  ┌──────────┐  │          ┌──────┴───────┐
              │  │  SQLite   │  │          │ Zigbee2MQTT  │
              │  └──────────┘  │          │ or other MQTT│
              │                │          │   sources    │
              │  ┌──────────┐  │          └──────────────┘
              │  │ React UI │  │  (embedded in the binary)
              │  └──────────┘  │
              └───────┬────────┘
                      │
              ┌───────▼────────┐
              │     Nginx      │  (optional TLS reverse proxy)
              └───────┬────────┘
                      │
              ┌───────▼────────┐
              │    Browser /   │
              │    CLI client  │
              └────────────────┘
```

Sensor Hub supports push-based data ingestion via MQTT. The connection
manager maintains persistent connections to configured MQTT brokers and routes
incoming messages through PushDriver implementations (e.g. Zigbee2MQTT) into
the same readings pipeline.

The Go binary embeds the built React SPA, so a single binary serves both the 
REST API and the frontend.

## Internal Layers

The Go backend follows a three-layer architecture:

```
  HTTP Request
      │
      ▼
┌─────────────────────────────────────────────┐
│  Router & Middleware (Gin)                   │
│  gin.Recovery → OTEL → Logger → CORS → CSRF │
│                                             │
│  Per-route: AuthRequired → RequirePermission │
└────────────────────┬────────────────────────┘
                     │
      ┌──────────────▼──────────────┐
      │  API Handlers (api/*.go)    │
      │  HTTP ↔ JSON, validation    │
      └──────────────┬──────────────┘
                     │
      ┌──────────────▼──────────────┐
      │  Services (service/*.go)    │
      │  Business logic, WebSocket  │
      │  broadcasts, alert checks   │
      └──────────────┬──────────────┘
                     │
      ┌──────────────▼──────────────┐
      │  Repositories (db/*.go)     │
      │  SQL queries, data mapping  │
      └──────────────┬──────────────┘
                     │
      ┌──────────────▼──────────────┐
      │  SQLite (WAL mode, FK on)   │
      └─────────────────────────────┘
```

## Application Entry Point

The `serve` command (`cmd/serve.go`) is the main entry point for the application.
## Reading pipeline

Every batch of readings goes through `readings.Pipeline.Process`, whichever way
it arrived:

```
Periodic collection (pull drivers) ──┐
On-demand collection (by name) ──────┼─► readings.Pipeline.Process
MQTT (after device intake in mqtt/) ─┘     │
                                           ├─ store the batch
                                           │    unknown measurement types are dropped with a warning
                                           │    storage failure marks the sensor's health Bad and stops here
                                           │
                                           └─ consumers, in order, with the stored readings:
                                                1. CommandTracker      acknowledges pending commands
                                                2. ThresholdProcessor  evaluates alert rules
                                                3. LiveView            WebSocket readings and sensor health
```

MQTT device intake stays in `mqtt/`: bridge device lists, device
identification, auto-discovery of pending sensors, the active and enabled
check, and message parsing all happen before the pipeline is called.

Consumers implement `readings.Consumer` and are registered in `cmd/serve.go`.
The order is the order they are passed to `readings.NewPipeline`, so adding a
consumer is one line of wiring there.
