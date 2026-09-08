package database

import (
	"context"
	"database/sql"
	gen "example/sensorHub/gen"
	"fmt"
	"log/slog"
	"strings"
)

type MeasurementTypeRepositoryImpl struct {
	db     *Handles
	logger *slog.Logger
}

func NewMeasurementTypeRepository(db *Handles, logger *slog.Logger) MeasurementTypeRepository {
	return &MeasurementTypeRepositoryImpl{db: db, logger: logger.With("component", "measurement_type_repository")}
}

func (r *MeasurementTypeRepositoryImpl) GetAll(ctx context.Context) ([]gen.MeasurementType, error) {
	query := fmt.Sprintf(`
		SELECT mt.id, mt.name, mt.display_name, mt.category, mt.default_unit,
			COALESCE(mta.function, 'avg') AS default_aggregation_function,
			COALESCE((SELECT GROUP_CONCAT(mta2.function, ',') FROM measurement_type_aggregations mta2 WHERE mta2.measurement_type_id = mt.id ORDER BY mta2.function), 'avg') AS supported_aggregation_functions
		FROM %s mt
		LEFT JOIN measurement_type_aggregations mta ON mta.measurement_type_id = mt.id AND mta.is_default = 1
		ORDER BY mt.name
	`, TableMeasurementTypes)
	rows, err := r.db.Reader.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying measurement types: %w", err)
	}
	defer rows.Close()

	var mts []gen.MeasurementType
	for rows.Next() {
		var mt gen.MeasurementType
		var supported string
		if err := rows.Scan(&mt.Id, &mt.Name, &mt.DisplayName, &mt.Category, &mt.Unit, &mt.DefaultAggregationFunction, &supported); err != nil {
			return nil, fmt.Errorf("error scanning measurement type row: %w", err)
		}
		mt.SupportedAggregationFunctions = strings.Split(supported, ",")
		mts = append(mts, mt)
	}
	return mts, rows.Err()
}

func (r *MeasurementTypeRepositoryImpl) GetAllWithReadings(ctx context.Context) ([]gen.MeasurementType, error) {
	query := fmt.Sprintf(`
		SELECT DISTINCT mt.id, mt.name, mt.display_name, mt.category, mt.default_unit,
			COALESCE(mta.function, 'avg') AS default_aggregation_function,
			COALESCE((SELECT GROUP_CONCAT(mta2.function, ',') FROM measurement_type_aggregations mta2 WHERE mta2.measurement_type_id = mt.id ORDER BY mta2.function), 'avg') AS supported_aggregation_functions
		FROM %s mt
		INNER JOIN %s r ON r.measurement_type_id = mt.id
		LEFT JOIN measurement_type_aggregations mta ON mta.measurement_type_id = mt.id AND mta.is_default = 1
		ORDER BY mt.name
	`, TableMeasurementTypes, TableReadings)

	rows, err := r.db.Reader.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying measurement types with readings: %w", err)
	}
	defer rows.Close()

	var mts []gen.MeasurementType
	for rows.Next() {
		var mt gen.MeasurementType
		var supported string
		if err := rows.Scan(&mt.Id, &mt.Name, &mt.DisplayName, &mt.Category, &mt.Unit, &mt.DefaultAggregationFunction, &supported); err != nil {
			return nil, fmt.Errorf("error scanning measurement type row: %w", err)
		}
		mt.SupportedAggregationFunctions = strings.Split(supported, ",")
		mts = append(mts, mt)
	}
	return mts, rows.Err()
}

func (r *MeasurementTypeRepositoryImpl) GetByName(ctx context.Context, name string) (*gen.MeasurementType, error) {
	query := fmt.Sprintf(`
		SELECT mt.id, mt.name, mt.display_name, mt.category, mt.default_unit,
			COALESCE(mta.function, 'avg') AS default_aggregation_function,
			COALESCE((SELECT GROUP_CONCAT(mta2.function, ',') FROM measurement_type_aggregations mta2 WHERE mta2.measurement_type_id = mt.id ORDER BY mta2.function), 'avg') AS supported_aggregation_functions
		FROM %s mt
		LEFT JOIN measurement_type_aggregations mta ON mta.measurement_type_id = mt.id AND mta.is_default = 1
		WHERE LOWER(mt.name) = LOWER(?)
	`, TableMeasurementTypes)
	var mt gen.MeasurementType
	var supported string
	err := r.db.Reader.QueryRowContext(ctx, query, name).Scan(&mt.Id, &mt.Name, &mt.DisplayName, &mt.Category, &mt.Unit, &mt.DefaultAggregationFunction, &supported)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error querying measurement type by name: %w", err)
	}
	mt.SupportedAggregationFunctions = strings.Split(supported, ",")
	return &mt, nil
}

func (r *MeasurementTypeRepositoryImpl) GetBySensorId(ctx context.Context, sensorId int) ([]SensorMeasurementType, error) {
	query := fmt.Sprintf(`
		SELECT smt.sensor_id, smt.measurement_type_id, mt.name, COALESCE(NULLIF(smt.unit, ''), mt.default_unit)
		FROM %s smt
		JOIN %s mt ON smt.measurement_type_id = mt.id
		WHERE smt.sensor_id = ?
		ORDER BY mt.name
	`, TableSensorMeasurementTypes, TableMeasurementTypes)

	rows, err := r.db.Reader.QueryContext(ctx, query, sensorId)
	if err != nil {
		return nil, fmt.Errorf("error querying sensor measurement types: %w", err)
	}
	defer rows.Close()

	var smts []SensorMeasurementType
	for rows.Next() {
		var smt SensorMeasurementType
		if err := rows.Scan(&smt.SensorId, &smt.MeasurementTypeId, &smt.MeasurementType, &smt.Unit); err != nil {
			return nil, fmt.Errorf("error scanning sensor measurement type row: %w", err)
		}
		smts = append(smts, smt)
	}
	return smts, rows.Err()
}

func (r *MeasurementTypeRepositoryImpl) EnsureExists(ctx context.Context, mt gen.MeasurementType) error {
	query := fmt.Sprintf("INSERT OR IGNORE INTO %s (name, display_name, category, default_unit) VALUES (?, ?, ?, ?)", TableMeasurementTypes)
	_, err := r.db.Writer.ExecContext(ctx, query, mt.Name, mt.DisplayName, mt.Category, mt.Unit)
	if err != nil {
		return fmt.Errorf("error ensuring measurement type exists: %w", err)
	}
	return nil
}

func (r *MeasurementTypeRepositoryImpl) AssignToSensor(ctx context.Context, sensorId, measurementTypeId int, unit string) error {
	query := fmt.Sprintf("INSERT OR IGNORE INTO %s (sensor_id, measurement_type_id, unit) VALUES (?, ?, ?)", TableSensorMeasurementTypes)
	_, err := r.db.Writer.ExecContext(ctx, query, sensorId, measurementTypeId, unit)
	if err != nil {
		return fmt.Errorf("error assigning measurement type to sensor: %w", err)
	}
	return nil
}

func (r *MeasurementTypeRepositoryImpl) RemoveFromSensor(ctx context.Context, sensorId, measurementTypeId int) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE sensor_id = ? AND measurement_type_id = ?", TableSensorMeasurementTypes)
	_, err := r.db.Writer.ExecContext(ctx, query, sensorId, measurementTypeId)
	if err != nil {
		return fmt.Errorf("error removing measurement type from sensor: %w", err)
	}
	return nil
}

func (r *MeasurementTypeRepositoryImpl) GetMeasurementTypesWithReadings(ctx context.Context, sensorId int) ([]gen.MeasurementType, error) {
	query := fmt.Sprintf(`
		SELECT DISTINCT mt.id, mt.name, mt.display_name, mt.category,
			COALESCE(NULLIF(smt.unit, ''), mt.default_unit) AS unit,
			COALESCE(mta.function, 'avg') AS default_aggregation_function,
			COALESCE((SELECT GROUP_CONCAT(mta2.function, ',') FROM measurement_type_aggregations mta2 WHERE mta2.measurement_type_id = mt.id ORDER BY mta2.function), 'avg') AS supported_aggregation_functions
		FROM %s r
		JOIN %s mt ON r.measurement_type_id = mt.id
		LEFT JOIN %s smt ON smt.sensor_id = r.sensor_id AND smt.measurement_type_id = mt.id
		LEFT JOIN measurement_type_aggregations mta ON mta.measurement_type_id = mt.id AND mta.is_default = 1
		WHERE r.sensor_id = ?
		ORDER BY mt.name
	`, TableReadings, TableMeasurementTypes, TableSensorMeasurementTypes)

	rows, err := r.db.Reader.QueryContext(ctx, query, sensorId)
	if err != nil {
		return nil, fmt.Errorf("error querying measurement types with readings: %w", err)
	}
	defer rows.Close()

	var mts []gen.MeasurementType
	for rows.Next() {
		var mt gen.MeasurementType
		var supported string
		if err := rows.Scan(&mt.Id, &mt.Name, &mt.DisplayName, &mt.Category, &mt.Unit, &mt.DefaultAggregationFunction, &supported); err != nil {
			return nil, fmt.Errorf("error scanning measurement type row: %w", err)
		}
		mt.SupportedAggregationFunctions = strings.Split(supported, ",")
		mts = append(mts, mt)
	}
	return mts, rows.Err()
}

func (r *MeasurementTypeRepositoryImpl) GetAggregationsForMeasurementType(ctx context.Context, name string) (*MeasurementTypeAggregation, error) {
	query := `
		SELECT mta.function, mta.is_default
		FROM measurement_type_aggregations mta
		JOIN measurement_types mt ON mta.measurement_type_id = mt.id
		WHERE LOWER(mt.name) = LOWER(?)
		ORDER BY mta.is_default DESC, mta.function ASC
	`
	rows, err := r.db.Reader.QueryContext(ctx, query, name)
	if err != nil {
		return nil, fmt.Errorf("error fetching aggregations for measurement type %q: %w", name, err)
	}
	defer func() { _ = rows.Close() }()

	result := &MeasurementTypeAggregation{
		MeasurementType: name,
	}
	for rows.Next() {
		var fn string
		var isDefault int
		if err := rows.Scan(&fn, &isDefault); err != nil {
			return nil, fmt.Errorf("error scanning aggregation row: %w", err)
		}
		result.SupportedFunctions = append(result.SupportedFunctions, fn)
		if isDefault == 1 {
			result.DefaultFunction = fn
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating aggregation rows: %w", err)
	}
	if len(result.SupportedFunctions) == 0 {
		result.DefaultFunction = "avg"
		result.SupportedFunctions = []string{"avg"}
	}
	return result, nil
}
