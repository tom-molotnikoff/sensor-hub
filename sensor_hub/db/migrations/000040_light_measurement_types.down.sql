DELETE FROM measurement_type_aggregations
WHERE measurement_type_id IN (SELECT id FROM measurement_types WHERE name IN ('brightness', 'color_temp'));
DELETE FROM measurement_types WHERE name IN ('brightness', 'color_temp');
