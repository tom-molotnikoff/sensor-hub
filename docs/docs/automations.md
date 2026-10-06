---
id: automations
title: Automations
sidebar_position: 8
---

# Automations

An automation switches devices for you. It has one or more **triggers** and an ordered list of **steps**. When any trigger fires, the hub starts a **run** and carries out the steps from top to bottom. A step either sets a device or waits.

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

## Interval triggers

An interval trigger fires every so many seconds, for example to cycle a pump or a fan:

```json
{ "type": "interval", "seconds": 1800 }
```

- `seconds` is a whole number of seconds, at least `60`. In the editor, an interval is entered in minutes or hours.
- The interval counts from the moment the automation was last saved or switched on. Every 30 minutes, saved at 10:07, fires at 10:37, 11:07 and so on. Switched off and on again at 14:00, it next fires at 14:30.
- The hub stores when the trigger is next due, so a restart does not move it. Times the hub was down for are handled by the [grace window](#restarts-and-the-grace-window).
- An interval is a length of time, so a clock change or a change to `hub.timezone` does not move it.

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

## Wait steps

A wait step pauses the run before the next step:

```json
{ "type": "wait", "seconds": 14400 }
```

`seconds` is a whole number of seconds, at least `1`. There is no maximum. In the editor, a wait is entered in seconds, minutes or hours.

While a run waits, its status is `waiting` and `resume_at` says when it carries on. The automation shows as `running` for the whole wait.

## When a step fails

A set step fails when:

- the command is not acknowledged within `actuator.command.timeout_seconds`, or the command fails
- the sensor already has a command in flight for that property, for example because someone pressed a toggle a moment earlier. The step is not retried.
- the sensor is disabled or not active
- the value no longer suits the capability

A failed step ends the run as `failed`, and no later steps run. Everyone with the `manage_automations` permission gets an **Automation failures** notification naming the automation, the step and the reason, in the app and by email according to their [notification preferences](alerts-and-notifications#notification-preferences).

## Restarts and the grace window

Runs are stored in the database, so a hub restart does not lose them. Restarting the hub to update it during "on, wait 4 h, off" still switches the lamp off.

When the hub starts, before any trigger can fire:

- A waiting run carries on at its `resume_at`. If that time passed while the hub was down, the run carries on straight away, however late it is.
- A run that was in the middle of a set step finishes that step from command history. If the step's command was recorded, the step takes that command's outcome, waiting for the acknowledgement if the command is still in flight, and the command is not sent again. If no command was recorded, the step is sent.
- A schedule or interval trigger that came due while the hub was down is caught up if the hub started within `automation.missed.grace.minutes` of the due time (10 minutes by default, see [Configuration](configuration#missed-trigger-grace-window)). The run starts on startup.
- A trigger that came due longer ago than that does not run. It is recorded once as a `missed` run, with `due_at` and `past_grace_seconds` saying when it came due and how long after the end of the grace window the hub started. A trigger missed several times in one outage, such as a schedule on several days or an interval many times over, records one `missed` run, for its latest due time. At most one run starts for it on startup.

Catching up only once, and only shortly after the due time, keeps a hub that was down overnight from switching yesterday evening's lights on at breakfast.

If the hub stops after a command was published but before it was recorded, that command is sent again when the hub starts.

## Runs and command history

Every run is recorded with the trigger that fired it, its status, the step it is on, a copy of the steps it started with, its start and finish times and any error. `GET /api/automations/{id}/runs` lists them newest first, with the outcome of each step.

A run's status is `running` while it goes and `waiting` during a wait step, then `succeeded` or `failed`. A trigger that came due while the hub was down, past the grace window, is recorded as `missed` (see [Restarts and the grace window](#restarts-and-the-grace-window)).

Commands sent by a run go out from the hub itself rather than from a user. In a sensor's command history (`GET /api/sensors/by-id/{id}/commands`) they carry the automation's id and name instead of a user, which answers "why did this switch on?".

## Status

Each automation reports a status:

| Status    | Meaning                                         |
|-----------|-------------------------------------------------|
| `off`     | switched off; its triggers start nothing        |
| `armed`   | switched on and waiting for a trigger           |
| `running` | a run is in progress or waiting                 |

A separate `last_run_failed` flag is true from a failed run until the next run that succeeds.

`next_fire_at` is when the earliest schedule or interval trigger next comes due, in UTC, with `hub_timezone` alongside for showing it in local time. It is empty when the automation is off.

Switch an automation on or off with `PUT /api/automations/{id}/enabled`. Deleting an automation deletes its triggers, steps and run history. The commands its runs sent stay in command history, without the link to the run.

## Permissions

| Permission           | Granted to             | Allows                                    |
|----------------------|------------------------|-------------------------------------------|
| `view_automations`   | admin, user, viewer    | listing automations and their runs        |
| `manage_automations` | admin, user            | creating, editing, switching and deleting |

Creating, editing and switching an automation on or off also need `control_sensors`, so an automation cannot be used to control devices you could not control yourself. Once saved, an automation runs as the hub: it keeps working if its author later loses `control_sensors`.

## Example: lamp timer

Turn the hallway lamp on at 19:00 every day, dimmed, and off again four hours later:

```json
{
  "name": "Lamp timer",
  "triggers": [
    { "type": "schedule", "at": "19:00", "days": ["mon", "tue", "wed", "thu", "fri", "sat", "sun"] }
  ],
  "steps": [
    { "type": "set", "sensor_id": 14, "property": "state", "value": "ON" },
    { "type": "set", "sensor_id": 14, "property": "brightness", "value": "150" },
    { "type": "wait", "seconds": 14400 },
    { "type": "set", "sensor_id": 14, "property": "state", "value": "OFF" }
  ]
}
```

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

## Example: pump cycle

Run the pond pump for 2 minutes every 30 minutes:

```json
{
  "name": "Pond pump cycle",
  "triggers": [
    { "type": "interval", "seconds": 1800 }
  ],
  "steps": [
    { "type": "set", "sensor_id": 22, "property": "state", "value": "ON" },
    { "type": "wait", "seconds": 120 },
    { "type": "set", "sensor_id": 22, "property": "state", "value": "OFF" }
  ]
}
```

## Saving an automation

Send an automation with `POST /api/automations`. A new automation is switched on unless the body sets `"enabled": false`. A save the hub refuses comes back as `400` with a message naming the field, such as `triggers[0].days must hold at least one weekday`.
