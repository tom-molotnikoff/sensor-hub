-- Migration 000040: Add the brightness and colour temperature measurement
-- types, so a light's reported values are stored and acknowledge the commands
-- that set them.
INSERT OR IGNORE INTO measurement_types (name, display_name, category, default_unit) VALUES
    ('brightness', 'Brightness', 'numeric', ''),
    ('color_temp', 'Colour Temperature', 'numeric', 'mired');

INSERT OR IGNORE INTO measurement_type_aggregations (measurement_type_id, function, is_default)
SELECT id, 'avg', 1 FROM measurement_types WHERE name IN ('brightness', 'color_temp');

INSERT OR IGNORE INTO measurement_type_aggregations (measurement_type_id, function, is_default)
SELECT id, 'min', 0 FROM measurement_types WHERE name IN ('brightness', 'color_temp');

INSERT OR IGNORE INTO measurement_type_aggregations (measurement_type_id, function, is_default)
SELECT id, 'max', 0 FROM measurement_types WHERE name IN ('brightness', 'color_temp');
