DELETE FROM measurement_type_aggregations WHERE function = 'increase';

CREATE TABLE measurement_type_aggregations_old (
    measurement_type_id INTEGER NOT NULL,
    function TEXT NOT NULL CHECK (function IN ('avg', 'count', 'last', 'min', 'max', 'sum')),
    is_default INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (measurement_type_id, function),
    FOREIGN KEY (measurement_type_id) REFERENCES measurement_types(id) ON DELETE CASCADE
);

INSERT INTO measurement_type_aggregations_old (measurement_type_id, function, is_default)
SELECT measurement_type_id, function, is_default FROM measurement_type_aggregations;

DROP TABLE measurement_type_aggregations;
ALTER TABLE measurement_type_aggregations_old RENAME TO measurement_type_aggregations;
