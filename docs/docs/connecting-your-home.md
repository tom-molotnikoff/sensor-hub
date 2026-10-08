---
id: connecting-your-home
title: Connecting your home
sidebar_position: 3.6
---

# Connecting your home

Devices at home, such as [Zigbee2MQTT](sensors/zigbee) or a Mosquitto bridge, send their readings to Sensor Hub by connecting to its embedded MQTT broker. The hub never connects into your home network: the home side always dials in. This page covers the three ways it can reach the broker:

| Where the hub runs | How home connects | Broker listen address |
|---|---|---|
| Cloud VM or other remote host | [Through a tunnel](#through-a-tunnel-preferred) (preferred) | The tunnel's end on the hub, or `127.0.0.1` for an SSH forward |
| Cloud VM or other remote host | [Through a public port](#through-a-public-port) on nginx | `127.0.0.1` (default) |
| At home, on the same network | [Over the LAN](#over-the-lan-hub-at-home) | `0.0.0.0` or the hub's LAN address |

Whichever way you choose, the broker only accepts devices that present a credential you created, and each credential can only publish and subscribe under its own topic prefix.

## Create a credential for the device

Every device that connects needs an MQTT client, created by an admin. Create one per device, so a leaked credential reaches one topic tree and can be disabled or rotated on its own.

**Web UI:** open the **MQTT** page and click **Add Client** on the **MQTT Clients** card. Give it a name, which is the MQTT username, and a topic prefix ending in `/`. For Zigbee2MQTT, use the prefix `zigbee2mqtt/`, matching its `base_topic`.

**CLI:**

```bash
sensor-hub mqtt clients create --name zigbee2mqtt --topic-prefix zigbee2mqtt/
```

The generated password is shown once. Put it in the device's MQTT configuration straight away. If you lose it, rotate the password (**Rotate password** in the client's menu, or `sensor-hub mqtt clients rotate-password <id>`) and update the device.

## Through a tunnel (preferred)

A tunnel between home and the hub carries MQTT inside an encrypted, authenticated link. The broker is never exposed to the internet, so nobody without access to the tunnel can reach it, and nothing on the hub needs to answer MQTT from strangers. Use this whenever you can.

Running the tunnel is up to you. Sensor Hub does not run one itself. Two common setups:

### WireGuard or another VPN

The hub gets an address on the tunnel, such as `10.8.0.1`. Make the broker listen on that address only, in `/etc/sensor-hub/application.properties`:

```properties
mqtt.broker.listen.address=10.8.0.1
```

The broker cannot bind an address that does not exist yet, so the tunnel must be up before Sensor Hub starts. For WireGuard managed by `wg-quick`, run `sudo systemctl edit sensor-hub` and add:

```ini
[Unit]
Wants=wg-quick@wg0.service
After=wg-quick@wg0.service
```

Then restart: `sudo systemctl restart sensor-hub`. The home device connects to `mqtt://10.8.0.1:1883`.

### SSH port forward

If the home machine can SSH to the hub, it can forward a local port to the broker. The broker stays on its default address, `127.0.0.1`, with nothing to change on the hub. On the home machine:

```bash
ssh -N -o ServerAliveInterval=30 -o ExitOnForwardFailure=yes \
  -L 127.0.0.1:1883:127.0.0.1:1883 tunnel-user@hub.example.com
```

Run it under systemd or `autossh` so it reconnects after a drop. The home device then connects to `mqtt://localhost:1883` on the home machine. A device in a Docker container must share the host's network (`network_mode: host`) to reach that forward.

## Through a public port

When a tunnel is not an option, nginx can accept MQTT over TLS on port `8883` and pass it to the broker on `127.0.0.1:1883`. The broker keeps its default listen address, and only nginx is reachable from the internet. See [Nginx Setup](nginx-setup#mqtt-over-tls-on-port-8883) for the `stream` block, the limits it sets on each client address, and the nginx module it needs. Open port `8883` in your firewall and cloud security group.

The home device connects to `mqtts://hub.example.com:8883`, using the same certificate and host name as the web UI.

:::warning[Accepted risk]
With the public port open, anyone on the internet can open a connection to the broker. TLS and nginx's limits sit in front of it, but every part of the broker that runs before a CONNECT is authenticated, including the MQTT packet parser and the CONNECT handling in the [mochi-mqtt](https://github.com/mochi-mqtt/server) library, is reachable by an unauthenticated stranger on `8883`. A flaw in any of them would be exposed. This is the cost of the public port, and the reason a tunnel is preferred. The broker sheds CONNECT floods with `mqtt.broker.connect.rate.limit` (see [Configuration](configuration#broker-connect-rate-limit)).
:::

## Over the LAN (hub at home)

When Sensor Hub runs at home, on the same network as your devices, the devices can connect to the broker directly. Make the broker listen on the network, in `/etc/sensor-hub/application.properties`:

```properties
mqtt.broker.listen.address=0.0.0.0
```

Or use the hub's LAN address, such as `192.168.1.10`, to listen on that interface only. Restart with `sudo systemctl restart sensor-hub`. The device connects to `mqtt://192.168.1.10:1883`.

:::warning[Plaintext on the LAN]
MQTT on the home LAN is plaintext: the broker has no TLS listener. Anything that can see traffic on your network, such as a compromised device on the same Wi-Fi, can read the readings and the device's username and password as it connects. What protects the hub is that the credential is scoped to its topic prefix: a stolen Zigbee2MQTT password can publish and subscribe under `zigbee2mqtt/` and nothing else. Never forward port `1883` from your router to the hub. If you suspect a credential has leaked, rotate its password.
:::

If the device runs on the hub's own machine, it can connect to `mqtt://localhost:1883` with the default listen address. A device in a Docker container on that machine must share the host's network (`network_mode: host`), because the default address does not take connections from Docker's bridge network.

## Configure the device

For Zigbee2MQTT, see the [`mqtt:` block for each case](sensors/zigbee#connect-zigbee2mqtt-to-the-broker). Any other MQTT client needs the same three things: the server address from the case above, the username, which is the client's name, and the password generated for it.
