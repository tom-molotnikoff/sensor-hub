---
id: installation
title: Installation
sidebar_position: 3
---

# Installation

## Download the package

Download the latest RPM or DEB package from the [GitHub Releases](https://github.com/tom-molotnikoff/sensor-hub/releases) page. Packages are GPG-signed.

## Install

**Fedora / RHEL:**

```bash
sudo dnf install ./sensor-hub-*.rpm
```

**Debian / Ubuntu:**

```bash
sudo apt install ./sensor-hub_*.deb
```

The package:

- Installs the binary to `/usr/bin/sensor-hub`
- Creates a `sensor-hub` system user and group
- Creates configuration directory `/etc/sensor-hub/` with default files
- Creates data directory `/var/lib/sensor-hub/`
- Creates log directory `/var/log/sensor-hub/`
- Installs and enables the `sensor-hub.service` systemd unit
- Installs logrotate configuration (daily rotation + 50 MB max, 14 days retention)

## Configure

Edit the configuration files in `/etc/sensor-hub/`:

### application.properties

Review and adjust the defaults. See [Configuration Settings](configuration) for a description of each property.

### smtp.properties

Set the Gmail address used as the sender for email notifications (only required if you plan to enable email alerts):

```properties
smtp.user=your-email@gmail.com
```

## Set the initial admin user

Edit `/etc/sensor-hub/environment` and uncomment the admin line:

```bash
SENSOR_HUB_INITIAL_ADMIN=admin:yourpassword
```

This creates an admin user with full permissions on first startup, only if no users exist in the database. Comment it out or remove the value after the first start.

## Start the service

```bash
sudo systemctl start sensor-hub
```

On first start:

1. Embedded migrations create the SQLite database and schema automatically
2. The binary starts serving the API and embedded React UI on port 8080

## Set up nginx

Nginx provides TLS termination in front of sensor-hub. Follow the [nginx setup guide](nginx-setup) to configure it.

## Verify the installation

```bash
curl -k https://localhost/api/health
```

Expected response:

```json
{"status": "ok"}
```

Open the web UI at `https://<host>/` and log in with the admin credentials you configured.

## Set up OAuth for email notifications (optional)

Email notifications require Gmail OAuth 2.0 authorization.

After deployment, navigate to the Alerts and Notifications page and use the OAuth Configuration card to authorize Gmail access. This requires the `manage_oauth` permission.
