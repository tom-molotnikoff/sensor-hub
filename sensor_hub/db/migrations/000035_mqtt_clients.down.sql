PRAGMA foreign_keys = OFF;

BEGIN;

CREATE TABLE mqtt_brokers_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL CHECK (type IN ('embedded', 'external')) DEFAULT 'external',
    host TEXT NOT NULL,
    port INTEGER NOT NULL DEFAULT 1883,
    username TEXT DEFAULT NULL,
    password TEXT DEFAULT NULL,
    client_id TEXT DEFAULT NULL,
    ca_cert_path TEXT DEFAULT NULL,
    client_cert_path TEXT DEFAULT NULL,
    client_key_path TEXT DEFAULT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

INSERT INTO mqtt_brokers_old (id, name, type, host, port, username, password, client_id,
    ca_cert_path, client_cert_path, client_key_path, enabled, created_at, updated_at)
SELECT id, name, type, COALESCE(host, 'localhost'), COALESCE(port, 1883),
    username, password, client_id, ca_cert_path, client_cert_path, client_key_path,
    enabled, created_at, updated_at
FROM mqtt_brokers;

DROP TABLE mqtt_brokers;
ALTER TABLE mqtt_brokers_old RENAME TO mqtt_brokers;

COMMIT;

PRAGMA foreign_keys = ON;

DROP TABLE mqtt_clients;
