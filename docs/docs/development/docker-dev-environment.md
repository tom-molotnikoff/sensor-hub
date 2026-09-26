# Docker Dev Environment

The Docker Compose setup provides a complete development environment with
hot-reload, remote debugging, mock temperature sensors, and full
OpenTelemetry observability via Grafana.

## Start the Environment

```bash
cd sensor_hub
docker compose -f docker_tests/docker-compose.yml up --build
```

## Seeded Logins and API Key

The new stack in `sensor_hub/devstack` runs a one-shot `seed` service before
the hub starts. From an empty volume it creates these users, none of which
has to change their password:

| Username | Password | Role |
|---|---|---|
| `admin` | `adminpassword` | admin |
| `user` | `userpassword` | user |
| `viewer` | `viewerpassword` | viewer |

It also creates an admin API key and prints it on every start:

```bash
cd sensor_hub/devstack
docker compose logs seed | grep admin_api_key
```

The seed only creates these once, so anything you change or delete stays that
way. `docker compose down -v` gives you a fresh set. The passwords and key are
for local development only.

## Grafana — Observability Stack

The `grafana/otel-lgtm` container bundles the full Grafana observability
stack in a single image.

The sensor-hub container is pre-configured to export all OpenTelemetry data
(logs, traces, and metrics) to the Grafana LGTM collector via gRPC on port
4317. No additional application configuration is needed.

### Accessing Grafana

Open [http://localhost:4000](http://localhost:4000) in your browser. The
default credentials are **admin / admin** (skip the password change prompt
for local development).

## Air — Go Hot-Reload

Go source changes trigger an automatic rebuild and restart of the backend. The UI directory is excluded from file watching.

## Delve — Remote Debugging

Connect your IDE debugger to `localhost:2345` (DAP / Delve API v2). The application starts immediately — attach a debugger at any time without restarting.

## Vite — React HMR

The UI container runs the Vite dev server with hot module replacement. Source changes in `ui/sensor_hub_ui/src/` are reflected immediately in the browser at **localhost:3000**.

## Mock Sensors

The dev stack includes mock sensors for both data collection models.

### HTTP Sensors (Pull Model)

Two Python Flask containers (`mock-sensor-downstairs`, `mock-sensor-upstairs`)
simulate HTTP temperature sensors. Each returns random temperature readings on
port 5000 inside the container, exposed as ports 5001 and 5002 on the host.
Register them in the Sensor Hub UI to test the full pull-based pipeline.

### MQTT Sensor (Push Model)

The `mock-mqtt-sensor` container simulates three Zigbee2MQTT devices that
publish to the embedded MQTT broker inside sensor-hub every 5 seconds:

Values drift randomly within realistic ranges. The container also publishes a
retained `zigbee2mqtt/bridge/devices` message so Sensor Hub can discover device
metadata and controllable capabilities, and the `office-plug` device accepts
`zigbee2mqtt/office-plug/set` commands for manual control testing.

#### Setting Up MQTT Ingest

The mock sensor starts publishing immediately, but Sensor Hub won't process
the messages until you create a broker record and subscription. After the
stack is running:

**1. Log in and create a broker record pointing at the embedded broker:**

**2. Create a subscription that routes zigbee2mqtt topics to the driver:**

**3. Check that devices were auto-discovered as pending sensors:**

You should see some sensors listed as pending. Approve them to start recording readings.

Once the `office-plug` sensor is approved, it can be used with the Sensor Toggle
dashboard widget or the command API for end-to-end local actuator testing.
