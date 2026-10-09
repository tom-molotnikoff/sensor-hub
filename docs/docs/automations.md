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

## Sensor reading triggers

A sensor reading trigger fires when a sensor's readings of one measurement type cross a threshold, or when a binary reading becomes a value. Readings reach automations through the same reading pipeline as alerts and the live view, so a reading pushed over MQTT starts a run within a second.

On a numeric measurement type, such as temperature, the operator is `falls_below` or `rises_above`:

```json
{ "type": "reading", "sensor_id": 3, "measurement_type": "temperature",
  "operator": "falls_below", "threshold": 16, "rearm_margin": 0.2 }
```

On a binary measurement type, such as a door contact, the operator is `becomes`:

```json
{ "type": "reading", "sensor_id": 10, "measurement_type": "contact",
  "operator": "becomes", "value": "false", "hold_seconds": 5 }
```

- `measurement_type` must be one the sensor reports. `GET /api/sensors/by-id/{id}/measurement-types` lists them, with whether each is numeric or binary.
- `falls_below` means the value is below the threshold, and `rises_above` means it is above it. A reading equal to the threshold meets neither.
- `rearm_margin` is required on a numeric trigger and must be `0` or more. A binary trigger has no margin.
- `value` is the reading as the sensor reports it. Zigbee2MQTT binary readings are `true` or `false`, so a door contact reads `false` when the door is open. Values are matched without regard to case.
- `hold_seconds` is optional and defaults to `0`. In the editor it is "for at least", in seconds, minutes or hours.

An automation with only reading triggers has no next fire time. The list shows "on reading" in its Next column.

### Firing once per crossing

A reading trigger fires when its condition becomes true, not on every reading while it stays true. A heating automation sends one command when the room gets cold, not one a minute for as long as it is cold.

A sensor near a threshold jitters across it. A room hovering around 16 °C reads 15.9, 16.0, 15.9, 16.0, and firing on every one of those crossings would switch the heating on and off over and over. The **re-arm margin** stops this. Once a trigger has fired, it cannot fire again until the value has gone back past the threshold by at least the margin:

| Trigger                              | Fires at       | Can fire again once the value reaches |
|--------------------------------------|----------------|---------------------------------------|
| `falls_below` 16, margin 0.2         | below 16       | 16.2 or more                          |
| `rises_above` 20, margin 0.2         | above 20       | 19.8 or less                          |

So with "falls below 16, margin 0.2", readings of 16.2, 15.9, 15.8, 16.1, 15.9 fire once, at the first 15.9: the value never got back to 16.2. A reading of 16.2 followed by 15.9 fires it again.

Choose a margin a little larger than the sensor's usual reading-to-reading jitter. On a temperature sensor that reports in 0.1 °C steps, 0.2 is enough: replaying a week of real readings from four rooms, every threshold produced no crossing that reversed within 15 minutes, where a margin of 0 produced dozens.

A binary trigger fires when the value changes to the one it watches. A door contact that repeats "open" on its hourly heartbeat does not fire again. It fires the next time the door opens after being closed.

When the hub starts, and when an automation is saved or switched on, its reading triggers start afresh: the first reading that already meets the condition fires the trigger. After a restart in a cold room, the heating still comes on. Readings that arrive while an automation is off are ignored.

### Suggested margin

You don't have to guess how noisy a sensor is. When you choose a numeric series for a reading trigger in the editor, an empty margin is filled in with a suggestion worked out from that series' own recent readings, and labelled "Suggested from recent readings". A margin you have typed is never replaced.

The suggestion is the 95th percentile of the change between consecutive readings, rounded up to a whole multiple of the smallest change the sensor reports (its step), and at least one step. It uses the latest 1000 readings when the series has that many, else the latest 100, else the latest 30. With fewer than 30 readings there is no suggestion. It is worked out each time it is asked for, so a sensor whose settings have changed gets a suggestion that matches.

Scripts and agents get the same suggestion from the API, with `view_automations`:

```
GET /api/automations/margin-suggestion?sensor_id=3&measurement_type=temperature

{ "suggested_margin": 0.2, "step": 0.1, "p95_change": 0.2, "sample_count": 1000, "confidence": "high" }
```

`confidence` is `high` from 1000 readings, `medium` from 100, `low` from 30 and `none` below that, when `suggested_margin`, `step` and `p95_change` are null. A binary measurement type returns 400.

### Margin hints

Sensors change: a new battery, a firmware update or a different reporting interval can make a series noisier than it was when you set its margin. Once a day the hub works out a fresh suggestion for every numeric reading trigger. Where the saved margin is below it, and the suggestion comes from at least 100 readings, the trigger gets a **margin hint**. The editor shows it on the trigger card, as "This sensor is noisier now: suggested 0.4 °C", and `GET /api/automations/{id}` returns it on the trigger as `margin_hint`, with `margin_hint_checked_at`.

A hint is advice only. The hub never changes a saved margin, and sends no notification. The hint clears when you save the automation, or when a later check finds the margin at or above the suggestion.

### For at least

`hold_seconds` makes the condition hold for a while before the trigger fires. With "falls below 16, for at least 5 minutes", a reading of 15.9 at 10:00 starts the clock, and the trigger fires at 10:05:00 if no reading at or above 16 has arrived by then. It does not wait for another reading. A reading of 16.0 at 10:02 stops the clock without firing, and the trigger stays ready for the next drop.

A hold suits a door contact that bounces: "becomes open, for at least 5 seconds" ignores a door that opens and closes again within 3 seconds.

## Set steps

A set step sends a command to a writable capability of a controllable sensor, the same command you can send from a Sensor Toggle widget or `POST /api/sensors/{id}/command`:

```json
{ "type": "set", "sensor_id": 14, "property": "state", "value": "ON" }
```

Any capability the sensor has can be set, not only on and off. The value is a string, checked against the capability both when the automation is saved and again when the step runs:

| Capability type | Accepted values                          |
|-----------------|------------------------------------------|
| Binary          | its `value_on` or `value_off`            |
| Numeric         | a number within its `min` and `max`      |

`GET /api/sensors/by-id/{id}/capabilities` lists a sensor's capabilities. Sensor Hub only offers a property whose new value the device reports back, such as `state` and `brightness`, so that the step can be acknowledged. See [Device Control](sensors/device-control#only-commands-the-hub-can-confirm) for how they are detected.

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
- the sensor already has a command in flight for that property, for example because someone pressed a toggle a moment earlier. The step is not retried. A command left in flight by a cancelled run of the same automation is the exception: the step waits for it (see [When a trigger fires during a run](#when-a-trigger-fires-during-a-run)).
- the sensor is disabled or not active
- the value no longer suits the capability

A failed step ends the run as `failed`, and no later steps run. Everyone with the `manage_automations` permission gets an **Automation failures** notification naming the automation, the step and the reason, in the app and by email according to their [notification preferences](alerts-and-notifications#notification-preferences).

## Loop guard

Two automations can keep triggering each other. For example, "Plug off" switches a plug off when it turns on, and "Plug on" switches it on when it turns off. The loop guard stops a chain like this before it can drive a device forever.

A device acknowledges a command by reporting the new value. When that reading starts a run of a reading trigger, the new run records the run that sent the command as its `cause_run`. A run's cause chain is its cause run, that run's cause run, and so on. A run started by a "for at least" hold records the cause of the reading that started the hold.

When a run would start with a cause chain that is already `automation.loop.max.chain` runs long (5 by default, see [Configuration](configuration#loop-guard-chain-limit)), it does not start. Instead a `failed` run is recorded, with an error that starts with "loop guard" and names the automations in the chain. Everyone with the `manage_automations` permission gets an **Automation failures** notification, as for a failed step. The refused run sends nothing, so the chain ends there, and the automations stay switched on for the next reading that is not part of a chain.

Only readings that acknowledge an automation's command carry a cause. A reading that acknowledges a person's command, or no command at all, starts a run with no cause, so ordinary triggers never count towards a chain. The loop guard does not follow loops through the physical world, such as heating that raises a temperature that then switches the heating off. That is how a heating pair is meant to work, and a [re-arm margin](#firing-once-per-crossing) keeps it from switching too often.

## When a trigger fires during a run

An automation has at most one active run. What happens when a trigger fires while a run is going or waiting depends on the automation's `mode`, set in the editor under "If a trigger fires while already running":

| Mode      | In the editor         | What happens                                                                 |
|-----------|-----------------------|------------------------------------------------------------------------------|
| `single`  | Ignore it (single)    | The active run carries on, and the trigger is recorded as a `skipped` run.   |
| `restart` | Start over (restart)  | The active run ends as `cancelled`, and a new run starts from step 1.        |

`single` is the default. `restart` suits a timer that each new trigger should extend, such as "on, wait 5 minutes, off": every trigger starts the 5 minutes again. A cancelled run sends no more steps, but a command it already sent carries on (see [Cancelling a run](#cancelling-a-run)). A step of the new run for the same sensor and property waits for that command's outcome, then sends its own command, so starting over never strands the device. A command in flight from a person or another automation still fails the step.

Two triggers of one automation that come due at the same moment still start one run, and record no `skipped` run.

### Editing or switching off during a run

A run copies the automation's steps when it starts, so an edit never changes a run that is already going:

- Saving an automation during a run lets the run finish with the steps it started with. The next run uses the new steps.
- Switching an automation off during a run lets the run finish. No trigger fires while it is off, and none is recorded as `skipped`.

This keeps a device from being stranded. Saving "Evening lights" during its four-hour wait still switches the lamp off at the end of it. To stop a run, cancel it.

## Run now

**Run now** in the editor, or `POST /api/automations/{id}/run`, starts a run straight away. The run is recorded with trigger kind `manual` and the user who asked for it, in `initiated_by`. It works on an automation that is switched off, and it follows the automation's mode like any trigger: in `single` mode an automation that is already running records a `skipped` run instead. It is refused with `409` on an automation that is [broken](#broken).

## Cancelling a run

**Cancel run**, beside an active run in Recent runs, or `POST /api/automations/{id}/runs/{runId}/cancel`, ends a running or waiting run as `cancelled`. No further steps run, and a waiting run does not resume. A command the run had already sent carries on through its own lifecycle in the sensor's command history. Cancelling a run that has already ended returns `409`.

## Restarts and the grace window

Runs are stored in the database, so a hub restart does not lose them. Restarting the hub to update it during "on, wait 4 h, off" still switches the lamp off.

When the hub starts, before any trigger can fire:

- A waiting run carries on at its `resume_at`. If that time passed while the hub was down, the run carries on straight away, however late it is.
- A run that was in the middle of a set step finishes that step from command history. If the step's command was recorded, the step takes that command's outcome, waiting for the acknowledgement if the command is still in flight, and the command is not sent again. If no command was recorded, the step is sent.
- A schedule or interval trigger that came due while the hub was down is caught up if the hub started within `automation.missed.grace.minutes` of the due time (10 minutes by default, see [Configuration](configuration#missed-trigger-grace-window)). The run starts on startup.
- A caught-up trigger follows the automation's mode. In `restart` mode it cancels a run that would otherwise resume.
- A trigger that came due longer ago than that does not run. It is recorded once as a `missed` run, with `due_at` and `past_grace_seconds` saying when it came due and how long after the end of the grace window the hub started. A trigger missed several times in one outage, such as a schedule on several days or an interval many times over, records one `missed` run, for its latest due time. At most one run starts for it on startup.

Catching up only once, and only shortly after the due time, keeps a hub that was down overnight from switching yesterday evening's lights on at breakfast.

If the hub stops after a command was published but before it was recorded, that command is sent again when the hub starts.

## Runs and command history

Every run is recorded with the trigger that fired it, or `manual` and the user for Run now, its cause run if another automation's command started it (see [Loop guard](#loop-guard)), its status, the step it is on, a copy of the steps it started with, its start and finish times and any error. `GET /api/automations/{id}/runs` lists them newest first, with the outcome of each step.

A run's status is `running` while it goes and `waiting` during a wait step, then `succeeded`, `failed` or `cancelled`. Two statuses record a run that never started: `missed` for a trigger that came due while the hub was down, past the grace window (see [Restarts and the grace window](#restarts-and-the-grace-window)), and `skipped` for a trigger that fired while the automation was already running in `single` mode (see [When a trigger fires during a run](#when-a-trigger-fires-during-a-run)).

Commands sent by a run go out from the hub itself rather than from a user. In a sensor's command history (`GET /api/sensors/by-id/{id}/commands`) they carry the automation's id and name instead of a user, which answers "why did this switch on?". A command keeps its automation after its run is deleted by [run history retention](configuration#automation-run-history-retention), which keeps runs for less time than command history by default.

## Status

Each automation reports a status:

| Status    | Meaning                                                        |
|-----------|----------------------------------------------------------------|
| `off`     | switched off; its triggers start nothing                       |
| `armed`   | switched on and waiting for a trigger                          |
| `running` | a run is in progress or waiting                                |
| `broken`  | a set step no longer matches its device; see [Broken](#broken) |

A separate `last_run_failed` flag is true from a failed run until the next run that succeeds.

`next_fire_at` is when the earliest schedule or interval trigger next comes due, in UTC, with `hub_timezone` alongside for showing it in local time. It is empty when the automation is off or broken, or only has reading triggers.

### Broken

An automation is broken when a set step targets a property its device no longer has as a capability, or a device that can no longer be controlled. This happens without anything being deleted. A Zigbee device's capabilities come from the `exposes` metadata that Zigbee2MQTT republishes with its device list, so a firmware update or a re-pair can drop a property. An upgrade can drop one too: a property Sensor Hub can't confirm, such as `effect` or `color_temp`, is no longer offered, so an automation saved with a step on it shows as broken after the upgrade.

`status_reason` names the step and the property, such as `step 2: hallway-lamp no longer has color_temp_preset`, and the Automations list shows it under the Broken status.

- A broken automation starts no runs and records nothing, however often its triggers come due. Run now is refused with `409`. A run that was already going carries on, and fails at the step that no longer matches.
- The hub checks again whenever a device's metadata or driver changes: a Zigbee2MQTT device-list refresh, or a sensor update through the API. It also checks every automation when it starts.
- When an automation becomes broken, holders of `manage_automations` get one `automation_failure` notification naming the automation and the reason. No more are sent while it stays broken, and a restart does not send another.
- It stops being broken, without a notification, when the property comes back in a later device-list refresh, or when you edit it so that every set step is valid and save it. If it breaks again later, it notifies again.

A value that no longer fits its property, such as a brightness above a new maximum, does not make an automation broken. The step fails when it runs.

Switch an automation on or off with `PUT /api/automations/{id}/enabled`. Deleting an automation deletes its triggers, steps and run history. The commands its runs sent stay in command history, without the link to the run or the automation. An automation with a running or waiting run cannot be deleted: the delete returns `409` until the run is cancelled or finishes.

### Deleting a sensor

Deleting a sensor deletes every automation that uses it in a trigger or a set step, including any run in progress. Nothing asks first. An automation that switches the deleted device and then another device is deleted outright, so the other device is not switched by it again, and a run waiting to switch it is dropped. Commands those automations sent to other devices stay in command history, without the link to the run or the automation.

To keep an automation, edit it to stop using the sensor before deleting the sensor.

## Permissions

| Permission           | Granted to             | Allows                                                              |
|----------------------|------------------------|---------------------------------------------------------------------|
| `view_automations`   | admin, user, viewer    | listing automations and their runs                                  |
| `manage_automations` | admin, user            | creating, editing, switching, running, cancelling runs and deleting |

Creating, editing, switching an automation on or off and Run now also need `control_sensors`, so an automation cannot be used to control devices you could not control yourself. Once saved, an automation runs as the hub: it keeps working if its author later loses `control_sensors`.

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

## Example: heating pair

A heating loop is two automations, one to switch the heating on and one to switch it off. Here the living room heats below 16 °C and stops above 20 °C:

```json
{
  "name": "Lounge heat on",
  "triggers": [
    { "type": "reading", "sensor_id": 3, "measurement_type": "temperature",
      "operator": "falls_below", "threshold": 16, "rearm_margin": 0.2 }
  ],
  "steps": [
    { "type": "set", "sensor_id": 21, "property": "state", "value": "ON" }
  ]
}
```

```json
{
  "name": "Lounge heat off",
  "triggers": [
    { "type": "reading", "sensor_id": 3, "measurement_type": "temperature",
      "operator": "rises_above", "threshold": 20, "rearm_margin": 0.2 }
  ],
  "steps": [
    { "type": "set", "sensor_id": 21, "property": "state", "value": "OFF" }
  ]
}
```

Each sends its command once per crossing, and the gap between 16 and 20 keeps the two from fighting. Both start afresh after a restart, so whichever condition holds at the first reading puts the heating in the right state.

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

Send an automation with `POST /api/automations`. A new automation is switched on unless the body sets `"enabled": false`, and is in `single` mode unless it sets `"mode": "restart"`. An update that leaves out `enabled` or `mode` keeps the current setting. A save the hub refuses comes back as `400` with a message naming the field, such as `triggers[0].days must hold at least one weekday`.
