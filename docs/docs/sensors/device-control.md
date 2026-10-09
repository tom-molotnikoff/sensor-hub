---
id: device-control
title: Device Control
sidebar_position: 4
---

# Device Control

Some sensors do more than report readings: they can also accept commands. In Sensor Hub these are **controllable sensors**. A common example is a Zigbee smart plug that reports power usage and can be turned on or off from the dashboard, or a dimmable light whose brightness can be set.

This page explains how Sensor Hub decides whether a sensor is controllable, how to use the **Sensor Toggle** and **Sensor Slider** widgets, which permission is required, and what to check when controls do not appear or commands time out.

## What makes a sensor controllable

A sensor is controllable when Sensor Hub has at least one writable **capability** for it.

For Zigbee devices, that information comes from Zigbee2MQTT metadata rather than from live readings alone. Zigbee2MQTT publishes `bridge/devices` data describing each device's available features. Sensor Hub uses that metadata to determine:

- which properties are writable
- what kind of control each property uses
- which values represent **on** and **off** for binary controls
- the range of a numeric control, such as a light's brightness

This matters because a reading like `state: ON` tells Sensor Hub the current value, but it does not reliably tell the hub whether that property can be commanded back to `OFF`.

### Only commands the hub can confirm

A command counts as done only when the device reports the new value back. So Sensor Hub offers a property as a capability only when all of these are true:

- the device exposes it as both settable and reported in its state
- Sensor Hub turns the reported value into a reading of the same name

Today that means `state` (on and off) and `brightness`. Anything else a device exposes, such as `color_temp`, `effect`, `identify`, `power_on_behavior` or the `level_config` settings, is left off the list rather than offered and left to time out. It isn't offered in the dashboard, the automation editor, the CLI or the API.

Not every Zigbee device is controllable. Many devices are read-only sensors, while smart plugs, relays, and similar devices usually expose one or more writable capabilities.

## Setting up device control

### Step 1 - Connect and approve the device

Follow [How to connect a Zigbee device to Sensor Hub](../how-to/connect-zigbee-device) if the device is not already connected.

Once the device publishes its first message:

1. Open **Sensors**
2. Approve the device if it is still **Pending**
3. Confirm it is an **Active** sensor

### Step 2 - Confirm Sensor Hub detected a capability

The easiest sign is that the sensor becomes selectable in the **Sensor Toggle** widget configuration, or for a numeric property such as brightness, the **Sensor Slider** widget configuration.

If you need to inspect the raw capability data or recent command history for troubleshooting or automation, use the built-in Swagger UI rather than a separate reference page.

### Step 3 - Add a Sensor Toggle widget

1. Open **Dashboards**
2. Create or edit a dashboard
3. Add a **Sensor Toggle** widget
4. Select the controllable sensor
5. Select the binary property you want to control (for most smart plugs this is `state`)

The widget then shows a large on/off control for that property. For broader dashboard configuration details, see [Dashboards](../dashboards).

### Step 4 - Add a Sensor Slider widget for a numeric property

To set a numeric property, such as a light's brightness:

1. Add a **Sensor Slider** widget
2. Select the sensor
3. Select the numeric property (`brightness` by default)

The slider's range comes from the device: the minimum and maximum Zigbee2MQTT reports for the property, and its step where it gives one. Let go of the slider to send the value. The widget shows the value it's setting until the device reports it back, then says whether the command was acknowledged, failed or timed out. If it fails or times out, the slider goes back to the last value the device reported.

## Permissions

Sending commands requires the `control_sensors` permission.

By default:

- **admin** users have it
- **user** users have it
- **viewer** users do not

If a user can see the widget but cannot operate it, check their assigned roles in [User Management and RBAC](../user-management).

## How commands reach the device

For Zigbee2MQTT devices, Sensor Hub publishes commands to the device's Zigbee2MQTT set topic using the device's friendly name:

```text
zigbee2mqtt/<friendly-name>/set
```

For example, a device named `office-plug` receives commands on:

```text
zigbee2mqtt/office-plug/set
```

Sensor Hub also records a **command history** for troubleshooting and auditing. That history lets you see whether a command was sent, acknowledged, timed out, or failed.

## Troubleshooting

### The sensor does not show any control options

- Confirm the device is one that Zigbee2MQTT exposes as writable
- Check that the sensor is approved and active in Sensor Hub
- Check that Sensor Hub is ingesting Zigbee2MQTT `bridge/devices` metadata, because controllable capabilities are derived from that metadata
- If the sensor reports readings but still looks read-only, the device may not expose any writable properties, or none that Sensor Hub can confirm (see [Only commands the hub can confirm](#only-commands-the-hub-can-confirm))

### The Sensor Toggle or Sensor Slider widget is read-only

- Check that your user has the `control_sensors` permission
- Verify the user has an **admin** or **user** role, or another custom role that grants the same permission

### The command times out

- Check that Sensor Hub is connected to the MQTT broker
- Check that Zigbee2MQTT is still connected to the same broker
- Check that the device is powered, reachable, and online in Zigbee2MQTT
- If other readings are stale too, fix broker or device connectivity first and then retry the command
