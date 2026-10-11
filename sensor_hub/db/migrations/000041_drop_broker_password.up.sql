-- Broker passwords are held encrypted in secrets (000036), so the plaintext
-- password column goes. A 2.0.x hub dropped it at startup and other databases
-- still have it, so the table is rebuilt rather than the column dropped, which
-- works either way. Foreign keys are off for the rebuild: dropping the old
-- table would otherwise cascade and delete every subscription.
PRAGMA foreign_keys = OFF;

BEGIN;

CREATE TABLE mqtt_brokers_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL CHECK (type IN ('embedded', 'external')) DEFAULT 'external',
    host TEXT,
    port INTEGER,
    username TEXT DEFAULT NULL,
    client_id TEXT DEFAULT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    tls INTEGER NOT NULL DEFAULT 0,
    ca_cert_pem TEXT,
    CHECK (type = 'embedded' OR (host IS NOT NULL AND port IS NOT NULL))
);

INSERT INTO mqtt_brokers_new (id, name, type, host, port, username, client_id,
    enabled, created_at, updated_at, tls, ca_cert_pem)
SELECT id, name, type, host, port, username, client_id,
    enabled, created_at, updated_at, tls, ca_cert_pem
FROM mqtt_brokers;

-- The copy only advances the sequence to the highest id left, so without this
-- the ids of brokers deleted from the top would be handed out again.
DELETE FROM sqlite_sequence WHERE name = 'mqtt_brokers_new';
INSERT INTO sqlite_sequence (name, seq)
SELECT 'mqtt_brokers_new', seq FROM sqlite_sequence WHERE name = 'mqtt_brokers';

DROP TABLE mqtt_brokers;
ALTER TABLE mqtt_brokers_new RENAME TO mqtt_brokers;

COMMIT;

PRAGMA foreign_keys = ON;

-- Migrations run with secure_delete on, so the pages the old table freed are
-- zeroed, but only in the WAL. The checkpoint writes the zeros over the
-- plaintext still in the database file, and truncating the WAL drops any
-- older frames that held it.
PRAGMA wal_checkpoint(TRUNCATE);
