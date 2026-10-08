---
id: connect-zigbee-device
title: How to connect a Zigbee device to Sensor Hub
sidebar_position: 1
---

# How to connect a Zigbee device to Sensor Hub

This guide walks you through connecting a Zigbee device — such as a smart plug, temperature sensor, or contact sensor — to Sensor Hub using Zigbee2MQTT.

## Before you start

You will need:

- A running Sensor Hub instance with MQTT enabled (`mqtt.broker.enabled=true`, the default)
- A Zigbee USB coordinator (e.g. SONOFF Zigbee 3.0 Dongle Plus ZBDongle-E, ConBee II, or similar)
- A Zigbee-compatible device (e.g. SONOFF S40 Lite smart plug, Aqara temperature sensor, SONOFF SNZB-02 sensor)
- A machine to run Zigbee2MQTT, with the coordinator plugged in
- A way for that machine to reach Sensor Hub's broker: a tunnel, the public MQTT port, or the same network as a hub at home. See [Connecting your home](../connecting-your-home)

## Step 1 — Install Zigbee2MQTT

Zigbee2MQTT is an open-source bridge that translates Zigbee radio traffic into MQTT messages. Install it on a machine with the USB coordinator plugged in.

**Find your coordinator device path:**

Plug in the USB coordinator and run:

```bash
ls -la /dev/serial/by-id/
```

You will see output like:

```
usb-ITEAD_SONOFF_Zigbee_3.0_USB_Dongle_Plus_V2_... -> ../../ttyACM0
```

or:

```
usb-Silicon_Labs_Sonoff_Zigbee_3.0_USB_Dongle_Plus_... -> ../../ttyUSB0
```

Note the full `/dev/serial/by-id/...` path — using this instead of `/dev/ttyUSB0` ensures the device is identified correctly even if USB port numbering changes after a reboot.

**Using Docker (recommended):**

```yaml
# docker-compose.yml
services:
  zigbee2mqtt:
    image: koenkk/zigbee2mqtt
    restart: unless-stopped
    volumes:
      - ./zigbee2mqtt-data:/app/data
      - /run/udev:/run/udev:ro
    ports:
      - 8081:8080
    devices:
      - /dev/serial/by-id/YOUR_COORDINATOR_ID:/dev/ttyUSB0
    environment:
      - TZ=Europe/London
    extra_hosts:
      - "host.docker.internal:host-gateway"
```

Replace `YOUR_COORDINATOR_ID` with the full path from the `ls` command above (e.g. `usb-Itead_Sonoff_Zigbee_3.0_USB_Dongle_Plus_V2_...`).

:::info[Why 8081:8080?]
Zigbee2MQTT's web frontend listens on port 8080 inside the container. Sensor Hub also uses port 8080 by default, so we remap the Zigbee2MQTT frontend to **8081** on the host to avoid a port conflict. After starting the container, access the Zigbee2MQTT dashboard at `http://YOUR_HOST:8081`.
:::

:::note[Container device path]
The container-side device path (`/dev/ttyUSB0` above) depends on your coordinator model:
- **SONOFF Zigbee 3.0 USB Dongle Plus V2** (EFR32-based): maps to `/dev/ttyUSB0`
- **SONOFF Zigbee 3.0 USB Dongle Plus V1** and **ConBee II** (CC2652-based): map to `/dev/ttyACM0`

Check the output of `ls -la /dev/serial/by-id/` — the symlink target (`../../ttyUSB0` or `../../ttyACM0`) tells you which to use.
:::

**Auto-starting Zigbee2MQTT with systemd:**

If you want the Zigbee2MQTT Docker Compose stack to start automatically on boot, create a systemd unit:

```bash
sudo tee /etc/systemd/system/zigbee2mqtt.service > /dev/null <<'EOF'
[Unit]
Description=Zigbee2MQTT Docker Compose
Requires=docker.service
After=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=/path/to/your/zigbee2mqtt
ExecStart=/usr/bin/docker compose up -d
ExecStop=/usr/bin/docker compose down

[Install]
WantedBy=multi-user.target
EOF
```

Replace `/path/to/your/zigbee2mqtt` with the directory containing your `docker-compose.yml`. Then enable and start it:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now zigbee2mqtt
```

**Or follow the [official Zigbee2MQTT installation guide](https://www.zigbee2mqtt.io/guide/installation/).**

## Step 2 — Configure Zigbee2MQTT to publish to Sensor Hub

Create the credential Zigbee2MQTT connects with. In the Sensor Hub web UI, open the **MQTT** page and click **Add Client** on the **MQTT Clients** card, with the name `zigbee2mqtt` and the topic prefix `zigbee2mqtt/`. Or with the CLI:

```bash
sensor-hub mqtt clients create --name zigbee2mqtt --topic-prefix zigbee2mqtt/
```

The password is shown once, so copy it now.

Edit the Zigbee2MQTT configuration file (`zigbee2mqtt-data/configuration.yaml`) to point at Sensor Hub's MQTT broker:

```yaml
mqtt:
  base_topic: zigbee2mqtt
  server: mqtt://SENSOR_HUB_BROKER:1883
  user: zigbee2mqtt
  password: GENERATED_PASSWORD

serial:
  port: /dev/ttyUSB0

frontend:
  enabled: true
  port: 8080

advanced:
  log_level: info
```

Replace `GENERATED_PASSWORD` with the password from the step above, and set `server` for the way your home reaches the hub: the hub's tunnel address, `mqtts://hub.example.com:8883` for the public port, or the hub's LAN address for a hub at home. [Zigbee (via Zigbee2MQTT)](../sensors/zigbee#connect-zigbee2mqtt-to-the-broker) shows the full block for each. The `serial.port` should match the right-hand side of the Docker `devices` mapping (i.e. `/dev/ttyUSB0` or `/dev/ttyACM0` depending on your coordinator). If you are running Zigbee2MQTT without Docker, use the full `/dev/serial/by-id/...` path instead.

:::tip[Same machine as Sensor Hub?]
The broker listens on `127.0.0.1` by default, so Zigbee2MQTT on the same machine connects to `mqtt://localhost:1883`. Running natively, that works as it is. In Docker, the container must share the host's network: replace the `ports` and `extra_hosts` entries with `network_mode: host`, and move the Zigbee2MQTT frontend off port 8080 with `frontend.port: 8081`, since Sensor Hub uses 8080.
:::

Restart Zigbee2MQTT after editing the configuration.

## Step 3 — Pair your Zigbee device

1. Open the Zigbee2MQTT web UI (default: `http://ZIGBEE2MQTT_HOST:8081`)
2. Click **Permit join** to enable pairing mode
3. Put your device into pairing mode:
   - **SONOFF S40 Lite**: Hold the button for 5 seconds until the LED flashes
   - **Aqara sensors**: Hold the reset button for 5 seconds
   - Check your device manual for specific instructions
4. The device should appear in the Zigbee2MQTT dashboard within 30 seconds
5. Give it a friendly name (e.g. `office-plug`, `living-room-sensor`) — this becomes the sensor name in Sensor Hub

## Step 4 — Add an MQTT subscription in Sensor Hub

In the Sensor Hub web UI:

1. Navigate to **Settings → MQTT Brokers**
2. The embedded broker should already be listed. If not, check that `mqtt.broker.enabled=true` in your configuration
3. Navigate to **Settings → MQTT Subscriptions**
4. Click **Add Subscription** and fill in:
   - **Broker**: Select the embedded broker
   - **Topic pattern**: `zigbee2mqtt/#`
   - **Driver type**: `mqtt-zigbee2mqtt`
5. Save the subscription

**Using the CLI:**

```bash
sensor-hub mqtt subscriptions add \
  --broker-id 1 \
  --topic "zigbee2mqtt/#" \
  --driver mqtt-zigbee2mqtt
```

## Step 5 — Verify the device appears

Once a Zigbee device publishes its first message, Sensor Hub auto-discovers it and creates a sensor entry with a **Pending** status.

1. Navigate to **Sensors** in the web UI
2. You should see the new device listed with its Zigbee2MQTT friendly name
3. Click **Approve** to activate the sensor
4. Readings will start appearing immediately

**Using the CLI:**

```bash
# List pending sensors
sensor-hub sensors list --status pending

# Approve a sensor
sensor-hub sensors approve --id <sensor-id>
```

## What gets collected

Sensor Hub automatically extracts all known measurement types from the Zigbee2MQTT JSON payload. The measurements available depend on your device:

| Device type | Typical measurements |
|------------|---------------------|
| Smart plug (e.g. SONOFF S40) | Power (W), Current (A), Voltage (mV), Energy (kWh) |
| Temperature/humidity sensor | Temperature (°C), Humidity (%), Battery (%) |
| Contact sensor | Contact (open/closed), Battery (%) |
| Motion sensor | Occupancy (true/false), Battery (%), Illuminance (lx) |
| Air quality sensor | CO₂ (ppm), VOC (ppb), Temperature (°C), Humidity (%) |

All devices also report **Link Quality** (lqi), which indicates Zigbee signal strength.

## Troubleshooting

**Device does not appear in Sensor Hub:**

- Check that Zigbee2MQTT is connected to the MQTT broker — the Zigbee2MQTT log should show `Connected to MQTT server`
- `Not authorized` in the Zigbee2MQTT log means the broker refused the credential: check `user` and `password`, and that the client is enabled on the **MQTT** page
- A connection that times out or is refused means Zigbee2MQTT cannot reach the broker: check the tunnel or nginx, and `mqtt.broker.listen.address` (see [Connecting your home](../connecting-your-home))
- Verify the subscription topic matches: the default Zigbee2MQTT base topic is `zigbee2mqtt`, so `zigbee2mqtt/#` should catch everything
- Check the MQTT Broker Stats page in Sensor Hub to confirm messages are being received

**Device appears but shows no readings:**

- Ensure the sensor status is **Active** (approve it if it is still Pending)
- Some devices only report when a value changes — press the button or trigger the sensor to force an update

**Wrong device name:**

- Rename the device in the Zigbee2MQTT web UI. The new name will be reflected in Sensor Hub on the next message
