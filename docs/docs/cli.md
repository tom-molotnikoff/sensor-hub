---
id: cli-tool
title: CLI Tool
sidebar_position: 7
---

# CLI Tool

Sensor Hub ships as a single binary that can run as a **server** (`sensor-hub serve`) or as a **command-line client** for interacting with a remote Sensor Hub instance.

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
