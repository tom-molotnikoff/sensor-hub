---
name: sensor-hub
description: Interact with a Sensor Hub home monitoring instance via CLI
---

# Sensor Hub CLI

`sensor-hub` is a CLI tool for interacting with a Sensor Hub instance — a home sensor monitoring system supporting multiple sensor types via pluggable drivers.

## Discovery

Run `sensor-hub --help` to see all available commands.
Run `sensor-hub <command> --help` for detailed usage of any command.
Run `sensor-hub <command> <subcommand> --help` for subcommand details.

## Prerequisites

The CLI must be configured with a server URL and API key:
- Config file: `~/.sensor-hub.yaml`
- Run `sensor-hub config init` for interactive setup
- Or pass `--server` and `--api-key` flags to any command

## Output

All commands output JSON to stdout. Errors go to stderr.
Pipe through `jq` for filtering: `sensor-hub sensors list | jq '.[].name'`

## Command Reference

### Health
```bash
sensor-hub health
```

### Sensors
```bash
sensor-hub sensors list                              # List active sensors
sensor-hub sensors list --status all                 # List every sensor, including pending and dismissed
sensor-hub sensors get "Living Room"                 # Get by name
sensor-hub sensors exists "Living Room"              # Check if exists
sensor-hub sensors list-by-driver sensor-hub-http-temperature  # List by driver
sensor-hub sensors add --name X --driver sensor-hub-http-temperature --config url=Z  # Create sensor
sensor-hub sensors update 1 --name X --config url=Z  # Update by ID
sensor-hub sensors update 1 --retention-hours 48     # Set per-sensor retention (hours)
sensor-hub sensors update 1 --retention-hours 0      # Clear per-sensor retention (use global default)
sensor-hub sensors delete "Living Room"              # Delete by name, and every automation that uses it
sensor-hub sensors enable "Living Room"              # Enable sensor
sensor-hub sensors disable "Living Room"             # Disable sensor
sensor-hub sensors health "Living Room"              # Health history
sensor-hub sensors stats                             # Total readings per sensor
sensor-hub sensors collect                           # Collect all
sensor-hub sensors collect "Living Room"             # Collect specific
sensor-hub drivers list                              # List available sensor drivers
sensor-hub sensors pending                           # List pending (auto-discovered) sensors
sensor-hub sensors approve 5                         # Approve a pending sensor by ID
sensor-hub sensors dismiss 5                         # Dismiss a pending sensor by ID
```

**Deleting a sensor** also deletes every automation that uses it in a trigger or a set step, including a run in progress, without asking. To keep an automation, update it to stop using the sensor first.

**Sensor status.** Every sensor has a `status` field:
- `active` - an installed sensor that Sensor Hub collects readings from. These are the user's real sensors.
- `pending` - auto-discovered (e.g. from MQTT) and awaiting approval with `sensors approve`. Not yet a real sensor.
- `dismissed` - auto-discovered and rejected. Not a real sensor; ignore these. They can still report `enabled: true` and health `unknown`, so never judge a sensor by those fields alone.

`sensors list` shows only active sensors by default. Use `--status pending`, `--status dismissed` or `--status all` to see the rest. Zigbee bridge topics such as `bridge/response/permit_join` often end up dismissed, and names containing `/` work as-is in name-addressed commands.

**Sensor response fields (GET /api/sensors/:name):**
- `retention_hours` — nullable integer; per-sensor data retention override in hours. Null means use the global default.
- `effective_retention_hours` — read-only integer; the retention period in hours that actually applies (per-sensor value if set, otherwise the global default).

### MQTT Brokers
```bash
sensor-hub mqtt brokers list                         # List all MQTT brokers
sensor-hub mqtt brokers get 1                        # Get broker by ID
sensor-hub mqtt brokers create --name "zigbee" --host localhost --port 1883  # Create broker
printf '%s\n' "$BROKER_PASSWORD" | sensor-hub mqtt brokers create --name "remote" --host mqtt.example.com --port 8883 --username user --password-stdin
sensor-hub mqtt brokers update 1 --file broker.json  # Update from JSON file; omit "password" to keep the stored one
sensor-hub mqtt brokers delete 1                     # Delete by ID
sensor-hub mqtt brokers enable 1                     # Enable a broker
sensor-hub mqtt brokers disable 1                    # Disable a broker
```

Broker passwords are write-only: responses carry `password_status` (`unset`, `set`, `needs_reentry`) and never the password. A password is never passed as a flag; `create` reads it from `--password-stdin` or a prompt, only when `--username` is given; an empty one means the broker takes none. In an update body, omitting `password` or sending `"****"` keeps the stored one and `""` clears it.

### MQTT Subscriptions
```bash
sensor-hub mqtt subscriptions list                   # List all subscriptions
sensor-hub mqtt subscriptions list --broker-id 1     # Filter by broker
sensor-hub mqtt subscriptions get 1                  # Get subscription by ID
sensor-hub mqtt subscriptions create --broker-id 1 --topic "zigbee2mqtt/#" --driver mqtt-zigbee2mqtt  # Create
sensor-hub mqtt subscriptions create --broker-id 1 --topic "rtl_433/#" --driver mqtt-rtl433 --qos 1
sensor-hub mqtt subscriptions update 1 --file sub.json  # Update from JSON file
sensor-hub mqtt subscriptions delete 1               # Delete by ID
```

### Readings
```bash
sensor-hub readings between --start 2026-03-01 --end 2026-03-26
sensor-hub readings between --start 2026-03-26T10:00:00Z --end 2026-03-26T16:00:00Z
sensor-hub readings between --sensor "Living Room" --start 2026-03-01 --end 2026-03-26
sensor-hub readings between --type temperature --start 2026-03-01 --end 2026-03-26
sensor-hub readings between --start 2026-03-01 --end 2026-03-26 --aggregation PT1H
sensor-hub readings between --start 2026-03-01 --end 2026-03-26 --aggregation raw
sensor-hub readings between --sensor "Front Door" --type contact --start 2026-03-01 --end 2026-03-26 --aggregation-function count
```

> **Start/end** accept either `YYYY-MM-DD` (expanded to full day) or ISO 8601 datetime (e.g. `2026-03-26T10:00:00Z`). All timestamps are stored and returned in UTC. The server auto-aggregates readings based on the time span; use `--aggregation` to override the interval (e.g. `PT1H`, `PT5M`, or `raw` for no aggregation) and `--aggregation-function` to override the function (`avg`, `count`, `last`, `min`, `max`, `increase`, where `increase` gives how much an energy counter rose per bucket). `--type` filters to one measurement type and is required for `--aggregation-function`; `measurement-types list` shows the functions each type supports. Without `--type`, auto-aggregation averages every type, so binary types such as `contact` or `motion` come back with no values - pass `--type` or `--aggregation raw` for those.

### Measurement Types
```bash
sensor-hub measurement-types list                    # List all measurement types
sensor-hub measurement-types list --has-readings     # Only types with stored readings
sensor-hub measurement-types for-sensor 1            # Types supported by sensor ID 1
```

### Alerts
```bash
sensor-hub alerts list                               # List all rules
sensor-hub alerts get 1                              # Get by rule ID
sensor-hub alerts create --sensor-id 1 --measurement-type-id 1 --type HIGH_TEMP --threshold 30
sensor-hub alerts update 1 --alert-type HIGH_TEMP --high-threshold 30 --low-threshold 10 --enabled --rate-limit-seconds 3600
sensor-hub alerts delete 1                           # Delete by rule ID
sensor-hub alerts history 1                          # Alert history for rule
sensor-hub alerts history 1 --limit 20               # With limit
```

> Multiple alerts can be created per sensor (one per measurement type + alert type combo). Rate limit is in seconds (e.g. 60 = 1 minute, 3600 = 1 hour).

### Automations
```bash
sensor-hub automations list                          # Every automation with status and next fire time
sensor-hub automations get 3                         # One automation by ID
sensor-hub automations create --file timer.json      # Create from a JSON file
cat timer.json | sensor-hub automations create --file -   # Create from stdin
sensor-hub automations update 3 --file timer.json    # Replace name, triggers and steps
sensor-hub automations delete 3                      # Delete with its triggers, steps and runs
sensor-hub automations enable 3                      # Switch on
sensor-hub automations disable 3                     # Switch off (a run in progress finishes)
sensor-hub automations run 3                         # Run now, even when switched off
sensor-hub automations runs 3                        # Runs, newest first, with step outcomes
sensor-hub automations cancel 3 42                   # Cancel run 42 of automation 3
sensor-hub automations margin-suggestion --sensor-id 3 --measurement-type temperature
```

An automation is one or more triggers (any of them starts a run) followed by steps run top to bottom. `create` and `update` take the same JSON as `POST /api/automations`. A new automation is switched on unless the JSON sets `"enabled": false`, and is in `"single"` mode unless it sets `"mode": "restart"`. An update that leaves out `enabled` or `mode` keeps the current setting. A rejected save fails with HTTP 400 and a message naming the field.

**Triggers:**
- `{"type": "schedule", "at": "19:00", "days": ["mon", "tue", "wed", "thu", "fri"]}` - time of day in the hub's timezone (the `hub.timezone` property), at least one day.
- `{"type": "interval", "seconds": 1800}` - every so many seconds, at least 60, counted from the last save or enable.
- `{"type": "reading", "sensor_id": 3, "measurement_type": "temperature", "operator": "falls_below", "threshold": 16, "rearm_margin": 0.2}` - numeric types take `falls_below` or `rises_above` with a required `rearm_margin`. It fires once per crossing, and only again after the value has gone back past the threshold by the margin.
- `{"type": "reading", "sensor_id": 9, "measurement_type": "contact", "operator": "becomes", "value": "false"}` - binary types take `becomes` with the reading's raw text and no margin. Zigbee2MQTT binary readings are `"true"`/`"false"`, so a contact sensor opening is `"value": "false"`. Matching ignores case.
- Any reading trigger can add `"hold_seconds": 300` so the condition must hold that long before it fires.
- A reading trigger only saves once the sensor has sent at least one reading of that measurement type (`sensor-hub measurement-types for-sensor 3`).
- Use `margin-suggestion` for `rearm_margin`. `suggested_margin` is null when `confidence` is `"none"` (fewer than 30 readings). `get` shows `margin_hint` on a trigger when the daily check found the sensor noisier than its saved margin allows. The margin is never changed for you.

**Steps:**
- `{"type": "set", "sensor_id": 14, "property": "state", "value": "ON"}` - any writable capability, with the value as a string, as for `sensor-hub sensors command`. See `sensor-hub sensors capabilities 14` for properties and allowed values. The next step waits until the device acknowledges it, and a failed or timed-out command fails the run.
- `{"type": "wait", "seconds": 14400}` - at least 1 second, no maximum. Waits survive a hub restart.

**Status** (from `list` and `get`): `off`, `armed`, `running` or `broken`, with `last_run_failed`. `broken` means a set step targets a property its device no longer has, and `status_reason` names it, such as `step 2: hallway-lamp no longer has color_temp_preset`. A broken automation starts no runs until it is saved with valid steps or the device gets the property back. `next_fire_at` is UTC, with the zone in `hub_timezone`.

**Runs:** statuses are `running` and `waiting` while active, then `succeeded`, `failed` or `cancelled`, or `missed` and `skipped` for runs that never started. `cause_run` names the run, usually of another automation, whose command acknowledgement started this one. A run refused by the loop guard (`automation.loop.max.chain`) is `failed` with an error starting `loop guard`.

**Run, cancel and delete:**
- `run` prints a run, but in `single` mode an automation already running records a skipped run instead: check that `status` is not `"skipped"`. In `restart` mode it cancels the active run first. It fails with HTTP 409 on a broken automation.
- `cancel` fails with HTTP 409 when the run has already ended, and 404 when the run is not that automation's. A command already sent carries on.
- `delete` fails with HTTP 409 while the automation has a running or waiting run. Cancel it first.

#### Example: lights timer

Lamp on at 19:00 on weekdays, dimmed, and off again 4 hours later:

```json
{
  "name": "Lamp timer",
  "triggers": [
    { "type": "schedule", "at": "19:00", "days": ["mon", "tue", "wed", "thu", "fri"] }
  ],
  "steps": [
    { "type": "set", "sensor_id": 14, "property": "state", "value": "ON" },
    { "type": "set", "sensor_id": 14, "property": "brightness", "value": "150" },
    { "type": "wait", "seconds": 14400 },
    { "type": "set", "sensor_id": 14, "property": "state", "value": "OFF" }
  ]
}
```

#### Example: heating pair

Two automations: heating plug 21 on below 16 °C and off above 20 °C on sensor 3. Get the margin from `sensor-hub automations margin-suggestion --sensor-id 3 --measurement-type temperature` first.

```bash
sensor-hub automations create --file - <<'JSON'
{
  "name": "Lounge heat on",
  "triggers": [
    { "type": "reading", "sensor_id": 3, "measurement_type": "temperature",
      "operator": "falls_below", "threshold": 16, "rearm_margin": 0.2 }
  ],
  "steps": [ { "type": "set", "sensor_id": 21, "property": "state", "value": "ON" } ]
}
JSON
sensor-hub automations create --file - <<'JSON'
{
  "name": "Lounge heat off",
  "triggers": [
    { "type": "reading", "sensor_id": 3, "measurement_type": "temperature",
      "operator": "rises_above", "threshold": 20, "rearm_margin": 0.2 }
  ],
  "steps": [ { "type": "set", "sensor_id": 21, "property": "state", "value": "OFF" } ]
}
JSON
```

The gap between 16 and 20 stops the two fighting. After a restart, save or enable, the first reading fires whichever trigger it already meets, so the heating ends up in the right state.

### Notifications
```bash
sensor-hub notifications list                        # List notifications
sensor-hub notifications list --limit 10 --offset 20 --include-dismissed
sensor-hub notifications unread-count                # Unread count
sensor-hub notifications read 5                      # Mark as read
sensor-hub notifications dismiss 5                   # Dismiss
sensor-hub notifications bulk-read                   # Mark all as read
sensor-hub notifications bulk-dismiss                # Dismiss all
sensor-hub notifications preferences                 # Get preferences
sensor-hub notifications set-preference --category threshold_alert --email-enabled --inapp-enabled
```

### Auth
```bash
sensor-hub auth login --username admin --password secret
sensor-hub auth logout
sensor-hub auth me                                   # Current user info
sensor-hub auth sessions                             # List sessions
sensor-hub auth revoke-session abc123                # Revoke session
```

### Users
```bash
sensor-hub users list
printf '%s\n' "$PASSWORD" | sensor-hub users create --username X --email Z --password-stdin   # Omit --password-stdin to be prompted
sensor-hub users delete 2
printf '%s\n' "$PASSWORD" | sensor-hub users change-password --user-id 1 --password-stdin
sensor-hub users set-must-change 1 --must-change
sensor-hub users set-roles 1 --roles admin,viewer
sensor-hub users disable 2                           # Sign out and block a user; their API keys stop working
sensor-hub users enable 2
```

### Roles
```bash
sensor-hub roles list                                # List roles
sensor-hub roles list-permissions                    # List all permissions
sensor-hub roles get-permissions 1                   # Permissions for role
sensor-hub roles assign-permission 1 --permission-id 5
sensor-hub roles remove-permission 1 5               # roleId permissionId
```

### API Keys
```bash
sensor-hub api-keys list
sensor-hub api-keys create --name "my-key"
sensor-hub api-keys update-expiry 3 --expires-at "2026-12-31T23:59:59Z"
sensor-hub api-keys revoke 3
sensor-hub api-keys delete 3
```

### Email
```bash
sensor-hub email show                                # SMTP settings, password status, last send
sensor-hub email set --host smtp.example.com --port 587 --security starttls \
  --username USER --from-address alerts@example.com --password-stdin  # password on stdin
sensor-hub email test                                # Send a test email to your own address
```

### Properties
```bash
sensor-hub properties get                            # Get all properties
sensor-hub properties set --key weather.latitude --value 53.3811
sensor-hub properties set --key hub.timezone --value Europe/London  # Zone automation schedules run in
```

### Dashboards
```bash
sensor-hub dashboards list                           # List all dashboards
sensor-hub dashboards get 1                          # Get dashboard by ID
sensor-hub dashboards create --name "My Dashboard"   # Create dashboard
sensor-hub dashboards delete 1                       # Delete by ID
sensor-hub dashboards update 1 --file dashboard.json # Update from JSON file
```

The `update` command requires a JSON file with the full dashboard structure.

#### Dashboard JSON schema

```json
{
  "name": "My Dashboard",
  "config": {
    "widgets": [
      {
        "id": "unique-string-id",
        "type": "readings-chart",
        "config": { "measurementType": "temperature" },
        "layout": { "x": 0, "y": 0, "w": 6, "h": 4 }
      }
    ]
  }
}
```

- `id`: Unique string per widget (e.g. UUID or descriptive slug)
- `layout`: Grid position on the 12-column grid — `x` (column), `y` (row), `w` (width in columns), `h` (height in row units)

#### Available widget types and their config fields

| type                 | config fields                                                                                                              | description                                  |
|----------------------|----------------------------------------------------------------------------------------------------------------------------|----------------------------------------------|
| `readings-chart`     | `measurementType` (measurement-type), `timeRange` (time-range, default "24h"), `refreshInterval` (number, default 30), `aggregationFunction` (aggregation-function-select, dynamic — shows only functions supported by the selected measurement type; empty = auto) | Line chart for any measurement type          |
| `live-readings`      | —                                                                                                                          | Real-time sensor readings data grid          |
| `weather-forecast`   | —                                                                                                                          | External weather forecast card               |
| `sensor-health-pie`  | —                                                                                                                          | Sensor health status pie chart               |
| `sensor-type-pie`    | —                                                                                                                          | Sensor driver distribution pie chart         |
| `health-timeline`    | `sensorId` (number)                                                                                                        | Retained-window health status timeline for a sensor |
| `reading-stats`      | —                                                                                                                          | Total readings per sensor data grid          |
| `notifications-feed` | —                                                                                                                          | Recent notifications feed                    |
| `markdown-note`      | `content` (string)                                                                                                         | User-defined markdown text block             |
| `current-reading`    | `sensorId` (number), `measurementType` (measurement-type)                                                                  | Current value display for a sensor (numeric or binary/text) |
| `min-max-avg`        | `sensorId` (number), `measurementType` (measurement-type), `timeRange` (time-range, default "24h")                         | Min/max/avg statistics for a sensor          |
| `gauge`              | `sensorId` (number), `measurementType` (measurement-type), `min` (number, default 0), `max` (number, default 40)            | Reading gauge dial for a single sensor       |
| `comparison-chart`   | `measurementType` (measurement-type), `sensorIds` (number[]), `timeRange` (time-range, default "24h"), `refreshInterval` (number, default 30), `aggregationFunction` (aggregation-function-select, dynamic — shows only functions supported by the selected measurement type; empty = auto) | Multi-sensor overlay line chart        |
| `group-summary`      | `measurementType` (measurement-type)                                                                                       | Average reading for a measurement type across all sensors |
| `alert-summary`      | —                                                                                                                          | Compact list of configured alert rules       |
| `uptime`             | `sensorId` (number)                                                                                                        | Time-weighted uptime over the retained health window |
| `heatmap`            | `sensorId` (number), `measurementType` (measurement-type), `scaleMin` (number, default 10), `scaleMax` (number, default 30) | Colour-coded 30-day heatmap                 |
| `sensor-detail`      | `sensorId` (number)                                                                                                        | Latest readings grid for a sensor            |
| `sensor-toggle`      | `sensorId` (controllable binary sensor), `property` (binary capability property, default `state`)                         | Large optimistic on/off switch for a controllable sensor |

**Config field notes:**
- `sensorId` is a numeric sensor ID (see `sensor-hub sensors list` to find IDs)
- `sensorIds` is an array of numeric sensor IDs
- `measurementType` is a measurement type name (e.g. `"temperature"`, `"humidity"`, `"power"`) — see `sensor-hub measurement-types list` for all types, or `sensor-hub measurement-types for-sensor <id>` for types supported by a specific sensor
- `timeRange` is a relative time preset: `"5m"`, `"15m"`, `"30m"`, `"1h"`, `"6h"`, `"24h"`, `"3d"`, `"7d"`, `"30d"`, or `"custom"`. When `"custom"`, also set `customStart` and `customEnd` as ISO date strings. Defaults to `"24h"` if omitted.
- Legacy `startDate` / `endDate` ISO date strings still work for backward compatibility but prefer `timeRange`
- `refreshInterval` is the polling interval in seconds for chart data updates; defaults to 30 if omitted
- Health history widgets use the full retained history window configured by `health.history.retention.days`
- Legacy type `temperature-chart` is an alias for `readings-chart` and still works

#### Example: dashboard with two widgets

```json
{
  "name": "Living Room Monitor",
  "config": {
    "widgets": [
      {
        "id": "readings-chart-1",
        "type": "readings-chart",
        "config": { "measurementType": "temperature", "timeRange": "3d" },
        "layout": { "x": 0, "y": 0, "w": 8, "h": 4 }
      },
      {
        "id": "gauge-living",
        "type": "gauge",
        "config": { "sensorId": 1, "measurementType": "temperature", "min": 10, "max": 35 },
        "layout": { "x": 8, "y": 0, "w": 4, "h": 4 }
      }
    ]
  }
}
```
