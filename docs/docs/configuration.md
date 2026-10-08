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
| `secrets.key`          | The secret-store key, unless it is kept elsewhere (see [Secret-store key](#secret-store-key)) |
| `secrets.key.cred`     | The secret-store key sealed with the TPM, when sealed |

## CLI flags

The commands under `sensor-hub local` act on this machine's install and read its configuration directory. See [Local commands](cli-tool#local-commands) for each one.

| Command | What it does |
|---|---|
| `local serve` | Run the server |
| `local admin create <username>` | Create the first admin user |
| `local db backup <path>` | Write a consistent copy of the live database to a new file |
| `local secrets init-key` | Create the secret-store key |
| `local secrets show-key` | Print the secret-store key |

They accept the following flags:

| Flag           | Applies to            | Default           | Description |
|----------------|-----------------------|-------------------|-------------|
| `--config-dir` | every `local` command | `/etc/sensor-hub` | Path to the configuration directory. It must hold `application.properties` and `database.properties` |
| `--log-file`   | `local serve`         | stdout            | Path to the log file. The packaged systemd unit sets `/var/log/sensor-hub/sensor-hub.log` |
| `--secrets-key-file` | `local serve`, `local secrets show-key` | none | Path to the secret-store key, read when there is no systemd credential or Compose secret. See [Secret-store key](#secret-store-key) |
| `--from-stdin` | `local secrets init-key` | off | Read the key from stdin rather than generating one |
| `--seal`       | `local secrets init-key` | off | Seal the key with the TPM through `systemd-creds` (root only) |

`sensor-hub --version` prints the version and exits.

These flags are useful for running sensor-hub outside the standard package layout (e.g., during development).

## Secret-store key

The credentials the hub presents to other systems, such as outbound MQTT broker passwords, are stored in the database encrypted with AES-256-GCM. The key that decrypts them lives outside the database, so a copy of the database or a backup gives nothing usable on its own. No API response and no command other than `local secrets show-key` reveals a stored secret or the key.

The key is 32 random bytes, held as one line of standard base64. When the hub starts it uses the first key it finds, in this order:

1. `$CREDENTIALS_DIRECTORY/secrets.key`, a systemd credential. A key sealed with `local secrets init-key --seal` reaches the hub this way.
2. `/run/secrets/sensor-hub-secrets-key`, a Compose secret named `sensor-hub-secrets-key`.
3. The path given with `--secrets-key-file` to `local serve`. The file must exist; the hub does not fall back to another location when it is missing.
4. `<config-dir>/secrets.key`, which is `/etc/sensor-hub/secrets.key` on a package install.

When there is no key at any of them, the hub generates one, writes it to `<config-dir>/secrets.key` with mode 0600, logs that it did and where, and carries on. It refuses to do so when a sealed key, `<config-dir>/secrets.key.cred`, exists without systemd passing it in, since secrets stored under a second key would be unreadable under the unit.

A key file given with `--secrets-key-file` or found in the configuration directory must meet two rules, or the hub logs which one it broke and exits with an error:

- It is not readable by others: its mode has no `o+r` bit, so `chmod 600` it.
- It is not inside the directory that holds `database.path`, at any depth, so that copying the database directory never carries the key along.

A systemd credential and a Compose secret are exempt, because their manager puts them in place.

Keep a copy of the key, from `sudo sensor-hub local secrets show-key`, somewhere other than the hub, such as a password manager. If the key is lost or replaced, the stored secrets cannot be decrypted and have to be entered again.

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

### HTTP listen address

`http.listen.address` is the host and port the HTTP API, WebSocket and web UI listen on. The default is `127.0.0.1:8080`, which takes connections from the same machine only: on a packaged install that is nginx, and nothing else on the network reaches the hub directly. To serve the hub without nginx in front of it, set an address other hosts can reach, such as `0.0.0.0:8080`. A value that is not `host:port` is rejected. A change applies when the service restarts.

### Trusted proxies

`http.trusted.proxies` is a comma-separated list of IP addresses and CIDR ranges, such as `127.0.0.1,::1` or `10.0.0.0/8`, of the reverse proxies in front of the hub. The hub believes the `X-Forwarded-For` and `X-Real-IP` headers only on a request that comes from one of them, and then takes the client's address as the rightmost address in `X-Forwarded-For` that is not a trusted proxy. A client can put any address it likes at the front of that header, so the forged part is never used.

The default is empty, which trusts no proxy: the client's address is the address the connection came from, and both headers are ignored. The packaged `application.properties` sets `127.0.0.1,::1`, for nginx on the same machine. Behind nginx with an empty list, every request appears to come from nginx, so one person failing to log in puts everyone under the login backoff. A value that is not a list of addresses and ranges is rejected. A change applies when the service restarts. See [Nginx Setup](nginx-setup#client-addresses) for nginx on another host.

### Metrics listen address

`metrics.listen.address` is the host and port the Prometheus `/metrics` endpoint listens on, on a listener of its own apart from the API. The default is `127.0.0.1:9464`. Empty turns the endpoint off, and any other value that is not `host:port` is rejected. A change applies when the service restarts. See [Telemetry](telemetry#prometheus-metrics) for the scrape configuration.

### Readings aggregation

Readings aggregation is controlled by the `readings.aggregation.*` properties. Tier values use ISO 8601 durations in `THRESHOLD:INTERVAL` format. The special interval `raw` means no aggregation. Tiers are evaluated in ascending order - the first tier whose threshold is >= the query span is used. Queries exceeding all thresholds fall back to `P1D` buckets. See the [auto-aggregation developer docs](development/auto-aggregation.md) for details.

## Environment variables

Environment variables are defined in `/etc/sensor-hub/environment` and loaded by the systemd unit via `EnvironmentFile=`.

| Variable                    | Description                                                                            |
|-----------------------------|----------------------------------------------------------------------------------------|
| `SENSOR_HUB_ALLOWED_ORIGIN` | An origin other than the hub's own that may call the API and open WebSockets, such as a UI dev server (e.g., `http://localhost:5173`). A browser WebSocket from any other origin is refused with 403 |
