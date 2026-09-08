-- Migration 000022: sensor_measurement_types becomes the truth about which series exist.
--
-- A row now means "this sensor has reported this type at least once", so nothing has to
-- scan readings to find out. Rows left over from the 000006 back-fill and from driver
-- declarations describe pairs that never reported, so they go; every pair with readings
-- gains one. Rows already present keep their configured unit override.
DELETE FROM sensor_measurement_types
WHERE NOT EXISTS (
    SELECT 1 FROM readings
    WHERE readings.sensor_id = sensor_measurement_types.sensor_id
      AND readings.measurement_type_id = sensor_measurement_types.measurement_type_id
);

INSERT OR IGNORE INTO sensor_measurement_types (sensor_id, measurement_type_id)
SELECT DISTINCT sensor_id, measurement_type_id FROM readings;
