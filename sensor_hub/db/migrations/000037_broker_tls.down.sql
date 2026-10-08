-- The path columns come back empty: the paths they held were dropped.
BEGIN;

ALTER TABLE mqtt_brokers ADD COLUMN ca_cert_path TEXT DEFAULT NULL;
ALTER TABLE mqtt_brokers ADD COLUMN client_cert_path TEXT DEFAULT NULL;
ALTER TABLE mqtt_brokers ADD COLUMN client_key_path TEXT DEFAULT NULL;
ALTER TABLE mqtt_brokers DROP COLUMN ca_cert_pem;
ALTER TABLE mqtt_brokers DROP COLUMN tls;

COMMIT;
