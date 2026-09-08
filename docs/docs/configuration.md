---
id: configuration
title: Configuration Settings
sidebar_position: 10
---

# Configuration Settings

Sensor Hub is configured through property files, environment variables, and CLI flags. Properties can be updated at runtime through the web UI or the REST API.

## Configuration files

Configuration files are located in `/etc/sensor-hub/`. There are three property files:

| File                     | Purpose                                                                  |
|--------------------------|--------------------------------------------------------------------------|
| `application.properties` | Application behavior, sensor polling, authentication, and OAuth settings |
| `database.properties`    | Database connection details                                              |
| `smtp.properties`        | Email sender configuration                                               |

Files use a simple `KEY=VALUE` format, one property per line.

Additional files in `/etc/sensor-hub/`:

| File                   | Purpose                                           |
|------------------------|---------------------------------------------------|
| `environment`          | Environment variables loaded by the systemd unit  |
| `credentials.json`     | Google OAuth credentials (email alerts)           |
| `token.json`           | Stored OAuth token (created during authorization) |
| `nginx.conf.example`   | Example nginx reverse proxy configuration         |

## CLI flags

The `sensor-hub` binary accepts the following flags:

| Flag             | Default                              | Description                         |
|------------------|--------------------------------------|-------------------------------------|
| `--config-dir`   | `/etc/sensor-hub`                    | Path to the configuration directory |
| `--log-file`     | `/var/log/sensor-hub/sensor-hub.log` | Path to the log file                |
| `--version`      | —                                    | Print version and exit              |

These flags are useful for running sensor-hub outside the standard package layout (e.g., during development).

## Runtime configuration updates

Properties can be updated at runtime through the Properties page in the web UI or via the `PATCH /api/properties` API endpoint. Runtime updates are:

- Applied in memory
- Saved back to the configuration files asynchronously
- Broadcast to all connected UI clients via WebSocket

Most changes take effect without restarting the service. Where a property needs a restart or another action before it applies, the Properties page says so next to the property.

## Property reference

The Properties page in the web UI is the reference for every property: it shows each property's description, default, unit, and whether a change applies live. Descriptions and defaults come from the configuration registry in the Go source, so the page is always in step with the running binary. See the [configuration package docs](development/configuration-package.md) for how the registry is defined.

If the page cannot load the property definitions, it still lists and saves every property, but shows raw keys and plain text fields with a banner explaining that descriptions and typed controls are unavailable.

Readings aggregation is controlled by the `readings.aggregation.*` properties. Tier values use ISO 8601 durations in `THRESHOLD:INTERVAL` format. The special interval `raw` means no aggregation. Tiers are evaluated in ascending order - the first tier whose threshold is >= the query span is used. Queries exceeding all thresholds fall back to `P1D` buckets. See the [auto-aggregation developer docs](development/auto-aggregation.md) for details.

## Environment variables

Environment variables are defined in `/etc/sensor-hub/environment` and loaded by the systemd unit via `EnvironmentFile=`.

| Variable                    | Description                                                                            |
|-----------------------------|----------------------------------------------------------------------------------------|
| `SENSOR_HUB_INITIAL_ADMIN`  | Creates an initial admin user on first startup; format is `username:password`          |
| `SENSOR_HUB_ALLOWED_ORIGIN` | The allowed CORS origin for the web UI (e.g., `https://sensor-hub.example.com`)        |
