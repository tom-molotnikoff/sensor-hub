---
id: http-temperature
title: HTTP Temperature Sensor
sidebar_position: 2
---

# HTTP Temperature Sensor

The `sensor-hub-http-temperature` driver collects temperature readings from sensors that expose an HTTP endpoint. It is a **pull-based** driver — Sensor Hub polls the sensor at a configurable interval (default: every 5 minutes).

This driver was originally built for DS18B20 temperature sensors connected to Raspberry Pi devices using the 1-wire protocol, but it works with any sensor that returns a compatible JSON response.

## Driver details

| Property          | Value                                                                      |
|-------------------|----------------------------------------------------------------------------|
| Driver type       | `sensor-hub-http-temperature`                                              |
| Protocol          | HTTP GET                                                                   |
| Measurement types | Temperature (°C)                                                           |
| Collection model  | Pull (Sensor Hub polls the sensor)                                         |
| Config fields     | `url` (required) — base URL of the sensor, e.g. `http://192.168.1.50:5000` |

## How it works

The driver makes a `GET` request to `{url}/temperature` and expects a JSON response:

```json
{
  "time": "2026-01-15 14:30:00",
  "temperature": 21.56
}
```

If the sensor cannot be read, the API returns a 500 status code with an error message. Sensor Hub marks the sensor health as "bad" and retries on the next collection cycle.

Sensors are expected to be on a trusted local network. The sensor API does not implement authentication.

## Registering in Sensor Hub

After deploying a sensor on your network, register it in Sensor Hub so that it is polled for readings. You can do this through the web UI, the REST API, or the CLI. See the [How to connect an HTTP temperature sensor](../how-to/connect-http-sensor) guide for step-by-step instructions, or [Managing Sensors](managing-sensors-ref) for details on the registration process.

---

## Sensor Hub temperature sensor package

Sensor Hub includes a companion temperature sensor application, packaged as a `.deb`, that runs on a Raspberry Pi and reads a DS18B20 sensor over the 1-wire protocol. This is a lightweight Flask API that exposes the HTTP endpoint consumed by the driver above.

:::note
This package is designed for a specific hardware setup (Raspberry Pi + DS18B20). If you have a different HTTP sensor that returns the JSON format shown above, you can use the `sensor-hub-http-temperature` driver directly without this package.
:::

### Hardware setup

1. Connect a DS18B20 temperature sensor to your Raspberry Pi using the 1-wire protocol.
2. Enable the 1-wire interface on the Raspberry Pi. You can do this through `raspi-config` or by adding the following line to `/boot/config.txt`:

```
dtoverlay=w1-gpio
```

3. Reboot the Raspberry Pi for the change to take effect.

### Package variants

Two package variants are available:

| Package                   | Description                                | Best for                                              |
|---------------------------|--------------------------------------------|-------------------------------------------------------|
| `temperature-sensor-lite` | Core sensor API only                       | Pi Zero, Pi 1, or any Pi where you don't need tracing |
| `temperature-sensor`      | Includes OpenTelemetry distributed tracing | Pi 3, Pi 4, Pi 5 with an OTLP collector               |

The lite package installs significantly faster and uses far less disk space since it does not include the OpenTelemetry and gRPC C extensions. Both packages provide identical sensor functionality — the only difference is tracing support.

The two packages conflict with each other. To switch variant, remove the current package first with `sudo apt remove temperature-sensor` or `sudo apt remove temperature-sensor-lite`.

### Prerequisites

Before installing, ensure the following packages are present on the Raspberry Pi:

```bash
sudo apt update
sudo apt install -y python3 python3-venv python3-pip
```

### Installing

Download the package from the [GitHub Releases](https://github.com/tom-molotnikoff/sensor-hub/releases) page and install it:

```bash
# Lite (recommended for Pi Zero / Pi 1)
wget https://github.com/tom-molotnikoff/sensor-hub/releases/download/vVERSION/temperature-sensor-lite_VERSION_all.deb
sudo apt install ./temperature-sensor-lite_VERSION_all.deb

# Full (with OpenTelemetry tracing)
wget https://github.com/tom-molotnikoff/sensor-hub/releases/download/vVERSION/temperature-sensor_VERSION_all.deb
sudo apt install ./temperature-sensor_VERSION_all.deb
```

Replace `VERSION` with the version you are installing (e.g. `1.0.0`).

The package automatically:
- Installs the sensor API to `/usr/lib/temperature-sensor/`
- Creates a Python virtual environment and installs all dependencies
- Creates a dedicated `temperature-sensor` system user with `gpio` group access for hardware reading
- Installs and enables a systemd service (`temperature-sensor.service`)

:::note
The Pi requires an internet connection during installation so that `pip` can download the Python dependencies into the virtual environment.
:::

### Verifying the GPG signature

Each release includes a detached GPG signature (`.deb.sig`). To verify:

```bash
# Import the public key (one-time)
wget https://github.com/tom-molotnikoff/sensor-hub/releases/download/vVERSION/sensor-hub-gpg-public.key
gpg --import sensor-hub-gpg-public.key

# Verify the package
gpg --verify temperature-sensor_VERSION_all.deb.sig temperature-sensor_VERSION_all.deb
```

### Configuration

Edit `/etc/temperature-sensor/environment` to configure the sensor:

```bash
# Change the Flask port (default: 5000)
# FLASK_RUN_PORT=5000

# Enable OpenTelemetry distributed tracing (optional)
# OTEL_EXPORTER_OTLP_ENDPOINT=http://your-collector:4317
# OTEL_SERVICE_NAME=temperature-sensor-living-room
```

The `OTEL_SERVICE_NAME` is a good place to give each sensor a human-readable name (e.g. `temperature-sensor-kitchen`). When OpenTelemetry is configured, the sensor participates in distributed traces — you can see the full request lifecycle from Sensor Hub through to the sensor in tools like Grafana Tempo.

### Starting the service

```bash
sudo systemctl start temperature-sensor
sudo systemctl status temperature-sensor
```

The sensor API starts on port 5000 by default and listens on all network interfaces.

### Verifying the sensor

Test that the sensor is responding:

```bash
curl http://<sensor-ip>:5000/temperature
```

### Updating

Download and install the new `.deb` package. The upgrade is handled automatically — dependencies are reinstalled and the service is restarted:

```bash
wget https://github.com/tom-molotnikoff/sensor-hub/releases/download/vNEW_VERSION/temperature-sensor_NEW_VERSION_all.deb
sudo apt install ./temperature-sensor_NEW_VERSION_all.deb
```

Your configuration in `/etc/temperature-sensor/environment` is preserved across upgrades.

### Uninstalling

```bash
# Remove the package (keeps configuration files)
sudo apt remove temperature-sensor

# Purge the package (removes everything including config and virtualenv)
sudo apt purge temperature-sensor
```
