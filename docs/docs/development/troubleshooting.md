# Troubleshooting

Common issues and how to investigate them.

## Checking Service Health

### Health Endpoint

```bash
curl http://localhost:8080/api/health
```

Returns 200 OK if the server is running.

### systemd Service Status

```bash
systemctl status sensor-hub
journalctl -u sensor-hub -f        # Follow logs
journalctl -u sensor-hub --since "1 hour ago"
```

### Log File

Production logs are at `/var/log/sensor-hub/sensor-hub.log`:

```bash
tail -f /var/log/sensor-hub/sensor-hub.log
```

### Debug Logging

Temporarily enable debug logging by setting `log.level` to `debug` on the
properties page, or by updating it in
`/etc/sensor-hub/application.properties`:

```
log.level=debug
```

The running process picks the new level up without a restart, either
immediately on save or within a couple of seconds of an external file edit.

Debug logging produces detailed output for every database operation, HTTP
request, WebSocket event, and periodic task execution.

## Common Issues

### Sensor Collection Not Happening

**Symptoms:** No new readings appearing, periodic cleanup runs but sensor
collection does not.

**Check:**
1. Verify sensors are registered and enabled:
   ```bash
   sensor-hub sensors list
   ```
2. Check that the sensor URLs are reachable from the hub:
   ```bash
   curl http://<sensor-ip>:<port>/temperature
   ```
3. Enable debug logging and look for `periodic task started` messages with
   `task=sensor_collection`
4. Check for `periodic task panicked` — if the collection goroutine panics, it
   will restart with backoff. Consecutive panics will be logged with stack traces

The periodic task supervisor logs at ERROR level when a task panics and when it
restarts after backoff. Check for these patterns:

```
periodic task panicked  task=sensor_collection  panic=...  stack=...
restarting periodic task after backoff  task=sensor_collection  backoff=10s
```

### Cannot Save Configuration (Read-Only Filesystem)

**Symptoms:** `PATCH /api/properties/` returns 202 but logs show
`open /etc/sensor-hub/application.properties: read-only file system`.

**Cause:** The systemd unit uses `ProtectSystem=strict`. The fix is already
applied — `/etc/sensor-hub` is in `ReadWritePaths`. If you see this on an older
installation, check that the service file includes:

```ini
ReadWritePaths=/var/log/sensor-hub /etc/sensor-hub
```

Then reload and restart:

```bash
systemctl daemon-reload
systemctl restart sensor-hub
```

### Sensor Not Found (Case Sensitivity)

**Symptoms:** API returns empty results or 404 when querying a sensor by name,
even though it exists.

All sensor name lookups are case-insensitive. If you encounter this with a
custom query or external tool accessing the database directly, ensure names
match exactly.

### Time Values One Hour Off

**Symptoms:** Readings appear shifted by one hour in the UI.

**Possible causes:**
- The Go application may be using UTC while the system clock is in a local
  timezone (or vice versa). Check with `date` on the host and compare with
  readings in the database
- During BST/DST transitions, timestamps stored without timezone information
  can appear shifted

### Database Locked

**Symptoms:** `database is locked` errors in logs.

**Cause:** SQLite allows only one writer at a time. This should not occur
normally. If it does, check for:
- Multiple instances of sensor-hub running
- External tools accessing the database file directly

### Migrations Failed (Dirty State)

**Symptoms:** Application refuses to start, logs show `dirty database version`.

**Cause:** A previous migration was interrupted (crash, power loss).

**Fix:** Manually inspect the `schema_migrations` table in the SQLite database,
correct the version and dirty flag, then restart.

```bash
sqlite3 /path/to/sensor_hub.db
> SELECT * FROM schema_migrations;
> UPDATE schema_migrations SET dirty = 0;
```

### WebSocket Not Connecting

**Symptoms:** Real-time updates not appearing in the UI.

**Check:**
1. Browser developer tools → Network → WS tab for WebSocket connections
2. Verify the WebSocket URL matches the server (check `VITE_WEBSOCKET_BASE` in
   development)
3. Check that the user has the required permissions (`view_readings` for
   temperature, `view_notifications` for notifications)
4. If behind Nginx, ensure WebSocket upgrade headers are proxied:
   ```nginx
   proxy_set_header Upgrade $http_upgrade;
   proxy_set_header Connection "upgrade";
   ```

### Login Returns 429 / Authentication Hangs

**Symptoms:** Login fails even with correct credentials.

**Cause:** Too many failed login attempts triggered the rate limiter.

**Check:** The backoff is time-based and clears automatically after the
configured window (default 15 minutes). You can also clear the
`failed_login_summary` table directly:

```sql
DELETE FROM failed_login_summary WHERE identifier = '<username_or_ip>';
```

## CLI Troubleshooting

### Connection Refused

```bash
sensor-hub --server http://localhost:8080 health
```

If this fails, the server is not running or the URL is wrong. Check
`systemctl status sensor-hub` and verify the port.

### Certificate Errors

When connecting to a server with a self-signed certificate:

```bash
sensor-hub --insecure sensors list
```

The `--insecure` flag skips TLS certificate verification.

### API Key Not Working

- Check the key has not been revoked or expired
- Verify the key has the correct permissions