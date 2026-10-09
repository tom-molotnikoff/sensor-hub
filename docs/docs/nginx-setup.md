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

The example configuration proxies all requests from port 443 to `http://127.0.0.1:8080` with WebSocket upgrade support, except `/metrics`, which it does not proxy. Its end carries an optional, commented-out `stream` block for [MQTT over TLS on port 8883](#mqtt-over-tls-on-port-8883).

## What sensor-hub expects of nginx

Three properties decide how nginx and sensor-hub fit together. Their defaults suit nginx on the same machine, so none of them needs setting there. See [Configuration](configuration) for each.

| Property | Default | Why |
|---|---|---|
| `http.listen.address` | `127.0.0.1:8080` | Only nginx, on the same machine, reaches the hub. Everything else goes through nginx |
| `http.trusted.proxies` | `127.0.0.1,::1` | The hub believes the client address nginx forwards, so the login backoff applies to each client, not to nginx |
| `metrics.listen.address` | `127.0.0.1:9464` | `/metrics` has no authentication, so it has its own listener and nginx never serves it. See [Telemetry](telemetry#prometheus-metrics) |

### Client addresses

nginx passes the client's address on in `X-Forwarded-For` and `X-Real-IP`. Sensor Hub believes those headers only when the request comes from an address in `http.trusted.proxies`, so nginx's own address must be in that list. The default, `127.0.0.1,::1`, covers nginx on the same machine.

If `http.trusted.proxies` is empty, every request appears to come from nginx. One person failing to log in then puts every user under the login backoff.

If nginx runs on another host:

1. Set `http.listen.address` to an address nginx can reach, such as `0.0.0.0:8080`, and change `proxy_pass` to match.
2. Set `http.trusted.proxies` to nginx's address, such as `192.168.1.20`.
3. Restart sensor-hub: `sudo systemctl restart sensor-hub`.

Never list an address that untrusted clients can connect from: a client sending from it could claim any address it liked.

### WebSocket origin

Sensor Hub refuses a browser WebSocket whose `Origin` host and port differ from the request's `Host`. The example sets `proxy_set_header Host $http_host;`, which passes on the host and port the browser used, so this holds on any port nginx listens on. `$host` drops the port, so with nginx on a port other than 443 every WebSocket would be refused.

## MQTT over TLS on port 8883

This section is only for the public-port way of [connecting your home](connecting-your-home). A tunnel is preferred, and needs none of it.

With the public port, nginx accepts MQTT over TLS on port `8883`, using the same certificate as HTTPS, and passes the connection to the embedded broker on `127.0.0.1:1883`. The broker keeps its default listen address, so only nginx is reachable from outside.

### Install the stream module

nginx forwards MQTT with its `stream` module. On most distributions that module is a separate package, not part of `nginx`:

**Debian / Ubuntu:**

```bash
sudo apt install libnginx-mod-stream
```

**Fedora / RHEL:**

```bash
sudo dnf install nginx-mod-stream
```

### Add the stream block

A `stream` block must sit at the top level of nginx's configuration, outside `http`. The example file is included inside `http`, so it carries the block commented out at its end. Copy the block, without the leading `# `, into a file of its own, such as `/etc/nginx/sensor-hub-stream.conf`:

```nginx
stream {
    # At most 5 open connections from one client address at a time.
    limit_conn_zone $binary_remote_addr zone=sensor_hub_mqtt_conn:10m;

    server {
        listen 8883 ssl;

        # The same certificate as HTTPS.
        ssl_certificate /etc/ssl/certs/home.sensor-hub.pem;
        ssl_certificate_key /etc/ssl/private/home.sensor-hub-key.pem;
        ssl_protocols TLSv1.2 TLSv1.3;

        limit_conn sensor_hub_mqtt_conn 5;

        proxy_pass 127.0.0.1:1883;
        proxy_connect_timeout 5s;
        proxy_timeout 10m;
    }
}
```

Set the certificate paths to the ones your HTTPS `server` block uses. Then include the file at the top level of `/etc/nginx/nginx.conf`, next to the `events` and `http` blocks rather than inside either:

```nginx
include /etc/nginx/sensor-hub-stream.conf;
```

Check the configuration and reload:

```bash
sudo nginx -t
sudo systemctl reload nginx
```

`unknown directive "stream"` from `nginx -t` means the stream module is not installed. Open port `8883` in the host's firewall and in any cloud security group.

### Limits

| Setting | Default | What it bounds |
|---|---|---|
| `limit_conn` in the stream block | `5` | Open connections from one client address at a time. A home normally needs one per device |
| `mqtt.broker.connect.rate.limit` on the hub | `20` per second | New CONNECTs from all clients together, refused before their credentials are checked. See [Configuration](configuration#broker-connect-rate-limit) |

nginx has no request-rate limit for `stream` connections (`limit_req` works for HTTP only), so the rate of new connections is bounded by the hub's CONNECT rate limit rather than by nginx.

:::warning[Accepted risk]
With port `8883` open, anyone on the internet can open a connection to the broker. Every part of the broker that runs before a CONNECT is authenticated, including the MQTT packet parser and the CONNECT handling in the [mochi-mqtt](https://github.com/mochi-mqtt/server) library, is reachable by an unauthenticated stranger on that port. A flaw in any of them would be exposed. This risk is accepted for the public port, and is the reason a tunnel is preferred.
:::

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
