DELETE FROM measurement_type_aggregations
WHERE measurement_type_id IN (SELECT id FROM measurement_types WHERE name = 'brightness');
DELETE FROM measurement_types WHERE name = 'brightness';
