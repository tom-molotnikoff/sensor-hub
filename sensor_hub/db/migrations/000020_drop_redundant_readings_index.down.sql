-- Restore the redundant single-column sensor_id index dropped by the up-migration.
CREATE INDEX IF NOT EXISTS idx_readings_sensor_id ON readings (sensor_id);
