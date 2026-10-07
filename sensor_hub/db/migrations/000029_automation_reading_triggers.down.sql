-- The previous schema cannot hold a reading trigger.
DELETE FROM automations WHERE id IN (SELECT automation_id FROM automation_triggers WHERE kind = 'reading');

DROP INDEX idx_automation_triggers_sensor;
ALTER TABLE automation_triggers DROP COLUMN hold_seconds;
ALTER TABLE automation_triggers DROP COLUMN rearm_margin;
ALTER TABLE automation_triggers DROP COLUMN binary_value;
ALTER TABLE automation_triggers DROP COLUMN threshold;
ALTER TABLE automation_triggers DROP COLUMN operator;
ALTER TABLE automation_triggers DROP COLUMN measurement_type_id;
ALTER TABLE automation_triggers DROP COLUMN sensor_id;
