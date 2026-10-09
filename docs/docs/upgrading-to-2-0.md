---
id: upgrading-to-2-0
title: Upgrading to 2.0
---

# Upgrading to 2.0

2.0 makes the hub safe to run on the internet. It encrypts the passwords it holds for other systems, sends email over SMTP instead of Gmail OAuth, makes every device log in to the embedded MQTT broker, and listens only on the hub's own machine unless you say otherwise.

For a 1.5.x package install the upgrade itself is one package install. Around it there are up to ten steps, and most installs need only some of them: each step says who needs it. Do them in this order.

:::warning[The upgrade cannot be undone]
The first 2.0 start rewrites the database in a way 1.5.x cannot use, and the install deletes the Gmail OAuth files. Going back to 1.5.x needs the backup from step 1. See [Going back to 1.5.x](#going-back-to-15x).
:::

| Step | Who needs it |
|---|---|
| [1. Back up](#1-back-up) | Everyone |
| [2. Check the systemd unit](#2-check-the-systemd-unit) | Everyone checks; only a changed unit needs work |
| [3. Add the settings your setup needs](#3-add-the-settings-your-setup-needs) | Some setups, such as no nginx on the hub's machine, devices on your network, or keeping old command history |
| [4. Install the package](#4-install-the-package) | Everyone |
| [5. Save the secret-store key](#5-save-the-secret-store-key) | Everyone |
| [6. Give Zigbee2MQTT a login](#6-give-zigbee2mqtt-a-login) | Anything that publishes to the embedded broker |
| [7. Enter the SMTP settings](#7-enter-the-smtp-settings) | Anyone who gets email from the hub |
| [8. Remove SENSOR_HUB_INITIAL_ADMIN](#8-remove-sensor_hub_initial_admin) | Only if you set it |
| [9. Check Prometheus and nginx](#9-check-prometheus-and-nginx) | If you scrape metrics, or your nginx differs from the example |
| [10. Rotate broker passwords and revoke the Google token](#10-rotate-broker-passwords-and-revoke-the-google-token) | If the hub connected out to a broker with a password, or sent email through Gmail |

## Before you install

### 1. Back up

`local db backup` arrives with 2.0, and the install starts 2.0 straight away, so it cannot take this copy. Stop the service and copy the data directory, which holds the database with its `-wal` file, and the configuration directory:

```bash
sudo systemctl stop sensor-hub
sudo install -d -m 0700 /var/backups/sensor-hub-1.5
sudo cp -a /var/lib/sensor-hub /var/backups/sensor-hub-1.5/lib
sudo cp -a /etc/sensor-hub /var/backups/sensor-hub-1.5/etc
```

Leave the service stopped. The install starts it again.

The copy holds your broker passwords and the Gmail token in plain text, so keep it where only root can read it, and delete it once you are sure you will stay on 2.0. From 2.0 on, take backups with [`local db backup`](cli-tool#back-up-the-database), which needs no stop.

### 2. Check the systemd unit

The service now runs `sensor-hub local serve`, since `sensor-hub serve` no longer exists, and runs `sensor-hub-key-check.service` before each start. A change you made to the unit can stop either. Look at what systemd runs:

```bash
systemctl cat sensor-hub
```

If it shows only `/usr/lib/systemd/system/sensor-hub.service`, or drop-ins that only add settings, such as ordering after a VPN, there is nothing to do. Otherwise:

- **A drop-in in `/etc/systemd/system/sensor-hub.service.d/` that sets `ExecStart=`**: remove it if you no longer need it, or change `sensor-hub serve` in it to `sensor-hub local serve`.
- **A full copy of the unit in `/etc/systemd/system/sensor-hub.service`**, made with `systemctl edit --full`: it hides the new unit. Delete it, and put any change you still need in a drop-in with `sudo systemctl edit sensor-hub`.

Make the change just before you install, since 1.5.x has no `local serve`. The install reloads systemd.

### 3. Add the settings your setup needs

2.0 listens on the hub's own machine only: the web UI and API on `127.0.0.1:8080`, metrics on `127.0.0.1:9464` and the MQTT broker on `127.0.0.1:1883`. It also believes the client address a proxy forwards only from a proxy on the hub's machine, unless you list others. The upgrade keeps your `application.properties`, so add a line to the end of `/etc/sensor-hub/application.properties` for each row that fits your setup, for example with `sudoedit`. nginx on the hub's machine, as in [Nginx Setup](nginx-setup), needs no line.

| If | Add |
|---|---|
| nginx runs on another host | `http.listen.address=0.0.0.0:8080` and `http.trusted.proxies=` nginx's address |
| There is no nginx: browsers and the CLI reach the hub on port 8080 | `http.listen.address=0.0.0.0:8080` |
| Zigbee2MQTT or another device on your home network connects to the embedded broker | `mqtt.broker.listen.address=0.0.0.0`, or the hub's LAN address |
| Devices reach the broker over WireGuard or another VPN | `mqtt.broker.listen.address=` the hub's tunnel address, and order sensor-hub after the tunnel as [Connecting your home](connecting-your-home#wireguard-or-another-vpn) shows |
| Prometheus scrapes the hub from another host | `metrics.listen.address=0.0.0.0:9464`, with a firewall that lets only Prometheus reach port 9464 |
| You want to keep command history older than 90 days | `command.history.retention.days=0` |

With nginx on another host and its address not in `http.trusted.proxies`, every request through nginx seems to come from nginx itself, so five failed logins by anyone hold up everyone's logins for a while.

2.0 deletes commands sent to devices more than `command.history.retention.days` ago, 90 by default, and the first cleanup runs as it starts. `0` keeps them all (see [Command history retention](configuration#command-history-retention)).

Devices on the hub's own machine, an SSH port forward and nginx's MQTT port all reach the broker on `127.0.0.1`, so they need no broker setting. 1.5.x ignores these lines, but saving on its Properties page drops them, so add them after your last change there. The install's restart puts them into effect.

## Install

### 4. Install the package

```bash
sudo apt install ./sensor-hub_*.deb       # Debian / Ubuntu
sudo dnf upgrade ./sensor-hub_*.rpm       # Fedora / RHEL
```

The install asks nothing and does the rest on its own:

- It creates the secret-store key, sealed with the TPM where the host has one and otherwise in `/etc/sensor-hub/secrets.key` (see [The secret-store key](installation#the-secret-store-key)).
- It deletes `/etc/sensor-hub/credentials.json` and `/etc/sensor-hub/token.json`, the Gmail OAuth files.
- It keeps your configuration files as they are (see [Configuration files](upgrading#configuration-files)). On Fedora and RHEL it prints `warning: ... saved as ... .rpmsave` for each file you changed, then `Kept your /etc/sensor-hub/...`. Both are expected, and no `.rpmsave` file is left behind.
- It starts the service. On that first start the hub encrypts every outbound broker password into the secret store and wipes the plain text from the database and its `-wal` file, and carries the old `smtp.user` into the new email settings.

Check the hub is up:

```bash
sudo systemctl status sensor-hub
curl -k https://localhost/api/health
```

Without nginx, check `http://localhost:8080/api/health` instead.

From now until step 6, the embedded broker refuses devices that have no login, so their readings stop.

## After you install

### 5. Save the secret-store key

```bash
sudo sensor-hub local secrets show-key
```

Keep the line it prints somewhere other than the hub, such as a password manager. You need it to restore a backup on another machine, or if the TPM refuses the key after a firmware update. Without it, every broker and SMTP password has to be entered again.

### 6. Give Zigbee2MQTT a login

Skip this if nothing publishes to the hub's embedded broker. Anything that does has had its readings refused since the install, and they start again as soon as it logs in.

The embedded broker no longer accepts anonymous devices. Each device needs an MQTT client, which can publish and subscribe only under its own topic prefix:

1. On the **MQTT** page, click **Add Client** on the **MQTT Clients** card. Name it `zigbee2mqtt` and give it the prefix `zigbee2mqtt/`, which is Zigbee2MQTT's `base_topic` followed by `/`. Copy the password: it is shown once. From the CLI:

   ```bash
   sensor-hub mqtt clients create --name zigbee2mqtt --topic-prefix zigbee2mqtt/
   ```

2. In Zigbee2MQTT's `configuration.yaml`, add the name and password to the `mqtt:` block, and restart Zigbee2MQTT. For a hub at home on `192.168.1.10`:

   ```yaml
   mqtt:
     base_topic: zigbee2mqtt
     server: mqtt://192.168.1.10:1883
     user: zigbee2mqtt
     password: GENERATED_PASSWORD
   ```

   Its log shows `Connected to MQTT server` once the broker accepts it. [Connect Zigbee2MQTT to the broker](sensors/zigbee#connect-zigbee2mqtt-to-the-broker) has the `server` for a tunnel and for the public port.

If Zigbee2MQTT runs in Docker on the hub's machine and reached the broker through `host.docker.internal`, that no longer works with the broker on `127.0.0.1`. Give the container `network_mode: host`, point it at `mqtt://localhost:1883` and move its frontend off port 8080 (see [How to connect a Zigbee device](how-to/connect-zigbee-device)).

Give any other device its own client in the same way, with a prefix that covers every topic it uses.

### 7. Enter the SMTP settings

Skip this if the hub did not send you email. Email stays off until the hub has an SMTP password: the Gmail token cannot be carried over.

Open **Alerts & Notifications** and fill in the **Email** card. It starts on `smtp.gmail.com`, port `587`, `starttls`, with your old `smtp.user` as the username and from address. Enter a password, save, and click **Send test email**, which sends to the email address on your own account. To stay on Gmail you need an app password, which gives whole-account access, so use a Google account that only sends the hub's email. [Getting a send-only credential](alerts-and-notifications#getting-a-send-only-credential) covers this and the alternatives. From the CLI, use `sensor-hub email set` and `sensor-hub email test` (see [Email](cli-tool#email)).

### 8. Remove SENSOR_HUB_INITIAL_ADMIN

```bash
sudo grep SENSOR_HUB_INITIAL_ADMIN /etc/sensor-hub/environment
```

If this prints a line that does not start with `#`, delete that line. 2.0 ignores the variable and logs a warning at every start, and the line holds an admin password in plain text. If that admin still uses the password, change it. The first admin of a new install is now created with [`sensor-hub local admin create`](cli-tool#create-the-first-admin).

### 9. Check Prometheus and nginx

**Prometheus**: `/metrics` is no longer on the API port, so `http://<hub>:8080/metrics` and `https://<hub>/metrics` now answer 404. Change the scrape target to `localhost:9464` on the hub's machine, or to the `metrics.listen.address` you set in step 3 (see [Prometheus metrics](telemetry#prometheus-metrics)).

**nginx**: the install updated `/etc/sensor-hub/nginx.conf.example`. A config copied from the 1.5.x example and served on port 443 needs no change. Otherwise, check yours against it:

- `proxy_pass` goes to `http://127.0.0.1:8080`, or to the `http.listen.address` you set in step 3. The hub no longer listens on its LAN address.
- On a port other than 443, use `proxy_set_header Host $http_host;` rather than `$host`, or the hub refuses every browser WebSocket with 403 and the pages stop updating live (see [WebSocket origin](nginx-setup#websocket-origin)).
- nginx passes `X-Forwarded-For`. nginx on another host also needs its address in `http.trusted.proxies` from step 3 (see [Client addresses](nginx-setup#client-addresses)).
- Only if devices are to dial in to the broker over the internet: add the `stream` block for [MQTT over TLS on port 8883](nginx-setup#mqtt-over-tls-on-port-8883). A tunnel needs none of it.

Run `sudo nginx -t` and `sudo systemctl reload nginx` after any change.

### 10. Rotate broker passwords and revoke the Google token

The upgrade removed the outbound broker passwords and the Gmail token from the hub's disk. Scrubbing is not revoking. Any copy of a 1.5.x database, backup or `/etc/sensor-hub`, including the one from step 1, still holds them, and they still work until you change them.

- **Brokers the hub connects out to with a password**: change the password on each broker, then on the **MQTT** page choose **Edit** from the broker's menu and enter the new one. The broker reconnects straight away, with no restart. Existing brokers keep connecting over plain TCP and show as **Unencrypted**: for one reached across a network you do not control, turn on its **TLS** switch at the same time (see [TLS to outbound MQTT brokers](configuration#tls-to-outbound-mqtt-brokers)).
- **Gmail**: in the Google account the hub sent from, remove the hub's access under [third-party apps and services](https://myaccount.google.com/connections). If you made an OAuth client in Google Cloud only for the hub, delete it too.

The hub deletes only `/etc/sensor-hub/credentials.json` and `/etc/sensor-hub/token.json`. If 1.5.x had `oauth.credentials.file.path` or `oauth.token.file.path` naming any other file, wherever it is, the hub leaves that file alone and logs a warning naming it at every start until the properties are next saved. Delete those files by hand.

## What needs no action

- **Configuration files** you changed stay as you left them, and ones you never changed take the new defaults. A property missing from a file takes its built-in default.
- **Outbound brokers** keep connecting as before. Changes to a broker now apply straight away, with no restart.
- **Users, roles, sessions and API keys** carry on working. Roles that had `manage_oauth` now have `manage_email`.
- **Leftovers from 1.5.x**: `smtp.properties` and any `oauth.*` or `smtp.*` keys in `application.properties` are ignored. The hub's next save of the properties drops the keys.
- **`auth.bcrypt.cost`** must now be from 10 to 31. A lower value in the file is used as 10, with a warning at start, and the next save writes 10.
- **The database migration** runs on its own as the hub starts. There is nothing to run by hand.

## Breaking changes

These matter to scripts, API clients and anything else built against 1.5.x.

**CLI**

- `sensor-hub serve` is now `sensor-hub local serve`. Every `local` command defaults `--config-dir` to `/etc/sensor-hub`.
- No command takes a secret as a flag. `--password` is gone from `users create`, `mqtt brokers create` and `auth login`, and `--new-password` from `users change-password`. On a terminal, leave the flag off and the command asks. In a script, pipe the password in with `--password-stdin`:

  ```bash
  printf '%s\n' "$PASSWORD" | sensor-hub auth login --username alice --password-stdin
  ```

- The global `--api-key` flag is gone. Export `SENSOR_HUB_API_KEY`, or keep the key in `~/.sensor-hub.yaml`, which keeps working as it is. A non-empty `SENSOR_HUB_API_KEY` now takes priority over the file. A removed flag fails as an unknown flag, so a script that is not updated stops rather than carrying on without credentials.
- `config init` only prompts on a terminal. A script writes the config file with `printf '%s\n' "$KEY" | sensor-hub config init --server <url> --api-key-stdin`, adding `--insecure` for a self-signed certificate. It exits with an error and writes nothing when the hub cannot be reached or refuses the key.
- `mqtt brokers create --tls` now turns TLS on. In 1.5.x it did nothing.
- The `oauth` command group is gone. `sensor-hub email` manages the SMTP settings.

**API**

- `MQTTBroker`: `password` is write-only, and responses carry `password_status` instead. `client_cert_path`, `client_key_path` and `ca_cert_path` are gone: the upgrade drops their values, and a request that sends them has them ignored. `tls` and `ca_cert_pem` are new. The embedded broker has no `host` or `port`.
- `/api/oauth/*` is gone, replaced by `/api/email/smtp`. The permission `manage_oauth` is renamed `manage_email`.
- `GET /metrics` on the API port answers 404.
- Changing the expiry of, revoking or deleting another user's API key answers 404 without `manage_users`, as does a key id that does not exist. 1.5.x answered 200.
- While a user must change their password, their API keys get 403 on everything except login, logout, `/api/auth/me` and the password change, as their browser session does.
- A disabled user's sessions and API keys get 401, and their login with the right password answers "account disabled". This includes users disabled straight in the 1.5.x database.
- A browser WebSocket whose `Origin` does not match the `Host` header is refused with 403.

**Configuration**

- The `oauth.*` and `smtp.*` properties and `smtp.properties` are gone, along with the `oauth-reload` apply action and the "Email & OAuth" properties group. The email settings live in the database.
- `SENSOR_HUB_INITIAL_ADMIN` is ignored.
- The HTTP API, metrics and the embedded broker listen on loopback by default (`http.listen.address`, `metrics.listen.address`, `mqtt.broker.listen.address`).
- The embedded broker requires every device to log in with an MQTT client, which can use only its own topic prefix. It refuses CONNECTs beyond 20 a second across all clients (`mqtt.broker.connect.rate.limit`).
- Saving properties leaves each file at mode 0640. On a package install they already are. A file not owned by `sensor-hub` makes the save fail until its ownership is put back.

## Going back to 1.5.x

The database 2.0 leaves behind is no use to 1.5.x, so going back means restoring the backup from step 1. Anything the hub recorded after the upgrade is lost.

Install the 1.5.x package:

```bash
# Debian / Ubuntu: dpkg may ask about each configuration file. Either answer
# will do, since the backup replaces them below.
sudo apt install --allow-downgrades ./sensor-hub_1.5.2_*.deb

# Fedora / RHEL
sudo dnf remove sensor-hub
sudo dnf install ./sensor-hub_1.5.2_*.rpm
```

If you changed an `ExecStart=` drop-in to `local serve` in step 2, change it back. Then put the backup in place. This also removes the 2.0 secret-store key and its drop-in, which 1.5.x does not use:

```bash
sudo systemctl stop sensor-hub
sudo rm -f /etc/systemd/system/sensor-hub.service.d/secrets-key.conf
sudo rm -rf /var/lib/sensor-hub /etc/sensor-hub
sudo cp -a /var/backups/sensor-hub-1.5/lib /var/lib/sensor-hub
sudo cp -a /var/backups/sensor-hub-1.5/etc /etc/sensor-hub
sudo systemctl daemon-reload
sudo systemctl start sensor-hub
```

The Gmail token in the backup works again only if you did not revoke it in step 10. Otherwise authorise Gmail again from 1.5.x.

## Not using the package

If you run the binary yourself rather than from the package, the same changes apply, with these differences:

- Start it with `sensor-hub local serve`, and pass `--config-dir` if your configuration is not in `/etc/sensor-hub`.
- No package creates the secret-store key, so the hub generates `<config-dir>/secrets.key` on its first 2.0 start. Save it with `sensor-hub local secrets show-key --config-dir=<config-dir>`. The hub will not use a key inside the directory that holds the database, so if your database is in the configuration directory, create the key outside it with `(umask 077; openssl rand -base64 32 > /path/to/secrets.key)` and pass that path to `local serve` and `show-key` with `--secrets-key-file` (see [Secret-store key](configuration#secret-store-key)).
- The hub deletes `credentials.json` and `token.json` from the configuration directory itself, as it starts.
