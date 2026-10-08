-- The encrypted broker passwords cannot be decrypted without the key, so they
-- are not put back. mqtt_brokers gets its password column back, empty, whether
-- or not the startup step had dropped it. The table is rebuilt because SQLite
-- has no ADD COLUMN IF NOT EXISTS; foreign keys are off so dropping the old
-- table does not cascade to the subscriptions.
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

INSERT INTO mqtt_brokers_new (id, name, type, host, port, username, client_id,
    ca_cert_path, client_cert_path, client_key_path, enabled, created_at, updated_at)
SELECT id, name, type, host, port, username, client_id,
    ca_cert_path, client_cert_path, client_key_path, enabled, created_at, updated_at
FROM mqtt_brokers;

DROP TABLE mqtt_brokers;
ALTER TABLE mqtt_brokers_new RENAME TO mqtt_brokers;

COMMIT;

PRAGMA foreign_keys = ON;

DROP TABLE secrets;
