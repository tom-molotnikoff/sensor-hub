---
id: upgrading
title: Upgrading
sidebar_position: 4
---

# Upgrading

:::info[Upgrading from 1.5.x]
Follow [Upgrading to 2.0](upgrading-to-2-0) instead. 2.0 needs a few steps of its own, and its backup step differs from the one below.
:::

## Back up the database

Take a copy of the database while the hub runs, and keep a copy of the configuration files with it:

```bash
sudo install -d -m 0700 /var/backups/sensor-hub/etc-$(date +%F)
sudo sensor-hub local db backup /var/backups/sensor-hub/sensor_hub-$(date +%F).db
sudo cp -a /etc/sensor-hub/*.properties /etc/sensor-hub/environment /var/backups/sensor-hub/etc-$(date +%F)/
```

`local db backup` writes one consistent file that includes the writes still in the database's `-wal` file. A plain copy of `sensor_hub.db` while the hub runs leaves those out, and can catch the file halfway through a write. The backup holds the stored secrets encrypted, and not the key that decrypts them. Keep the key apart from it, as the output of `sudo sensor-hub local secrets show-key` in a password manager, so the backup alone gives nothing away. See [Back up the database](cli-tool#back-up-the-database).

## Download and install the new package

Download the latest package from the [GitHub Releases](https://github.com/tom-molotnikoff/sensor-hub/releases) page.

**Fedora / RHEL:**

```bash
sudo dnf upgrade ./sensor-hub_*.rpm
```

**Debian / Ubuntu:**

```bash
sudo apt install ./sensor-hub_*.deb
```

The package restarts `sensor-hub.service` automatically.

## Database migrations

Embedded migrations run automatically on startup. The migrate library tracks which migrations have been applied in a `schema_migrations` table and only runs new ones. Migrations are embedded into the binary at build time using `//go:embed`, so no external migration tool is needed.

Migrations are forward-only. There is no automated rollback mechanism, which is why backing up the database before upgrading is recommended.

## Changes that delete data

Some releases change what the hub deletes. Read these before upgrading past the release that brings them.

- **Automations:** deleting a sensor now also deletes every automation that uses it, including any run in progress. See [Deleting a sensor](automations#deleting-a-sensor).
- **Command history:** command history is now deleted after `command.history.retention.days`, 90 days by default, including commands sent by people. The first cleanup runs as the upgraded hub starts and deletes every command sent more than 90 days ago. To keep all of it, add `command.history.retention.days=0` to `application.properties` before upgrading. The release you are upgrading from does not know this property, so its Properties page does not show it, and saving any property there rewrites `application.properties` without this line. Add it after your last change on that page. See [Command history retention](configuration#command-history-retention).

## Configuration files

The configuration files in `/etc/sensor-hub/` are yours, not the package's. The package ships their defaults as templates in `/usr/share/sensor-hub/defaults/` and copies one to `/etc/sensor-hub/` only when it is missing, so an upgrade never changes a file you have and never stops to ask about one. A property missing from your `application.properties` takes its built-in default.

Up to 1.5.x the package owned these files. The upgrade from 1.5.x keeps every file you or the hub changed as it is, and gives a file nobody changed the new default, as the package manager used to. It asks nothing, so it also runs unattended.

Review release notes for any new properties and refer to [Configuration Settings](configuration) for the full property reference.

## Verify

```bash
sudo systemctl status sensor-hub
curl -k https://localhost/api/health
```

## Clean up old backups

Once you are happy with the new release, delete the backups you no longer need from `/var/backups/sensor-hub/`.

## Rollback (if needed)

If you encounter issues after upgrading, you can downgrade back to the previous version and restore the database from the backup you created:

**Fedora / RHEL:**

```bash
sudo dnf remove sensor-hub
sudo dnf install ./sensor-hub_previous-version.rpm
```

**Debian / Ubuntu:**

```bash
sudo apt install --allow-downgrades ./sensor-hub_previous-version.deb
```

Then restore the database, removing the newer database's `-wal` and `-shm` files with it:

```bash
# Stop the service after installing the old version
sudo systemctl stop sensor-hub
# Restore the database from the backup
sudo rm -f /var/lib/sensor-hub/sensor_hub.db /var/lib/sensor-hub/sensor_hub.db-wal /var/lib/sensor-hub/sensor_hub.db-shm
sudo install -m 0600 -o sensor-hub -g sensor-hub /var/backups/sensor-hub/sensor_hub-<date>.db /var/lib/sensor-hub/sensor_hub.db
# Start the service
sudo systemctl start sensor-hub
```

To go back from 2.0 to 1.5.x, see [Going back to 1.5.x](upgrading-to-2-0#going-back-to-15x).
