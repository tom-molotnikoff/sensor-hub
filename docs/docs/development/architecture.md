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