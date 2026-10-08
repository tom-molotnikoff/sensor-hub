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
- Creates the secret-store key, unless there already is one (see below)
- Installs logrotate configuration (daily rotation + 50 MB max, 14 days retention)

## The secret-store key

The hub keeps the credentials it presents to other systems, such as outbound MQTT broker passwords, encrypted in its database under a key that lives outside it. The package creates that key on install or upgrade, and where it ends up depends on the host:

- **A host with a TPM**, which is when `systemd-creds has-tpm2` reports one: the key is sealed to the TPM in `/etc/sensor-hub/secrets.key.cred`, owned by root with mode 0600, and the drop-in `/etc/systemd/system/sensor-hub.service.d/secrets-key.conf` has systemd decrypt it for the service each time it starts. The packaged unit file is not changed. A copy of the disk carries nothing that opens the key, because only this machine's TPM can.
- **A host without a TPM**, or without `systemd-creds` (systemd older than 250): the key is written to `/etc/sensor-hub/secrets.key`, owned by `sensor-hub` with mode 0600.

If sealing fails on a host with a TPM, the install says why and writes the key file instead, so the service still starts. A key that already exists in any form, including a drop-in that passes the service one, is left as it is, so upgrading or reinstalling never replaces it.

Once the package is installed, print the key and save it somewhere other than the hub, such as a password manager:

```bash
sudo sensor-hub local secrets show-key
```

You need it to restore a backup on another machine, or if the TPM refuses to unseal the key after a firmware or boot change. Without it the stored secrets cannot be decrypted and have to be entered again. To seal the key yourself or bring one you already have, see [The secret-store key](cli-tool#the-secret-store-key) in the CLI reference, and [Secret-store key](configuration#secret-store-key) for where the hub looks for it.

## Configure

Edit the configuration files in `/etc/sensor-hub/`:

### application.properties

Review and adjust the defaults. See [Configuration Settings](configuration) for a description of each property.

### smtp.properties

Set the Gmail address used as the sender for email notifications (only required if you plan to enable email alerts):

```properties
smtp.user=your-email@gmail.com
```

## Start the service

```bash
sudo systemctl start sensor-hub
```

On first start:

1. Embedded migrations create the SQLite database and schema automatically
2. The binary starts serving the API and embedded React UI on port 8080

## Create the first admin user

```bash
sudo sensor-hub local admin create admin
```

It asks for the password twice and writes the admin straight to the database, so the password never lands in a file or the environment. To script it, pipe the password in on stdin instead. The command refuses once an admin exists. See [Local commands](cli-tool#create-the-first-admin) for its options.

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
