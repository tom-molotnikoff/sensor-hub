"""Mock Zigbee2MQTT devices that publish to the Sensor Hub embedded MQTT broker.

Simulates three devices:
  - living-room-sensor: temperature, humidity, battery, linkquality
  - front-door:         contact (binary), battery
  - office-plug:        power, energy, current, state (binary, commandable)

Each device publishes every PUBLISH_INTERVAL seconds (default 5) to topics
matching the zigbee2mqtt/<device-name> convention.
"""

import json
import os
import random
import signal
import sys
import time

import paho.mqtt.client as mqtt

BROKER_HOST = os.environ.get("MQTT_BROKER_HOST", "sensor-hub")
BROKER_PORT = int(os.environ.get("MQTT_BROKER_PORT", "1883"))
PUBLISH_INTERVAL = int(os.environ.get("PUBLISH_INTERVAL", "5"))

# Mutable state for drifting values
state = {
    "living-room-sensor": {"temperature": 21.0, "humidity": 45.0, "battery": 95},
    "front-door": {"contact": True, "battery": 88},
    "office-plug": {"energy": 1.2, "state": "ON"},
}

BRIDGE_DEVICES = [
    {
        "ieee_address": "0x00158d0001000001",
        "friendly_name": "living-room-sensor",
        "definition": {
            "model": "WSDCGQ11LM",
            "vendor": "Aqara",
            "description": "Temperature and humidity sensor",
            "exposes": [
                {"type": "numeric", "property": "temperature", "name": "temperature", "access": 1, "unit": "°C"},
                {"type": "numeric", "property": "humidity", "name": "humidity", "access": 1, "unit": "%"},
                {"type": "numeric", "property": "battery", "name": "battery", "access": 1, "unit": "%"},
            ],
        },
    },
    {
        "ieee_address": "0x00158d0001000002",
        "friendly_name": "front-door",
        "definition": {
            "model": "MCCGQ11LM",
            "vendor": "Aqara",
            "description": "Door and window sensor",
            "exposes": [
                {"type": "binary", "property": "contact", "name": "contact", "access": 1},
                {"type": "numeric", "property": "battery", "name": "battery", "access": 1, "unit": "%"},
            ],
        },
    },
    {
        "ieee_address": "0x00158d0001000003",
        "friendly_name": "office-plug",
        "definition": {
            "model": "TS011F",
            "vendor": "Tuya",
            "description": "Smart plug",
            "exposes": [
                {
                    "type": "switch",
                    "features": [
                        {"type": "binary", "property": "state", "name": "state", "access": 7, "value_on": "ON", "value_off": "OFF"}
                    ],
                },
                {
                    "type": "binary",
                    "property": "network_indicator",
                    "name": "network_indicator",
                    "access": 7,
                    "value_on": True,
                    "value_off": False,
                },
                {"type": "numeric", "property": "power", "name": "power", "access": 1, "unit": "W", "value_min": 0, "value_max": 2500},
                {"type": "numeric", "property": "energy", "name": "energy", "access": 1, "unit": "kWh", "value_min": 0},
                {"type": "numeric", "property": "current", "name": "current", "access": 1, "unit": "A", "value_min": 0},
            ],
        },
    },
]


def drift(value, low, high, step=0.3):
    """Random-walk a value within bounds."""
    value += random.uniform(-step, step)
    return round(max(low, min(high, value)), 2)


def build_living_room():
    s = state["living-room-sensor"]
    s["temperature"] = drift(s["temperature"], 16.0, 28.0, 0.2)
    s["humidity"] = drift(s["humidity"], 30.0, 70.0, 0.5)
    s["battery"] = max(0, min(100, s["battery"] + random.choice([-1, 0, 0, 0, 0])))
    return {
        "temperature": s["temperature"],
        "humidity": s["humidity"],
        "battery": s["battery"],
        "linkquality": random.randint(40, 255),
    }


def build_front_door():
    s = state["front-door"]
    # Toggle contact occasionally (~10% chance)
    if random.random() < 0.10:
        s["contact"] = not s["contact"]
    s["battery"] = max(0, min(100, s["battery"] + random.choice([-1, 0, 0, 0, 0])))
    return {
        "contact": s["contact"],
        "battery": s["battery"],
    }


def build_office_plug():
    s = state["office-plug"]
    if s["state"] == "ON":
        power = round(random.uniform(40.0, 120.0), 1)
        current = round(power / 230.0, 3)
        s["energy"] = round(s["energy"] + power * PUBLISH_INTERVAL / 3_600_000, 4)
    else:
        power = 0.0
        current = 0.0
    return {
        "power": power,
        "energy": s["energy"],
        "current": current,
        "state": s["state"],
    }


DEVICES = {
    "living-room-sensor": build_living_room,
    "front-door": build_front_door,
    "office-plug": build_office_plug,
}


def on_connect(client, userdata, flags, rc, properties=None):
    if rc == 0:
        print(f"Connected to MQTT broker at {BROKER_HOST}:{BROKER_PORT}", flush=True)
        client.subscribe("zigbee2mqtt/+/set", qos=1)
        publish_bridge_devices(client)
    else:
        print(f"Connection failed with code {rc}", flush=True)


def normalise_switch_value(value):
    if isinstance(value, bool):
        return "ON" if value else "OFF"
    if isinstance(value, str):
        upper = value.strip().upper()
        if upper in {"ON", "OFF"}:
            return upper
    return None


def publish_bridge_devices(client):
    payload = json.dumps(BRIDGE_DEVICES)
    client.publish("zigbee2mqtt/bridge/devices", payload, qos=1, retain=True)
    print("  → zigbee2mqtt/bridge/devices: retained device metadata", flush=True)


def publish_device_state(client, name):
    topic = f"zigbee2mqtt/{name}"
    payload = json.dumps(DEVICES[name]())
    client.publish(topic, payload, qos=0)
    print(f"  → {topic}: {payload}", flush=True)


def on_message(client, userdata, msg):
    try:
        payload = json.loads(msg.payload.decode("utf-8"))
    except json.JSONDecodeError:
        print(f"Ignoring invalid command payload on {msg.topic}: {msg.payload!r}", flush=True)
        return

    if msg.topic != "zigbee2mqtt/office-plug/set":
        print(f"Ignoring command for unsupported topic {msg.topic}", flush=True)
        return

    requested_state = normalise_switch_value(payload.get("state"))
    if requested_state is None:
        print(f"Ignoring unsupported office-plug state payload: {payload!r}", flush=True)
        return

    state["office-plug"]["state"] = requested_state
    print(f"  ← {msg.topic}: setting office-plug state to {requested_state}", flush=True)
    publish_device_state(client, "office-plug")


def main():
    client = mqtt.Client(mqtt.CallbackAPIVersion.VERSION2, client_id="mock-mqtt-sensor")
    client.on_connect = on_connect
    client.on_message = on_message

    print(f"Connecting to {BROKER_HOST}:{BROKER_PORT}...", flush=True)

    # Retry connection until broker is available
    while True:
        try:
            client.connect(BROKER_HOST, BROKER_PORT, keepalive=60)
            break
        except (ConnectionRefusedError, OSError) as e:
            print(f"Broker not ready ({e}), retrying in 3s...", flush=True)
            time.sleep(3)

    client.loop_start()

    def shutdown(sig, frame):
        print("Shutting down...", flush=True)
        client.loop_stop()
        client.disconnect()
        sys.exit(0)

    signal.signal(signal.SIGTERM, shutdown)
    signal.signal(signal.SIGINT, shutdown)

    device_names = list(DEVICES.keys())
    stagger = PUBLISH_INTERVAL / len(device_names)

    print(f"Publishing {len(device_names)} devices every {PUBLISH_INTERVAL}s", flush=True)

    while True:
        for i, name in enumerate(DEVICES):
            publish_device_state(client, name)
            if i < len(device_names) - 1:
                time.sleep(stagger)
        remaining = PUBLISH_INTERVAL - stagger * (len(device_names) - 1)
        time.sleep(max(0.1, remaining))


if __name__ == "__main__":
    main()
