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

The commands under `sensor-hub local` act on this machine's install and read its configuration directory. See [Local commands](cli-tool#local-commands) for each one.

| Command | What it does |
|---|---|
| `local serve` | Run the server |
| `local admin create <username>` | Create the first admin user |
| `local db backup <path>` | Write a consistent copy of the live database to a new file |

They accept the following flags:

| Flag           | Applies to            | Default           | Description |
|----------------|-----------------------|-------------------|-------------|
| `--config-dir` | every `local` command | `/etc/sensor-hub` | Path to the configuration directory. It must hold `application.properties` and `database.properties` |
| `--log-file`   | `local serve`         | stdout            | Path to the log file. The packaged systemd unit sets `/var/log/sensor-hub/sensor-hub.log` |

`sensor-hub --version` prints the version and exits.

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

### Hub timezone

`hub.timezone` is the zone [automation](automations) schedules run in, as an IANA zone name such as `Europe/London`. A change applies straight away. Its default is the server's own zone, or `UTC` when the server's zone has no IANA name, as in many Docker images. A value that is not a zone name Sensor Hub can load is rejected.

### Missed trigger grace window

`automation.missed.grace.minutes` decides what happens to an [automation](automations#restarts-and-the-grace-window) trigger that came due while the hub was down. When the hub starts no more than this many minutes after the trigger's due time, the run starts on startup. A trigger later than that is recorded as a `missed` run and does not run. The default is `10`. A change applies the next time the hub starts.

### Loop guard chain limit

`automation.loop.max.chain` is the longest chain of [automation](automations#loop-guard) runs that can start, where a reading that acknowledges each run's command started the next. The run that would make the chain longer is refused and recorded as `failed`. The default is `5`, and the value must be at least `1`. A change applies straight away.

### Automation run history retention

`automation.history.retention.days` is how long finished [automation](automations#runs-and-command-history) runs are kept, counted from when they finished. The cleanup task deletes older runs with their step outcomes. Running and waiting runs are never deleted. The default is `30`, and `0` keeps runs forever. A change applies from the next cleanup run, set by `data.cleanup.interval.hours`.

### Command history retention

`command.history.retention.days` is how long a sensor's command history is kept, counted from when each command was sent. It covers commands sent by people as well as by automations. The default is `90`, and `0` keeps command history forever. A change applies from the next cleanup run, set by `data.cleanup.interval.hours`.

### Readings aggregation

Readings aggregation is controlled by the `readings.aggregation.*` properties. Tier values use ISO 8601 durations in `THRESHOLD:INTERVAL` format. The special interval `raw` means no aggregation. Tiers are evaluated in ascending order - the first tier whose threshold is >= the query span is used. Queries exceeding all thresholds fall back to `P1D` buckets. See the [auto-aggregation developer docs](development/auto-aggregation.md) for details.

## Environment variables

Environment variables are defined in `/etc/sensor-hub/environment` and loaded by the systemd unit via `EnvironmentFile=`.

| Variable                    | Description                                                                            |
|-----------------------------|----------------------------------------------------------------------------------------|
| `SENSOR_HUB_ALLOWED_ORIGIN` | The allowed CORS origin for the web UI (e.g., `https://sensor-hub.example.com`)        |
