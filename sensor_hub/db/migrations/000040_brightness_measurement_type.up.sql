-- Migration 000040: Add the brightness measurement type, so a light's reported
-- brightness is stored and acknowledges a brightness command.
INSERT OR IGNORE INTO measurement_types (name, display_name, category, default_unit) VALUES
    ('brightness', 'Brightness', 'numeric', '');

INSERT OR IGNORE INTO measurement_type_aggregations (measurement_type_id, function, is_default)
SELECT id, 'avg', 1 FROM measurement_types WHERE name = 'brightness';

INSERT OR IGNORE INTO measurement_type_aggregations (measurement_type_id, function, is_default)
SELECT id, 'min', 0 FROM measurement_types WHERE name = 'brightness';

INSERT OR IGNORE INTO measurement_type_aggregations (measurement_type_id, function, is_default)
SELECT id, 'max', 0 FROM measurement_types WHERE name = 'brightness';
