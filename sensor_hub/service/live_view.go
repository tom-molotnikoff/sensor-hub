package service

import (
	"context"
	"log/slog"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/ws"
)

type LiveView struct {
	sensorRepo database.SensorRepositoryInterface[gen.Sensor]
	logger     *slog.Logger
}

func NewLiveView(sensorRepo database.SensorRepositoryInterface[gen.Sensor], logger *slog.Logger) *LiveView {
	return &LiveView{
		sensorRepo: sensorRepo,
		logger:     logger.With("component", "live_view"),
	}
}

func (v *LiveView) Consume(_ context.Context, _ gen.Sensor, readings []gen.Reading) {
	ws.PublishReadings(readings)
	v.AnnounceSensors()
}

func (v *LiveView) RecordHealth(ctx context.Context, sensorID int, status gen.SensorHealthStatus, reason string) {
	if err := v.sensorRepo.UpdateSensorHealthById(ctx, sensorID, status, reason); err != nil {
		v.logger.Error("error updating sensor health", "error", err)
		return
	}
	v.AnnounceSensors()
}

func (v *LiveView) AnnounceSensors() {
	go v.broadcastSensors(context.Background())
}

func (v *LiveView) broadcastSensors(ctx context.Context) {
	sensors, err := v.sensorRepo.GetAllSensors(ctx)
	if err != nil {
		v.logger.Error("failed to fetch sensors for broadcast", "error", err)
		return
	}
	sensors = enrichSensors(sensors, v.logger)

	byType := make(map[string][]gen.Sensor)
	for _, sensor := range sensors {
		byType[sensor.SensorDriver] = append(byType[sensor.SensorDriver], sensor)
	}
	for t, list := range byType {
		ws.BroadcastToTopic("sensors:"+t, list)
	}

	active := make([]gen.Sensor, 0, len(sensors))
	for _, sensor := range sensors {
		if sensor.Status == gen.SensorStatusActive {
			active = append(active, sensor)
		}
	}
	ws.BroadcastToTopic("sensors:all", active)
}
