package database

import (
	"context"

	gen "example/sensorHub/gen"
)

type MeasurementTypeRepository interface {
	GetAll(ctx context.Context) ([]gen.MeasurementType, error)
	GetAllWithReadings(ctx context.Context) ([]gen.MeasurementType, error)
	GetByName(ctx context.Context, name string) (*gen.MeasurementType, error)
	GetIdByName(ctx context.Context, name string) (int, error)
	GetBySensorId(ctx context.Context, sensorId int) ([]SensorMeasurementType, error)
	GetMeasurementTypesWithReadings(ctx context.Context, sensorId int) ([]gen.MeasurementType, error)
	EnsureExists(ctx context.Context, mt gen.MeasurementType) error
	AssignToSensor(ctx context.Context, sensorId, measurementTypeId int, unit string) error
	RemoveFromSensor(ctx context.Context, sensorId, measurementTypeId int) error
	GetAggregationsForMeasurementType(ctx context.Context, name string) (*MeasurementTypeAggregation, error)
}
