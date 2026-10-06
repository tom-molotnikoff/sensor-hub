---
id: upgrading
title: Upgrading
sidebar_position: 4
---

# Upgrading

## Back up the database

```bash
sudo cp /var/lib/sensor-hub/sensor_hub.db ~/sensor-hub-backup.db
```

## Download and install the new package

Download the latest package from the [GitHub Releases](https://github.com/tom-molotnikoff/sensor-hub/releases) page.

**Fedora / RHEL:**

```bash
sudo dnf upgrade ./sensor-hub-*.rpm
```

**Debian / Ubuntu:**

```bash
sudo apt install ./sensor-hub_*.deb
```

The postinstall scriptlet restarts the `sensor-hub.service` automatically.

## Database migrations

Embedded migrations run automatically on startup. The migrate library tracks which migrations have been applied in a `schema_migrations` table and only runs new ones. Migrations are embedded into the binary at build time using `//go:embed`, so no external migration tool is needed.

Migrations are forward-only. There is no automated rollback mechanism, which is why backing up the database before upgrading is recommended.

## Configuration files

Configuration files in `/etc/sensor-hub/` are marked as `noreplace` (RPM) or `conffiles` (DEB). Your edits are preserved during upgrades:

- **RPM:** If the package ships a new default, it is saved as `.rpmnew` alongside your existing file.
- **DEB:** If the package ships a new default, it is saved as `.dpkg-new` alongside your existing file.

Review release notes for any new properties and refer to [Configuration Settings](configuration) for the full property reference.

## Behaviour changes after 1.5.2

Every reading now goes through the same steps, whether it was collected on the timer, collected on demand or received over MQTT. Two things behave differently as a result:

- **The periodic collector marks a sensor Bad when its readings can't be stored.** Before, a storage failure during periodic collection was only logged and the sensor kept its old health. It now gets health Bad with the reason `error storing readings: ...`, as on-demand collection already did. MQTT sensors used the reason `storage error: ...` for the same failure and now use this one too.
- **MQTT readings of unknown measurement types no longer reach alerts or the WebSocket.** They were never stored, but they were still checked against alert rules and broadcast to open browsers. Now they are dropped with a warning in the log before either sees them.

MQTT sensors also record `successful reading` as their health reason after a stored reading, where they used to record `MQTT reading received`.

## Verify

```bash
sudo systemctl status sensor-hub
curl -k https://localhost/api/health
```

## Clean up old database backup

```bash
rm ~/sensor-hub-backup.db
```

## Rollback (if needed)

If you encounter issues after upgrading, you can downgrade back to the previous version and restore the database from the backup you created:

**Fedora / RHEL:**

```bash
sudo dnf remove sensor-hub
sudo dnf install ./sensor-hub-previous-version.rpm
```

**Debian / Ubuntu:**

```bash
sudo apt install --allow-downgrades ./sensor-hub_previous-version.deb
```

Then restore the database:

```bash
# Stop the service after installing the old version
sudo systemctl stop sensor-hub
# Restore the database from the backup
sudo rm /var/lib/sensor-hub/sensor_hub.db
sudo cp ~/sensor-hub-backup.db /var/lib/sensor-hub/sensor_hub.db
# Start the service
sudo systemctl start sensor-hub
```
