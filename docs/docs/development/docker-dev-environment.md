# Docker Dev Environment

The dev stack in `sensor_hub/devstack` runs the hub, the Vite UI and a set of
mock sensors with hot reload, and seeds a dataset on first start so there's
something to look at straight away.

## Start the Stack

```bash
cd sensor_hub/devstack
docker compose up --build --watch
```

Open [http://localhost:3000](http://localhost:3000). Go and React edits are
synced into the containers by Compose Watch, so the stack needs `--watch`.
Without it the containers start fine but never see an edit.

A one-shot `seed` service runs before the hub on every start. A change to
`go.mod`, `go.sum`, `package.json`, `package-lock.json` or
`mocks/requirements.txt` rebuilds the affected image on its own.

## Grafana and Delve

Both are off by default. Add either overlay to turn it on:

```bash
docker compose -f compose.yaml -f compose.grafana.yaml up --build --watch
docker compose -f compose.yaml -f compose.delve.yaml up --build --watch
```

To keep them on, copy `.env.example` to `.env` and trim `COMPOSE_FILE` to the
overlays you want. A plain `docker compose up --build --watch` then picks them
up. `.env` is git-ignored.

- **Grafana** receives logs, traces and metrics from the hub, and traces from
  the HTTP mocks. Sign in with admin / admin.
- **Delve** runs the hub under `dlv debug`. Attach your IDE's debugger (DAP or
  the Delve API v2) to `localhost:2345` at any time.

## Seeded Logins, API Key and Sensors

From an empty volume the seed creates these users, none of which has to change
their password:

| Username | Password | Role |
|---|---|---|
| `admin` | `adminpassword` | admin |
| `user` | `userpassword` | user |
| `viewer` | `viewerpassword` | viewer |

It also creates an admin API key and prints it on every start, for as long as
the key still works:

```bash
docker compose logs seed | grep admin_api_key
```

The hub is subscribed to `zigbee2mqtt/#`, and every mock sensor is registered
and approved: the Zigbee2MQTT devices in `mocks/mqtt_devices.py` and the two
HTTP mocks. Live readings land on those sensors from the first start, and
`office-plug` can be switched on and off from the hub.

The seed also creates range alert rules on `living-room-sensor` temperature and
`kitchen-sensor` humidity, a status rule on the `front-door` contact, and a few
notifications, some already read.

The seed only creates these once, so anything you change or delete stays that
way. A deleted MQTT sensor comes back as a pending sensor while its mock device
keeps publishing. The passwords and key are for local development only.

On every start the seed tops up the seeded sensors' readings, health history
and alert history from their newest reading, or from 30 days ago if that's
later, so charts over any range up to 30 days have no gap after downtime. A
14-day gap takes about 5 to 6 seconds.

## Ports

| Port | Service |
|---|---|
| 3000 | UI (Vite dev server) |
| 8080 | Hub API |
| 1883 | Hub's embedded MQTT broker |
| 5001, 5002 | HTTP mock sensors |
| 4000 | Grafana (Grafana overlay) |
| 4317, 4318 | OTLP gRPC and HTTP (Grafana overlay) |
| 2345 | Delve (Delve overlay) |

## Reset

```bash
docker compose down -v
```

This deletes the database and the Go caches, so the next start seeds from
scratch. If you started an overlay with `-f`, pass the same `-f` flags here
too. The smoke check, `node smoke-check.mjs`, runs `down -v` before it starts,
so running it wipes your dev data as well.

## Removing the Old Stack

The old stack that this one replaced left its containers and volumes behind.
This removes them once:

```bash
docker ps -aq --filter label=com.docker.compose.project=docker_tests | xargs -r docker rm -f
docker volume ls -q --filter name=^docker_tests_ | xargs -r docker volume rm
```
