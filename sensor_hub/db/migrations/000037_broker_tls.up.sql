-- Outbound brokers can be reached over TLS with the server verified. tls
-- switches it on, and ca_cert_pem holds the CA to verify against as PEM
-- content, so it works in a container without a mount; without it the system
-- roots are used. The certificate path columns were never read, so they go.
BEGIN;

ALTER TABLE mqtt_brokers ADD COLUMN tls INTEGER NOT NULL DEFAULT 0;
ALTER TABLE mqtt_brokers ADD COLUMN ca_cert_pem TEXT NULL;
ALTER TABLE mqtt_brokers DROP COLUMN client_cert_path;
ALTER TABLE mqtt_brokers DROP COLUMN client_key_path;
ALTER TABLE mqtt_brokers DROP COLUMN ca_cert_path;

COMMIT;
