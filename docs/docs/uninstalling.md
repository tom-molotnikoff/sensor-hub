---
id: uninstalling
title: Uninstalling
sidebar_position: 5
---

# Uninstalling

## Remove the package

**Fedora / RHEL:**

```bash
sudo systemctl stop sensor-hub
sudo dnf remove sensor-hub
```

**Debian / Ubuntu:**

```bash
sudo systemctl stop sensor-hub
sudo apt remove sensor-hub
```

Removing the package stops the service and removes the binary and the systemd unit. It keeps the configuration files, the secret-store key, the database and the logs.

## Purge (Debian / Ubuntu)

```bash
sudo apt purge sensor-hub
```

Purging also removes the configuration files the package created in `/etc/sensor-hub/`: `environment`, `application.properties` and `database.properties`, and the `smtp.properties` an install upgraded from 1.5.x may still have. It keeps the database in `/var/lib/sensor-hub/` and the logs, and so it keeps the secret-store key too (`/etc/sensor-hub/secrets.key`, or `/etc/sensor-hub/secrets.key.cred` and its drop-in `/etc/systemd/system/sensor-hub.service.d/secrets-key.conf`), because a database without its key has lost every stored secret. RPM has no purge, so on Fedora and RHEL `dnf remove` keeps everything.

## Full cleanup (optional)

To remove all data, configuration, the secret-store key and logs:

```bash
sudo rm -rf /var/lib/sensor-hub /var/log/sensor-hub /etc/sensor-hub /etc/systemd/system/sensor-hub.service.d
sudo systemctl daemon-reload
```

To remove the system user and group:

```bash
sudo userdel sensor-hub
sudo groupdel sensor-hub
```
