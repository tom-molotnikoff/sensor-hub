ALTER TABLE automation_triggers ADD COLUMN sensor_id INTEGER REFERENCES sensors(id) ON DELETE CASCADE;
ALTER TABLE automation_triggers ADD COLUMN measurement_type_id INTEGER REFERENCES measurement_types(id);
ALTER TABLE automation_triggers ADD COLUMN operator TEXT;
ALTER TABLE automation_triggers ADD COLUMN threshold REAL;
ALTER TABLE automation_triggers ADD COLUMN binary_value TEXT;
ALTER TABLE automation_triggers ADD COLUMN rearm_margin REAL;
ALTER TABLE automation_triggers ADD COLUMN hold_seconds INTEGER CHECK (kind <> 'reading' OR (
    sensor_id IS NOT NULL AND measurement_type_id IS NOT NULL AND hold_seconds IS NOT NULL AND hold_seconds >= 0 AND (
        (operator IN ('falls_below', 'rises_above') AND threshold IS NOT NULL AND rearm_margin IS NOT NULL AND rearm_margin >= 0 AND binary_value IS NULL)
        OR (operator = 'becomes' AND binary_value IS NOT NULL AND threshold IS NULL AND rearm_margin IS NULL))));

CREATE INDEX idx_automation_triggers_sensor ON automation_triggers(sensor_id) WHERE sensor_id IS NOT NULL;
