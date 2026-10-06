---
id: automations
title: Automations
sidebar_position: 8
---

# Automations

An automation switches devices for you. It has one or more **triggers** and an ordered list of **steps**. When any trigger fires, the hub starts a **run** and carries out the steps from top to bottom.

Automations are managed through the REST API under `/api/automations`. The built-in Swagger UI documents every endpoint and lets you try them.

## Schedule triggers

A schedule trigger fires at a time of day on the weekdays you choose:

```json
{ "type": "schedule", "at": "19:00", "days": ["mon", "tue", "wed", "thu", "fri"] }
```

- `at` is a time of day as `HH:MM`, from `00:00` to `23:59`.
- `days` holds at least one of `mon`, `tue`, `wed`, `thu`, `fri`, `sat` and `sun`.
- Cron expressions are not accepted.

An automation can have several triggers. Any one of them starts a run, and two that come due at the same moment start a single run.

### The hub timezone

Times are read on the hub's clock, set by the `hub.timezone` property on the Properties page (see [Configuration](configuration#hub-timezone)). It holds an IANA zone name such as `Europe/London`, so 19:00 stays 19:00 local time through every clock change with nothing to update.

Clock changes are handled like this:

- When the clocks go forward and a time does not exist that night, the trigger fires at the first minute after the gap. A 01:30 trigger in `Europe/London` fires at 02:00 BST on the last Sunday of March.
- When the clocks go back and a time happens twice, the trigger fires once, at the first occurrence.

Changing `hub.timezone` moves every schedule to the new zone straight away, without a restart.

## Set steps

A set step sends a command to a writable capability of a controllable sensor, the same command you can send from a Sensor Toggle widget or `POST /api/sensors/{id}/command`:

```json
{ "type": "set", "sensor_id": 14, "property": "state", "value": "ON" }
```

Any writable capability can be set, not only on and off. The value is a string, checked against the capability both when the automation is saved and again when the step runs:

| Capability type | Accepted values                          |
|-----------------|------------------------------------------|
| Binary          | its `value_on` or `value_off`            |
| Numeric         | a number within its `min` and `max`      |
| Enum            | one of its `values`                      |

`GET /api/sensors/by-id/{id}/capabilities` lists a sensor's writable capabilities. See [Device Control](sensors/device-control) for how they are detected.

Each step waits for the device to acknowledge its command before the next step starts.

## When a step fails

A set step fails when:

- the command is not acknowledged within `actuator.command.timeout_seconds`, or the command fails
- the sensor already has a command in flight for that property, for example because someone pressed a toggle a moment earlier. The step is not retried.
- the sensor is disabled or not active
- the value no longer suits the capability

A failed step ends the run as `failed`, and no later steps run. Everyone with the `manage_automations` permission gets an **Automation failures** notification naming the automation, the step and the reason, in the app and by email according to their [notification preferences](alerts-and-notifications#notification-preferences).

## Runs and command history

Every run is recorded with the trigger that fired it, its status, the step it is on, a copy of the steps it started with, its start and finish times and any error. `GET /api/automations/{id}/runs` lists them newest first, with the outcome of each step.

A run's status is `running` while it goes, then `succeeded` or `failed`.

Commands sent by a run go out from the hub itself rather than from a user. In a sensor's command history (`GET /api/sensors/by-id/{id}/commands`) they carry the automation's id and name instead of a user, which answers "why did this switch on?".

## Status

Each automation reports a status:

| Status    | Meaning                                         |
|-----------|-------------------------------------------------|
| `off`     | switched off; its triggers start nothing        |
| `armed`   | switched on and waiting for a trigger           |
| `running` | a run is in progress                            |

A separate `last_run_failed` flag is true from a failed run until the next run that succeeds.

`next_fire_at` is when the earliest trigger next comes due, in UTC, with `hub_timezone` alongside for showing it in local time. It is empty when the automation is off.

Switch an automation on or off with `PUT /api/automations/{id}/enabled`. Deleting an automation deletes its triggers, steps and run history. The commands its runs sent stay in command history, without the link to the run.

## Permissions

| Permission           | Granted to             | Allows                                    |
|----------------------|------------------------|-------------------------------------------|
| `view_automations`   | admin, user, viewer    | listing automations and their runs        |
| `manage_automations` | admin, user            | creating, editing, switching and deleting |

Creating, editing and switching an automation on or off also need `control_sensors`, so an automation cannot be used to control devices you could not control yourself. Once saved, an automation runs as the hub: it keeps working if its author later loses `control_sensors`.

## Example: evening lights

Turn the hallway lamp on at 19:00 on weekdays and 18:00 at weekends, at a dimmed brightness:

```json
{
  "name": "Evening lights",
  "triggers": [
    { "type": "schedule", "at": "19:00", "days": ["mon", "tue", "wed", "thu", "fri"] },
    { "type": "schedule", "at": "18:00", "days": ["sat", "sun"] }
  ],
  "steps": [
    { "type": "set", "sensor_id": 14, "property": "state", "value": "ON" },
    { "type": "set", "sensor_id": 14, "property": "brightness", "value": "150" }
  ]
}
```

Send it with `POST /api/automations`. A new automation is switched on unless the body sets `"enabled": false`. A save the hub refuses comes back as `400` with a message naming the field, such as `triggers[0].days must hold at least one weekday`.
