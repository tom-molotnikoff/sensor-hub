---
id: cli-tool
title: CLI Tool
sidebar_position: 7
---

# CLI Tool

Sensor Hub ships as a single binary. Its [local commands](#local-commands) act on the install on the machine they run on, including running the **server** (`sensor-hub local serve`). Every other command is a **command-line client** for interacting with a remote Sensor Hub instance.

## Installation

### CLI-only package (recommended for remote machines)

A lightweight `sensor-hub-cli` package is available that contains just the binary and shell completions — no server, systemd service, or configuration files. Install it on any machine you want to manage Sensor Hub from:

**Fedora / RHEL:**

```bash
sudo dnf install ./sensor-hub-cli-*.rpm
```

**Debian / Ubuntu:**

```bash
sudo apt install ./sensor-hub-cli_*.deb
```

Download the latest package from the [GitHub Releases](https://github.com/tom-molotnikoff/sensor-hub/releases) page. Packages are GPG-signed — see the [installation guide](installation) for verification steps.

:::note
The `sensor-hub-cli` and `sensor-hub` packages conflict with each other since they both provide the same binary. If you have the full server package installed, you already have the CLI — no need to install `sensor-hub-cli`.
:::

### Standalone binary

Alternatively, download a standalone binary from [GitHub Releases](https://github.com/tom-molotnikoff/sensor-hub/releases). Binaries are available for Linux, macOS, and Windows on both amd64 and arm64:

```bash
tar xzf sensor-hub_*_linux_amd64.tar.gz
sudo mv sensor-hub /usr/local/bin/sensor-hub
```

## Configuration

### Interactive Setup

Run the setup wizard to configure the CLI:

```bash
sensor-hub config init
```

This will prompt you for:

1. **Server URL** — the address of your Sensor Hub instance (e.g. `https://home.sensor-hub`)
2. **TLS verification** — if you entered an HTTPS URL, it asks whether to skip certificate verification (for self-signed certs)
3. **API key** — your API key for authentication

The wizard tests connectivity and API key validity before saving to `~/.sensor-hub.yaml`.

### Manual Configuration

Create `~/.sensor-hub.yaml`:

```yaml
server: https://home.sensor-hub
api_key: shk_your_api_key_here
insecure: true  # optional — skip TLS verification for self-signed certs
```

### Flag Overrides

All commands accept `--server`, `--api-key`, and `--insecure` flags, which override the config file:

```bash
sensor-hub sensors list --server https://home.sensor-hub --api-key shk_... --insecure
```

## Local commands

Commands under `sensor-hub local` act on the Sensor Hub install on the machine they run on. They read its configuration directory and database directly rather than talking to a hub over HTTP, so they need no server URL or API key. `sensor-hub --help` lists them apart from the commands that talk to a hub.

| Command | What it does |
|---|---|
| `local serve` | Run the server: the HTTP API, the embedded UI and sensor collection |
| `local admin create <username>` | Create the first admin user directly in the database |
| `local db backup <path>` | Write a consistent copy of the live database to a new file |

Every local command takes `--config-dir`, the directory holding `application.properties` and `database.properties`. It defaults to `/etc/sensor-hub`, where the package installs them. When either file is missing the command exits with an error naming the directory and the file. `local serve` also takes `--log-file`, which defaults to stdout.

### Run the server

The packaged systemd unit runs:

```bash
/usr/bin/sensor-hub local serve --config-dir=/etc/sensor-hub --log-file=/var/log/sensor-hub/sensor-hub.log
```

In development, from `sensor_hub/`, the configuration lives in `configuration/`:

```bash
go run . local serve --config-dir=configuration
```

### Create the first admin

```bash
sudo sensor-hub local admin create admin --email admin@example.com
```

On a terminal it asks for the password twice. Otherwise it reads one line from stdin, so a script can pipe the password in without it landing in a file, the environment or the process list:

```bash
printf '%s\n' "$ADMIN_PASSWORD" | sudo sensor-hub local admin create admin
```

The password must be at least 8 characters. It is hashed with bcrypt at `auth.bcrypt.cost`, raised to 10 when that is set lower. The admin is asked to change the password at first login only when `--must-change-password` is given. The server does not need to be running. Once any user holds the admin role the command refuses with "an admin already exists" and changes nothing; add further users from the web UI or with `users create`.

### Back up the database

```bash
sudo sensor-hub local db backup /var/backups/sensor-hub.db
```

The copy is taken with SQLite's `VACUUM INTO` while the server keeps running, so it is consistent and includes writes not yet checkpointed out of the `-wal` file. The file is created with mode 0600, and an existing file is never overwritten. Keep the backup together with a copy of `/etc/sensor-hub`.

## Automations

`sensor-hub automations` manages [automations](automations) with JSON in and out, so a script or an AI assistant can do everything the Automations page does. Every subcommand has `--help`.

| Command | What it does |
|---|---|
| `automations list` | Every automation with its status and next fire time |
| `automations get <id>` | One automation |
| `automations create --file <path>` | Create an automation from JSON |
| `automations update <id> --file <path>` | Replace an automation's name, triggers and steps |
| `automations delete <id>` | Delete an automation with its triggers, steps and runs |
| `automations enable <id>` / `disable <id>` | Switch an automation on or off |
| `automations run <id>` | Run an automation now |
| `automations runs <id>` | Its runs, newest first, with their step outcomes |
| `automations cancel <id> <runId>` | Cancel a running or waiting run |
| `automations margin-suggestion --sensor-id <id> --measurement-type <type>` | Suggest a re-arm margin for a numeric reading trigger |

`create` and `update` take the automation in the same shape as the API body, described with examples on the [automations](automations) page. Pass `--file -` to read it from stdin:

```bash
sensor-hub automations create --file lamp-timer.json
sensor-hub automations get 3 | jq '.triggers[0].at = "20:00"' | sensor-hub automations update 3 --file -
```

As with every command, the result is printed as JSON on stdout. A refused request prints the HTTP status and the server's message on stderr and exits with status 1:

- `create` and `update` fail with 400 and a message naming the field, such as `triggers[0].days must hold at least one weekday`.
- `run` prints the run it started. In `single` mode, an automation that is already running records a skipped run instead, so check that the printed `status` is not `skipped`. It fails with 409 on a broken automation.
- `cancel` fails with 409 when the run has already ended.
- `delete` fails with 409 while the automation has a running or waiting run.

The automation properties, such as `hub.timezone`, are read and set like any other:

```bash
sensor-hub properties get
sensor-hub properties set --key hub.timezone --value Europe/London
```
