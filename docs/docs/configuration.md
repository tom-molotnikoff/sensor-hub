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

- Applied immediately in memory
- Saved back to the configuration files asynchronously
- Broadcast to all connected UI clients via WebSocket

This means changes take effect without restarting the service.

## Application properties

| Property                               | Default                            | Description                                                                        |
|----------------------------------------|------------------------------------|------------------------------------------------------------------------------------|
| `sensor.collection.interval`           | `300`                              | Seconds between sensor polling cycles                                              |
| `sensor.discovery.skip`                | `true`                             | When set to `true`, skips automatic sensor discovery on startup                    |
| `openapi.yaml.location`                | `./docker_tests/openapi.yaml`      | Path to openapi.yaml file for auto-discovery of sensors (used in development)      |
| `health.history.retention.days`        | `30`                               | Number of days to retain sensor health history records                             |
| `sensor.data.retention.days`           | `90`                               | Number of days to retain temperature reading data                                  |
| `failed.login.retention.days`          | `2`                                | Number of days to retain failed login attempt records                              |
| `alert.history.retention.days`         | `90`                               | Number of days to retain alert sent history records                                |
| `data.cleanup.interval.hours`          | `1`                                | Hours between data cleanup runs                                                    |
| `auth.bcrypt.cost`                     | `12`                               | Bcrypt cost factor for password hashing (higher values are more secure but slower) |
| `auth.session.ttl.minutes`             | `43200`                            | Session duration in minutes (default is 30 days)                                   |
| `auth.session.cookie.name`             | `sensor_hub_session`               | Name of the session cookie                                                         |
| `auth.login.backoff.window.minutes`    | `15`                               | Time window in minutes for counting failed login attempts                          |
| `auth.login.backoff.threshold`         | `5`                                | Number of failed login attempts before backoff is applied                          |
| `auth.login.backoff.base.seconds`      | `2`                                | Base duration in seconds for exponential login backoff                             |
| `auth.login.backoff.max.seconds`       | `300`                              | Maximum backoff duration in seconds                                                |
| `log.level`                            | `info`                             | Logging level (e.g., debug, info, warn, error)                                     |
| `oauth.credentials.file.path`          | `/etc/sensor-hub/credentials.json` | Path to the Google OAuth credentials file                                          |
| `oauth.token.file.path`                | `/etc/sensor-hub/token.json`       | Path to the stored OAuth token file                                                |
| `oauth.token.refresh.interval.minutes` | `30`                               | Interval in minutes for background OAuth token refresh                             |
| `weather.latitude`                     | `53.383`                           | Latitude for weather data                                                          |
| `weather.location.name`                | `Sheffield`                        | Name of the location for weather data (used in UI)                                 |
| `weather.longitude`                    | `-1.4659`                          | Longitude for weather data                                                         |

## Readings aggregation properties

These properties control automatic aggregation of readings for charting. Aggregation is configured through tier rules that map time span thresholds to bucket intervals. See the [auto-aggregation developer docs](development/auto-aggregation.md) for details.

| Property                               | Default  | Description                                                                    |
|----------------------------------------|----------|--------------------------------------------------------------------------------|
| `readings.aggregation.enabled`         | `true`   | Enable automatic readings aggregation. When `false`, all queries return raw data. |
| `readings.aggregation.tiers`           | `PT15M:raw,PT1H:PT10S,PT6H:PT1M,P1D:PT5M,P7D:PT15M,P30D:PT1H` | Comma-separated tier rules mapping time span thresholds to bucket intervals. Format: `THRESHOLD:INTERVAL,...` |

Tier values use ISO 8601 durations in `THRESHOLD:INTERVAL` format. The special interval `raw` means no aggregation. Tiers are evaluated in ascending order — the first tier whose threshold is ≥ the query span is used. Queries exceeding all thresholds fall back to `P1D` buckets.


## Database properties

| Property        | Type   | Default                              | Description                      |
|-----------------|--------|--------------------------------------|----------------------------------|
| `database.path` | string | `/var/lib/sensor-hub/sensor_hub.db`  | Path to the SQLite database file |

## SMTP properties

| Property    | Description                                              |
|-------------|----------------------------------------------------------|
| `smtp.user` | Gmail address used as the sender for email notifications |

## Environment variables

Environment variables are defined in `/etc/sensor-hub/environment` and loaded by the systemd unit via `EnvironmentFile=`.

| Variable                    | Description                                                                            |
|-----------------------------|----------------------------------------------------------------------------------------|
| `SENSOR_HUB_INITIAL_ADMIN`  | Creates an initial admin user on first startup; format is `username:password`          |
| `SENSOR_HUB_ALLOWED_ORIGIN` | The allowed CORS origin for the web UI (e.g., `https://sensor-hub.example.com`)        |
