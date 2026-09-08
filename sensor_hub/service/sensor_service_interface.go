package service

import (
	"context"
	gen "example/sensorHub/gen"
)

type SensorServiceInterface interface {
	ServiceAddSensor(ctx context.Context, sensor gen.Sensor) error
	ServiceUpdateSensorById(ctx context.Context, sensor gen.Sensor, retentionHoursPresent bool) error
	ServiceDeleteSensorByName(ctx context.Context, name string) error
	ServiceGetSensorByName(ctx context.Context, name string) (*gen.Sensor, error)
	ServiceGetSensorByExternalId(ctx context.Context, externalId string) (*gen.Sensor, error)
	ServiceGetSensorById(ctx context.Context, id int) (*gen.Sensor, error)
	ServiceGetSensorCapabilities(ctx context.Context, id int) ([]gen.Capability, error)
	ServiceGetAllSensors(ctx context.Context) ([]gen.Sensor, error)
	ServiceGetSensorsByDriver(ctx context.Context, sensorDriver string) ([]gen.Sensor, error)
	ServiceGetSensorIdByName(ctx context.Context, name string) (int, error)
	ServiceSensorExists(ctx context.Context, name string) (bool, error)
	ServiceSensorExistsByExternalId(ctx context.Context, externalId string) (bool, error)
	ServiceCollectAndStoreAllSensorReadings(ctx context.Context) error
	ServiceCollectFromSensorByName(ctx context.Context, sensorName string) error
	ServiceCollectReadingToValidateSensor(ctx context.Context, sensor gen.Sensor) error
	ServiceStartPeriodicSensorCollection(ctx context.Context)
	ServiceDiscoverSensors(ctx context.Context) error
	ServiceValidateSensorConfig(ctx context.Context, sensor gen.Sensor) error
	ServiceUpdateSensorHealthById(ctx context.Context, sensorId int, healthStatus gen.SensorHealthStatus, healthReason string)
	ServiceSetEnabledSensorByName(ctx context.Context, name string, enabled bool) error
	ServiceGetSensorHealthHistoryByName(ctx context.Context, name string) ([]gen.SensorHealthHistory, error)
	ServiceGetTotalReadingsForEachSensor() gen.TotalReadingsSample
	ServiceGetSensorsByStatus(ctx context.Context, status string) ([]gen.Sensor, error)
	ServiceApproveSensor(ctx context.Context, sensorId int) error
	ServiceDismissSensor(ctx context.Context, sensorId int) error
	ServiceProcessPushReadings(ctx context.Context, sensor gen.Sensor, readings []gen.Reading) error
	ServiceGetMeasurementTypesForSensor(ctx context.Context, sensorId int) ([]gen.MeasurementType, error)
	ServiceGetAllMeasurementTypes(ctx context.Context) ([]gen.MeasurementType, error)
	ServiceGetAllMeasurementTypesWithReadings(ctx context.Context) ([]gen.MeasurementType, error)
}
