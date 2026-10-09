---
id: prerequisites
title: Prerequisites
sidebar_position: 2
---

# Prerequisites

Before installing Sensor Hub, ensure your environment meets the following requirements.

## Host system

Supported operating systems:

- Fedora (latest stable)
- RHEL / CentOS Stream 9+
- Debian 12+
- Ubuntu 22.04+
- Raspberry Pi OS (arm64)

Additional requirements:

- nginx (for TLS termination)
- Sufficient disk space for the SQLite database (depends on the number of sensors and data retention settings)

## Network

- Port **443** — nginx (HTTPS, public-facing)
- Port **8080** — sensor-hub (localhost only, proxied by nginx)

## TLS certificates

You need TLS certificates in PEM format for nginx:

- A certificate file (e.g., `sensor-hub.pem`)
- A private key file (e.g., `sensor-hub-key.pem`)

For local development, [mkcert](https://github.com/FiloSottile/mkcert) can generate locally-trusted certificates. For production, use [Let's Encrypt](https://letsencrypt.org/) with certbot.

## Email notifications (optional)

To send alert notifications by email, you need an SMTP server the hub can reach and a username and password for it, ideally a credential that can only send, such as Amazon SES SMTP credentials or a transactional email provider's. See [Getting a send-only credential](alerts-and-notifications#getting-a-send-only-credential).
