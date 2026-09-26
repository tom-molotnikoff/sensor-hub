package fixtures

import (
	"context"
	"fmt"

	gen "example/sensorHub/gen"
	"example/sensorHub/service"
)

type Sensor struct {
	Name       string
	Driver     string
	ExternalID string
	Config     map[string]string
}

func CreateApprovedSensor(ctx context.Context, sensors service.SensorServiceInterface, sensor Sensor) (int, error) {
	pending := gen.Sensor{
		Name:         sensor.Name,
		SensorDriver: sensor.Driver,
		Config:       sensor.Config,
		Status:       gen.SensorStatusPending,
	}
	if sensor.ExternalID != "" {
		pending.ExternalId = &sensor.ExternalID
	}
	if err := sensors.ServiceAddSensor(ctx, pending); err != nil {
		return 0, fmt.Errorf("failed to create sensor %s: %w", sensor.Name, err)
	}
	id, err := sensors.ServiceGetSensorIdByName(ctx, sensor.Name)
	if err != nil {
		return 0, fmt.Errorf("failed to find sensor %s after creating it: %w", sensor.Name, err)
	}
	if err := sensors.ServiceApproveSensor(ctx, id); err != nil {
		return 0, fmt.Errorf("failed to approve sensor %s: %w", sensor.Name, err)
	}
	return id, nil
}
