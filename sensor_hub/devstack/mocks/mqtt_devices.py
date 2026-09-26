import json
import os
import random
import signal
import sys
import time
from dataclasses import dataclass, field
from typing import Callable

import paho.mqtt.client as mqtt

BROKER_HOST = os.environ.get("MQTT_BROKER_HOST", "sensor-hub")
BROKER_PORT = int(os.environ.get("MQTT_BROKER_PORT", "1883"))
PUBLISH_INTERVAL = int(os.environ.get("PUBLISH_INTERVAL", "5"))


def numeric(prop, unit, **extra):
    return {"type": "numeric", "property": prop, "name": prop, "access": 1, "unit": unit, **extra}


def binary(prop, access=1, **extra):
    return {"type": "binary", "property": prop, "name": prop, "access": access, **extra}


def definition(model, vendor, description, exposes):
    return {"model": model, "vendor": vendor, "description": description, "exposes": exposes}


CLIMATE = definition(
    "WSDCGQ11LM",
    "Aqara",
    "Temperature and humidity sensor",
    [numeric("temperature", "°C"), numeric("humidity", "%"), numeric("battery", "%")],
)

CONTACT = definition(
    "MCCGQ11LM",
    "Aqara",
    "Door and window sensor",
    [binary("contact"), numeric("battery", "%")],
)

MOTION = definition(
    "RTCGQ11LM",
    "Aqara",
    "Motion sensor",
    [binary("occupancy"), numeric("illuminance", "lx"), numeric("battery", "%")],
)

PLUG_METERING = [
    numeric("power", "W", value_min=0, value_max=2500),
    numeric("energy", "kWh", value_min=0),
    numeric("current", "A", value_min=0),
]

SWITCHED_PLUG = definition(
    "TS011F",
    "Tuya",
    "Smart plug",
    [
        {"type": "switch", "features": [binary("state", access=7, value_on="ON", value_off="OFF")]},
        binary("network_indicator", access=7, value_on=True, value_off=False),
        *PLUG_METERING,
    ],
)

METERED_PLUG = definition(
    "SP 120",
    "Innr",
    "Smart plug with a locked relay",
    [binary("state", access=5, value_on="ON", value_off="OFF"), *PLUG_METERING],
)


def drift(value, low, high, step):
    value += random.uniform(-step, step)
    return round(max(low, min(high, value)), 2)


def drain_battery(state):
    state["battery"] = max(0, state["battery"] + random.choice([-1, 0, 0, 0, 0]))
    return state["battery"]


def climate_reading(state):
    state["temperature"] = drift(state["temperature"], 16.0, 28.0, 0.2)
    state["humidity"] = drift(state["humidity"], 30.0, 70.0, 0.5)
    return {
        "temperature": state["temperature"],
        "humidity": state["humidity"],
        "battery": drain_battery(state),
        "linkquality": random.randint(40, 255),
    }


def contact_reading(state):
    if random.random() < 0.10:
        state["contact"] = not state["contact"]
    return {"contact": state["contact"], "battery": drain_battery(state)}


def motion_reading(state):
    if random.random() < 0.15:
        state["occupancy"] = not state["occupancy"]
    state["illuminance"] = round(drift(state["illuminance"], 0.0, 800.0, 25.0))
    return {
        "occupancy": state["occupancy"],
        "illuminance": state["illuminance"],
        "battery": drain_battery(state),
    }


def plug_reading(state):
    power = 0.0
    if state["state"] == "ON":
        power = round(random.uniform(state["min_power"], state["max_power"]), 1)
        state["energy"] = round(state["energy"] + power * PUBLISH_INTERVAL / 3_600_000, 4)
    return {
        "power": power,
        "energy": state["energy"],
        "current": round(power / 230.0, 3),
        "state": state["state"],
    }


@dataclass
class Device:
    name: str
    ieee_address: str
    definition: dict
    build: Callable[[dict], dict]
    state: dict = field(default_factory=dict)
    switchable: bool = False

    def reading(self):
        return self.build(self.state)


DEVICES = [
    Device("living-room-sensor", "0x00158d0001000001", CLIMATE, climate_reading,
           {"temperature": 21.0, "humidity": 45.0, "battery": 95}),
    Device("bedroom-sensor", "0x00158d0001000004", CLIMATE, climate_reading,
           {"temperature": 18.5, "humidity": 52.0, "battery": 81}),
    Device("kitchen-sensor", "0x00158d0001000005", CLIMATE, climate_reading,
           {"temperature": 23.0, "humidity": 58.0, "battery": 67}),
    Device("front-door", "0x00158d0001000002", CONTACT, contact_reading,
           {"contact": True, "battery": 88}),
    Device("back-door", "0x00158d0001000006", CONTACT, contact_reading,
           {"contact": True, "battery": 74}),
    Device("office-plug", "0x00158d0001000003", SWITCHED_PLUG, plug_reading,
           {"state": "ON", "energy": 1.2, "min_power": 40.0, "max_power": 120.0}, switchable=True),
    Device("fridge-plug", "0x00158d0001000007", METERED_PLUG, plug_reading,
           {"state": "ON", "energy": 31.5, "min_power": 5.0, "max_power": 150.0}),
    Device("hallway-motion", "0x00158d0001000008", MOTION, motion_reading,
           {"occupancy": False, "illuminance": 120, "battery": 92}),
]

DEVICES_BY_NAME = {device.name: device for device in DEVICES}


def bridge_devices():
    return [
        {"ieee_address": device.ieee_address, "friendly_name": device.name, "definition": device.definition}
        for device in DEVICES
    ]


def normalise_switch_value(value):
    if isinstance(value, bool):
        return "ON" if value else "OFF"
    if isinstance(value, str):
        upper = value.strip().upper()
        if upper in {"ON", "OFF"}:
            return upper
    return None


def publish_bridge_devices(client):
    client.publish("zigbee2mqtt/bridge/devices", json.dumps(bridge_devices()), qos=1, retain=True)
    print("  → zigbee2mqtt/bridge/devices: retained device metadata", flush=True)


def publish_device_state(client, device):
    topic = f"zigbee2mqtt/{device.name}"
    payload = json.dumps(device.reading())
    client.publish(topic, payload, qos=0)
    print(f"  → {topic}: {payload}", flush=True)


def on_connect(client, userdata, flags, rc, properties=None):
    if rc == 0:
        print(f"Connected to MQTT broker at {BROKER_HOST}:{BROKER_PORT}", flush=True)
        client.subscribe("zigbee2mqtt/+/set", qos=1)
        publish_bridge_devices(client)
    else:
        print(f"Connection failed with code {rc}", flush=True)


def on_message(client, userdata, msg):
    try:
        payload = json.loads(msg.payload.decode("utf-8"))
    except json.JSONDecodeError:
        print(f"Ignoring invalid command payload on {msg.topic}: {msg.payload!r}", flush=True)
        return

    name = msg.topic.removeprefix("zigbee2mqtt/").removesuffix("/set")
    device = DEVICES_BY_NAME.get(name)
    if device is None or not device.switchable:
        print(f"Ignoring command for unsupported topic {msg.topic}", flush=True)
        return

    requested_state = normalise_switch_value(payload.get("state"))
    if requested_state is None:
        print(f"Ignoring unsupported {name} state payload: {payload!r}", flush=True)
        return

    device.state["state"] = requested_state
    print(f"  ← {msg.topic}: setting {name} state to {requested_state}", flush=True)
    publish_device_state(client, device)


def main():
    client = mqtt.Client(mqtt.CallbackAPIVersion.VERSION2, client_id="mock-mqtt-sensor")
    client.on_connect = on_connect
    client.on_message = on_message

    print(f"Connecting to {BROKER_HOST}:{BROKER_PORT}...", flush=True)

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

    stagger = PUBLISH_INTERVAL / len(DEVICES)

    print(f"Publishing {len(DEVICES)} devices every {PUBLISH_INTERVAL}s", flush=True)

    while True:
        for i, device in enumerate(DEVICES):
            publish_device_state(client, device)
            if i < len(DEVICES) - 1:
                time.sleep(stagger)
        remaining = PUBLISH_INTERVAL - stagger * (len(DEVICES) - 1)
        time.sleep(max(0.1, remaining))


if __name__ == "__main__":
    main()
