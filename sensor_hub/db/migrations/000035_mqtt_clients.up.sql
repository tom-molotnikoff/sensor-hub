-- Devices dial in to the embedded broker with a credential an admin created
-- for them, limited to one topic prefix. Only the password's hash is kept.
CREATE TABLE mqtt_clients (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    topic_prefix TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    last_connected_at TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

-- The hub reaches its embedded broker in-process, so that row has no host or
-- port. SQLite cannot drop NOT NULL in place, so the table is rebuilt. Foreign
-- keys are off for the rebuild: dropping the old table would otherwise cascade
-- and delete every subscription.
PRAGMA foreign_keys = OFF;

BEGIN;

CREATE TABLE mqtt_brokers_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL CHECK (type IN ('embedded', 'external')) DEFAULT 'external',
    host TEXT,
    port INTEGER,
    username TEXT DEFAULT NULL,
    password TEXT DEFAULT NULL,
    client_id TEXT DEFAULT NULL,
    ca_cert_path TEXT DEFAULT NULL,
    client_cert_path TEXT DEFAULT NULL,
    client_key_path TEXT DEFAULT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    CHECK (type = 'embedded' OR (host IS NOT NULL AND port IS NOT NULL))
);

INSERT INTO mqtt_brokers_new (id, name, type, host, port, username, password, client_id,
    ca_cert_path, client_cert_path, client_key_path, enabled, created_at, updated_at)
SELECT id, name, type,
    CASE WHEN type = 'embedded' THEN NULL ELSE host END,
    CASE WHEN type = 'embedded' THEN NULL ELSE port END,
    username, password, client_id, ca_cert_path, client_cert_path, client_key_path,
    enabled, created_at, updated_at
FROM mqtt_brokers;

DROP TABLE mqtt_brokers;
ALTER TABLE mqtt_brokers_new RENAME TO mqtt_brokers;

COMMIT;

PRAGMA foreign_keys = ON;
