---
id: nginx-setup
title: Nginx Setup
sidebar_position: 3.5
---

# Nginx Setup

Nginx provides TLS termination in front of sensor-hub. It is **not** included in the sensor-hub package and must be installed separately.

## Install nginx

**Fedora / RHEL:**

```bash
sudo dnf install nginx
```

**Debian / Ubuntu:**

```bash
sudo apt install nginx
```

## Configure

Copy the example configuration shipped with the package:

```bash
sudo cp /etc/sensor-hub/nginx.conf.example /etc/nginx/conf.d/sensor-hub.conf
```

Edit `/etc/nginx/conf.d/sensor-hub.conf` and set the paths to your TLS certificate and key:

```nginx
ssl_certificate     /path/to/sensor-hub.pem;
ssl_certificate_key /path/to/sensor-hub-key.pem;
```

The example configuration proxies all requests from port 443 to `http://127.0.0.1:8080` with WebSocket upgrade support, except `/metrics`, which it does not proxy.

## What sensor-hub expects of nginx

Three properties in `/etc/sensor-hub/application.properties` decide how nginx and sensor-hub fit together. See [Configuration](configuration) for each.

| Property | Packaged value | Why |
|---|---|---|
| `http.listen.address` | `127.0.0.1:8080` (default) | Only nginx, on the same machine, reaches the hub. Everything else goes through nginx |
| `http.trusted.proxies` | `127.0.0.1,::1` | The hub believes the client address nginx forwards, so the login backoff applies to each client, not to nginx |
| `metrics.listen.address` | `127.0.0.1:9464` (default) | `/metrics` has no authentication, so it has its own listener and nginx never serves it. See [Telemetry](telemetry#prometheus-metrics) |

### Client addresses

nginx passes the client's address on in `X-Forwarded-For` and `X-Real-IP`. Sensor Hub believes those headers only when the request comes from an address in `http.trusted.proxies`, so nginx's own address must be in that list. The packaged value covers nginx on the same machine.

If `http.trusted.proxies` is empty, every request appears to come from nginx. One person failing to log in then puts every user under the login backoff.

If nginx runs on another host:

1. Set `http.listen.address` to an address nginx can reach, such as `0.0.0.0:8080`, and change `proxy_pass` to match.
2. Set `http.trusted.proxies` to nginx's address, such as `192.168.1.20`.
3. Restart sensor-hub: `sudo systemctl restart sensor-hub`.

Never list an address that untrusted clients can connect from: a client sending from it could claim any address it liked.

### WebSocket origin

Sensor Hub refuses a browser WebSocket whose `Origin` host and port differ from the request's `Host`. The example sets `proxy_set_header Host $http_host;`, which passes on the host and port the browser used, so this holds on any port nginx listens on. `$host` drops the port, so with nginx on a port other than 443 every WebSocket would be refused.

## TLS certificates

### Self-signed with mkcert

```bash
mkcert -install
mkcert sensor-hub
```

This creates `sensor-hub.pem` and `sensor-hub-key.pem` in the current directory.

### Let's Encrypt with certbot

```bash
sudo dnf install certbot python3-certbot-nginx   # Fedora / RHEL
sudo apt install certbot python3-certbot-nginx    # Debian / Ubuntu

sudo certbot --nginx -d sensor-hub.example.com
```

Certbot configures nginx and sets up automatic certificate renewal.

## Test and enable

```bash
sudo nginx -t
sudo systemctl enable --now nginx
```

## Verify

```bash
curl -k https://localhost/api/health
```

Expected response:

```json
{"status": "ok"}
```
