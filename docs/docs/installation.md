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
- Creates configuration directory `/etc/sensor-hub/` with default files, copied from `/usr/share/sensor-hub/defaults/`. Upgrades never change them (see [Configuration files](upgrading#configuration-files))
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

### If the TPM will not unseal the key

systemd will not start a service whose credential it cannot decrypt, so before each start of the service the package runs `sensor-hub-key-check.service`, which checks that the sealed key still unseals. It logs what it finds and does to the journal (`journalctl -u sensor-hub-key-check`). It leaves the key alone while the service is running.

- **The TPM works but refuses the key**, as it can once the firmware or boot path changes: the check keeps the old key as `/etc/sensor-hub/secrets.key.cred.unsealable-<time>` and seals a new key with the TPM in its place. The hub starts with the new key, and every secret stored under the old one needs re-entry (see [Secrets that need re-entry](configuration#secrets-that-need-re-entry)).
- **The TPM cannot be used at all**, such as when it is late at boot or has gone: the check leaves the key alone, since the TPM may still appear. The service fails on its credential and restarts every few seconds, running the check again each time. If the TPM is still unusable five minutes after the first failure this boot, the check sets the key aside as above and seals a new key with the host's own credential secret (`/var/lib/systemd/credential.secret`) instead. The hub starts, and its "Stored secrets need re-entry" notification says the key is now protected by the host's credential secret, not the TPM. A copy of the disk then carries what opens the key, so seal it with the TPM again once the TPM works, using the steps below with the current key from `show-key`.

Either way, save the new key with `show-key` as above.

Rather than entering the secrets again, you can put back a key you saved, or seal the current key with the TPM again. Stop the service, remove the sealed key and its drop-in, seal the key, and start the service:

```bash
sudo systemctl stop sensor-hub
sudo rm /etc/sensor-hub/secrets.key.cred /etc/systemd/system/sensor-hub.service.d/secrets-key.conf
printf '%s\n' "$SAVED_KEY" | sudo sensor-hub local secrets init-key --from-stdin --seal
sudo systemctl daemon-reload
sudo systemctl start sensor-hub
```

If the TPM unseals the old key again later, for example once a firmware change is rolled back, you can move it back instead. Check that it unseals, then put it in place of the new one:

```bash
sudo systemd-creds decrypt --name=secrets.key /etc/sensor-hub/secrets.key.cred.unsealable-<time> - >/dev/null && echo "it unseals"
sudo systemctl stop sensor-hub
sudo mv /etc/sensor-hub/secrets.key.cred.unsealable-<time> /etc/sensor-hub/secrets.key.cred
sudo systemctl start sensor-hub
```

Either way, any secret entered again under the new key in the meantime then needs entering once more.

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
