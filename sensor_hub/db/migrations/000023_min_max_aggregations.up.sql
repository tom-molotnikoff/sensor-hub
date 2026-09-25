INSERT OR IGNORE INTO measurement_type_aggregations (measurement_type_id, function, is_default)
SELECT id, 'min', 0 FROM measurement_types
WHERE name IN ('temperature','humidity','pressure','power','battery','voltage',
               'luminance','link_quality','illuminance','energy','current','co2',
               'voc','formaldehyde','pm25','soil_moisture','energy_today',
               'energy_month','energy_yesterday');

INSERT OR IGNORE INTO measurement_type_aggregations (measurement_type_id, function, is_default)
SELECT id, 'max', 0 FROM measurement_types
WHERE name IN ('temperature','humidity','pressure','power','battery','voltage',
               'luminance','link_quality','illuminance','energy','current','co2',
               'voc','formaldehyde','pm25','soil_moisture','energy_today',
               'energy_month','energy_yesterday');
