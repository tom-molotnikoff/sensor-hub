-- The passwords stay encrypted in secrets, so the column comes back empty.
ALTER TABLE mqtt_brokers ADD COLUMN password TEXT DEFAULT NULL;
