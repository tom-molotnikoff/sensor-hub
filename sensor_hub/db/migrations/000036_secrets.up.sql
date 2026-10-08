-- Credentials the hub presents to other systems, encrypted with AES-256-GCM
-- under a key kept outside the database. owner names what the secret belongs
-- to, such as mqtt_broker:<id>, and key_id the key that sealed it.
--
-- Broker passwords move here from mqtt_brokers.password in a Go step that runs
-- at startup, because encrypting them needs the key. That step also drops the
-- password column.
CREATE TABLE secrets (
    owner TEXT NOT NULL,
    name TEXT NOT NULL,
    key_id INTEGER NOT NULL,
    nonce BLOB NOT NULL,
    ciphertext BLOB NOT NULL,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (owner, name)
);
