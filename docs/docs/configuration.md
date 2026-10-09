---
id: configuration
title: Configuration Settings
sidebar_position: 10
---

# Configuration Settings

Sensor Hub is configured through property files, environment variables, and CLI flags. Properties can be updated at runtime through the web UI or the REST API.

## Configuration files

Configuration files are located in `/etc/sensor-hub/`. There are two property files:

| File                     | Purpose                                                    |
|--------------------------|------------------------------------------------------------|
| `application.properties` | Application behavior, sensor polling and authentication    |
| `database.properties`    | Database connection details                                |

The email settings are not in a file: they are kept in the database and set on the Alerts & Notifications page (see [Email](alerts-and-notifications#email)).

Files use a simple `KEY=VALUE` format, one property per line.

Additional files in `/etc/sensor-hub/`:

| File                   | Purpose                                           |
|------------------------|---------------------------------------------------|
| `environment`          | Environment variables loaded by the systemd unit  |
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
| `local secrets check-seal` | Replace a sealed key that can no longer be unsealed |

They accept the following flags:

| Flag           | Applies to            | Default           | Description |
|----------------|-----------------------|-------------------|-------------|
| `--config-dir` | every `local` command | `/etc/sensor-hub` | Path to the configuration directory. It must hold `application.properties` and `database.properties` |
| `--log-file`   | `local serve`         | stdout            | Path to the log file. The packaged systemd unit sets `/var/log/sensor-hub/sensor-hub.log` |
| `--secrets-key-file` | `local serve`, `local secrets show-key` | none | Path to the secret-store key, read when there is no systemd credential or Compose secret. See [Secret-store key](#secret-store-key) |
| `--from-stdin` | `local secrets init-key` | off | Read the key from stdin rather than generating one |
| `--seal`       | `local secrets init-key` | off | Seal the key with the TPM through `systemd-creds` (root only) |
| `--tpm-grace`  | `local secrets check-seal` | `5m` | How long to wait for a TPM that cannot be used before replacing the sealed key |

`sensor-hub --version` prints the version and exits.

These flags are useful for running sensor-hub outside the standard package layout (e.g., during development).

## Secret-store key

The credentials the hub presents to other systems, such as outbound MQTT broker passwords, are stored in the database encrypted with AES-256-GCM. The key that decrypts them lives outside the database, so a copy of the database or a backup gives nothing usable on its own. No API response and no command other than `local secrets show-key` reveals a stored secret or the key.

The key is 32 random bytes, held as one line of standard base64. When the hub starts it uses the first key it finds, in this order:

1. `$CREDENTIALS_DIRECTORY/secrets.key`, a systemd credential. A key sealed with `local secrets init-key --seal` reaches the hub this way.
2. `/run/secrets/sensor-hub-secrets-key`, a Compose secret named `sensor-hub-secrets-key`.
3. The path given with `--secrets-key-file` to `local serve`. The file must exist; the hub does not fall back to another location when it is missing.
4. `<config-dir>/secrets.key`, which is `/etc/sensor-hub/secrets.key` on a package install.

A package install has a key from the start: the package creates it, sealed to the TPM where the host has one (see [The secret-store key](installation#the-secret-store-key)). When there is no key at any of them, the hub generates one, writes it to `<config-dir>/secrets.key` with mode 0600, logs that it did and where, and carries on. It refuses to do so when a sealed key, `<config-dir>/secrets.key.cred`, exists without systemd passing it in, since secrets stored under a second key would be unreadable under the unit.

A key file given with `--secrets-key-file` or found in the configuration directory must meet two rules, or the hub logs which one it broke and exits with an error:

- It is not readable by others: its mode has no `o+r` bit, so `chmod 600` it.
- It is not inside the directory that holds `database.path`, at any depth, so that copying the database directory never carries the key along.

A systemd credential and a Compose secret are exempt, because their manager puts them in place.

Keep a copy of the key, from `sudo sensor-hub local secrets show-key`, somewhere other than the hub, such as a password manager. If the key is lost or replaced, the stored secrets cannot be decrypted and have to be entered again.

### Secrets that need re-entry

When a stored secret does not decrypt under the key the hub starts with, because the key was lost or replaced or the database came from another host, the hub still starts. Readings, alerts, logins and the UI work as usual. Only what the secret belongs to stops:

- A broker whose password does not decrypt stays disconnected. Its `password_status` is `needs_reentry`, and the MQTT page shows a **Needs re-entry** chip on it with the text "This secret could not be decrypted. Enter it again."
- The hub logs one warning per secret, and raises one in-app notification, "Stored secrets need re-entry", naming everything affected. It goes to users with `view_notifications_config`, under the **Stored Secrets** notification category, which is in-app only by default.
- The gauge `sensor_hub_secret_decrypt_failures` reports how many stored secrets need re-entry (see [Telemetry](telemetry#prometheus-metrics)).

Enter the secret again, for a broker by editing it and typing its password, and it is stored under the current key and used straight away: the broker reconnects with no restart. Nothing else needs doing. If you still have the old key, you can instead put it back (see [The secret-store key](installation#the-secret-store-key)), and everything decrypts again at the next start.

## TLS to outbound MQTT brokers

An external broker, one the hub connects out to, is reached over plain TCP unless its **TLS** switch is on. Over plain TCP the broker's username and password, and every reading, cross the network unencrypted, so anything on the path can read them. The MQTT page marks such a broker **Unencrypted**.

Turn TLS on whenever the path to the broker crosses a network you do not control, such as the internet, a cloud provider's network or a shared LAN. Plain TCP is reasonable only where the hub and the broker share a host, or a network or tunnel that is already encrypted and that only you can reach. A broker serving TLS usually listens on port `8883` rather than `1883`, so check the port when you turn it on.

With TLS on, the hub verifies the broker before it sends anything: the broker's certificate must chain to a trusted CA and be issued for the host name or IP address set as the broker's **Host**. If either check fails, the connection fails and the password is never sent. There is no setting that skips verification. The hub presents no client certificate.

Which CAs are trusted depends on the **CA certificate** field:

- **Left empty**, the operating system's trusted CAs are used. This suits a broker with a certificate from a public CA, such as Let's Encrypt, or a hosted MQTT service.
- **Filled in**, that CA is the only one trusted, and the system's CAs are not. Use this for a broker whose certificate you issued yourself, such as a self-signed CA on a home Mosquitto.

To fill it in, open the broker from the MQTT page (**Add Broker**, or **Edit** from the broker's menu), turn **TLS** on, and paste the CA certificate into **CA certificate** in PEM form: the whole text from `-----BEGIN CERTIFICATE-----` to `-----END CERTIFICATE-----`, which is how a `.pem` or `.crt` file holds it. Paste the CA that signed the broker's certificate, not the broker's own certificate, and never a private key. Several certificates may be pasted one after the other. The hub stores the certificate itself rather than a file path, so it works the same in a container. A paste that holds no certificate, or anything other than certificates, is refused with a message saying so.

From the CLI, `mqtt brokers create --tls --ca-cert-file <path>` reads the CA from a PEM file and sends its content (see [MQTT brokers](cli-tool#mqtt-brokers)). In the JSON given to `mqtt brokers update --file`, the fields are `tls` and `ca_cert_pem`.

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

The default is `127.0.0.1,::1`, which trusts nginx on the same machine and nothing else. Set it empty, as `http.trusted.proxies=`, to trust no proxy: the client's address is then the address the connection came from, and both headers are ignored. Behind nginx with an empty list, every request appears to come from nginx, so one person failing to log in puts everyone under the login backoff. A value that is not a list of addresses and ranges is rejected. A change applies when the service restarts. See [Nginx Setup](nginx-setup#client-addresses) for nginx on another host.

### Metrics listen address

`metrics.listen.address` is the host and port the Prometheus `/metrics` endpoint listens on, on a listener of its own apart from the API. The default is `127.0.0.1:9464`. Empty turns the endpoint off, and any other value that is not `host:port` is rejected. A change applies when the service restarts. See [Telemetry](telemetry#prometheus-metrics) for the scrape configuration.

### Broker listen address

`mqtt.broker.listen.address` is the host or IP address the embedded MQTT broker listens on, without the port. The broker binds this address and `mqtt.broker.port` together, so the default of `127.0.0.1` and port `1883` gives `127.0.0.1:1883`. This takes connections from the same machine only, such as from a tunnel or from nginx's MQTT listener. To let devices on the network connect directly, set `0.0.0.0` (every IPv4 interface), `::` (every interface), or one address of the machine, such as a tunnel or LAN address. An empty value, or one that carries a port, is rejected. A change applies when the service restarts. See [Connecting your home](connecting-your-home) for when to change it.

### Broker CONNECT rate limit

`mqtt.broker.connect.rate.limit` is the most CONNECTs the embedded broker accepts in one second, counted across all clients together. A CONNECT over the limit is refused before its username and password are checked: an MQTT 5 client gets reason code `0x9F` (connection rate exceeded), an MQTT 3.1.1 client gets `0x03` (server unavailable), and the connection is closed. Each refusal adds one to `sensor_hub_mqtt_connect_refused_total{reason="rate_limit"}` (see [Telemetry](telemetry#prometheus-metrics)).

The broker keeps no count per username or per address, and a failed login brings no lockout or delay, so a flood of CONNECTs using your home device's username cannot lock the device out. The count starts again every second and nothing about a refusal is kept, so a device that retries is judged afresh in the next second. The default is `20`, and `0` turns the limit off. A change applies when the service restarts.

### Readings aggregation

Readings aggregation is controlled by the `readings.aggregation.*` properties. Tier values use ISO 8601 durations in `THRESHOLD:INTERVAL` format. The special interval `raw` means no aggregation. Tiers are evaluated in ascending order - the first tier whose threshold is >= the query span is used. Queries exceeding all thresholds fall back to `P1D` buckets. See the [auto-aggregation developer docs](development/auto-aggregation.md) for details.

## Environment variables

Environment variables are defined in `/etc/sensor-hub/environment` and loaded by the systemd unit via `EnvironmentFile=`.

| Variable                    | Description                                                                            |
|-----------------------------|----------------------------------------------------------------------------------------|
| `SENSOR_HUB_ALLOWED_ORIGIN` | An origin other than the hub's own that may call the API and open WebSockets, such as a UI dev server (e.g., `http://localhost:5173`). A browser WebSocket from any other origin is refused with 403 |
