package database

import (
	"context"
	gen "example/sensorHub/gen"
	"example/sensorHub/readings"
	"time"
)

type SensorIDResolver interface {
	GetSensorIdByName(ctx context.Context, name string) (int, error)
}

type MeasurementTypeIDResolver interface {
	GetIdByName(ctx context.Context, name string) (int, error)
}

type ReadingsRepository interface {
	Ingest(ctx context.Context, batch readings.ReadingBatch) ([]gen.Reading, error)
	GetBetweenDates(ctx context.Context, startDate, endDate, sensorName, measurementType string, interval AggregationInterval, aggFunc AggregationFunction) ([]gen.Reading, error)
	GetLatest(ctx context.Context) ([]gen.Reading, error)
	// Newest first.
	LatestNumericValues(ctx context.Context, sensorID int, measurementTypeID int, limit int) ([]float64, error)
	CountReadingsPerActiveSensor(ctx context.Context) (map[string]int, error)
	DeleteReadingsOlderThan(ctx context.Context, cutoffDate time.Time) error
	DeleteReadingsOlderThanForSensor(ctx context.Context, cutoffDate time.Time, sensorId int) error
	DeleteReadingsOlderThanExcludingSensors(ctx context.Context, cutoffDate time.Time, excludedSensorIds []int) error
}
