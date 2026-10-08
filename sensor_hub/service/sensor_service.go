package service

import (
	"context"
	"encoding/json"
	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	"example/sensorHub/drivers"
	gen "example/sensorHub/gen"
	"example/sensorHub/notifications"
	"example/sensorHub/periodic"
	"example/sensorHub/readings"
	"example/sensorHub/telemetry"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type AlreadyExistsError struct {
	Message string
}

func NewAlreadyExistsError(message string) *AlreadyExistsError {
	return &AlreadyExistsError{Message: message}
}

func (e *AlreadyExistsError) Error() string {
	return e.Message
}

// A SensorObserver hears about changes that can break the automations using a
// sensor, and about the sensor's deletion, which deletes them.
type SensorObserver interface {
	SensorChanged(ctx context.Context, sensorID int)
	SensorDeleted(sensorID int)
}

type SensorService struct {
	sensorRepo      database.SensorRepositoryInterface[gen.Sensor]
	mtRepo          database.MeasurementTypeRepository
	pipeline        *readings.Pipeline
	liveView        *LiveView
	notifSvc        NotificationServiceInterface
	readingsSampler ReadingsSamplerInterface
	observer        SensorObserver
	logger          *slog.Logger
}

func NewSensorService(sensorRepo database.SensorRepositoryInterface[gen.Sensor], mtRepo database.MeasurementTypeRepository, pipeline *readings.Pipeline, liveView *LiveView, notifSvc NotificationServiceInterface, readingsSampler ReadingsSamplerInterface, logger *slog.Logger) *SensorService {
	return &SensorService{
		sensorRepo:      sensorRepo,
		mtRepo:          mtRepo,
		pipeline:        pipeline,
		liveView:        liveView,
		notifSvc:        notifSvc,
		readingsSampler: readingsSampler,
		logger:          logger.With("component", "sensor_service"),
	}
}

// SetSensorObserver takes an observer built after the sensor service, because
// the automations it tells depend on the sensor service.
func (s *SensorService) SetSensorObserver(observer SensorObserver) {
	s.observer = observer
}

func (s *SensorService) notifyConfigEvent(action, sensorName string, metadata map[string]interface{}) {
	if s.notifSvc == nil {
		return
	}
	notif := notifications.Notification{
		Category: notifications.CategoryConfigChange,
		Severity: notifications.SeverityInfo,
		Title:    fmt.Sprintf("Sensor %s", action),
		Message:  fmt.Sprintf("Sensor '%s' was %s", sensorName, action),
		Metadata: metadata,
	}
	go s.notifSvc.CreateNotification(context.Background(), notif, "view_notifications_config")
}

func (s *SensorService) ServiceAddSensor(ctx context.Context, sensor gen.Sensor) error {
	err := s.ServiceValidateSensorConfig(ctx, sensor)
	if err != nil {
		return fmt.Errorf("sensor validation failed: %w", err)
	}

	exists, err := s.sensorRepo.SensorExists(ctx, sensor.Name)
	if err != nil {
		return fmt.Errorf("error checking if sensor exists: %w", err)
	}
	if exists {
		return NewAlreadyExistsError(fmt.Sprintf("sensor with name %s already exists", sensor.Name))
	}
	if sensor.ExternalId != nil && *sensor.ExternalId != "" {
		extExists, err := s.sensorRepo.SensorExistsByExternalId(ctx, *sensor.ExternalId)
		if err != nil {
			return fmt.Errorf("error checking if sensor exists by external_id: %w", err)
		}
		if extExists {
			return NewAlreadyExistsError(fmt.Sprintf("sensor with external_id %s already exists", *sensor.ExternalId))
		}
	}
	err = s.sensorRepo.AddSensor(ctx, sensor)
	if err != nil {
		return fmt.Errorf("error adding sensor: %w", err)
	}
	s.logger.Info("sensor added", "name", sensor.Name)
	s.liveView.AnnounceSensors()
	s.notifyConfigEvent("added", sensor.Name, map[string]interface{}{"sensor_name": sensor.Name})
	return nil
}

func (s *SensorService) ServiceUpdateSensorById(ctx context.Context, sensor gen.Sensor, retentionHoursPresent bool) error {
	err := s.ServiceValidateSensorConfig(ctx, sensor)
	if err != nil {
		return fmt.Errorf("sensor validation failed: %w", err)
	}
	err = s.sensorRepo.UpdateSensorById(ctx, sensor, retentionHoursPresent)
	if err != nil {
		return fmt.Errorf("error updating sensor: %w", err)
	}
	s.logger.Info("sensor updated", "id", sensor.Id, "name", sensor.Name)
	if s.observer != nil {
		s.observer.SensorChanged(ctx, sensor.Id)
	}
	s.liveView.AnnounceSensors()
	s.notifyConfigEvent("updated", sensor.Name, map[string]interface{}{"sensor_name": sensor.Name})
	return nil
}

func (s *SensorService) ServiceDeleteSensorByName(ctx context.Context, name string) error {
	exists, err := s.sensorRepo.SensorExists(ctx, name)
	if err != nil {
		return fmt.Errorf("error checking if sensor exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("sensor with name %s does not exist", name)
	}
	sensorID, err := s.sensorRepo.GetSensorIdByName(ctx, name)
	if err != nil {
		return fmt.Errorf("error retrieving sensor ID for deletion: %w", err)
	}
	err = s.sensorRepo.DeleteSensorByName(ctx, name)
	if err != nil {
		return fmt.Errorf("error deleting sensor: %w", err)
	}
	s.logger.Info("sensor deleted", "name", name)
	if s.observer != nil {
		s.observer.SensorDeleted(sensorID)
	}
	s.liveView.AnnounceSensors()
	s.notifyConfigEvent("removed", name, map[string]interface{}{"sensor_name": name})
	return nil
}

func (s *SensorService) ServiceGetSensorByName(ctx context.Context, name string) (*gen.Sensor, error) {
	if name == "" {
		return nil, fmt.Errorf("sensor name cannot be empty")
	}
	sensor, err := s.sensorRepo.GetSensorByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if sensor == nil {
		return nil, nil
	}
	return enrichSensor(sensor, s.logger), nil
}

func (s *SensorService) ServiceGetSensorById(ctx context.Context, id int) (*gen.Sensor, error) {
	sensor, err := s.sensorRepo.GetSensorById(ctx, id)
	if err != nil {
		return nil, err
	}
	if sensor == nil {
		return nil, nil
	}
	return enrichSensor(sensor, s.logger), nil
}

func (s *SensorService) ServiceGetSensorCapabilities(ctx context.Context, id int) ([]gen.Capability, error) {
	sensor, err := s.ServiceGetSensorById(ctx, id)
	if err != nil {
		return nil, err
	}
	if sensor == nil || sensor.Capabilities == nil {
		return []gen.Capability{}, nil
	}
	return append([]gen.Capability(nil), (*sensor.Capabilities)...), nil
}

func (s *SensorService) ServiceGetAllSensors(ctx context.Context) ([]gen.Sensor, error) {
	sensors, err := s.sensorRepo.GetAllSensors(ctx)
	if err != nil {
		return nil, err
	}
	return enrichSensors(sensors, s.logger), nil
}

func (s *SensorService) ServiceGetSensorsByDriver(ctx context.Context, sensorDriver string) ([]gen.Sensor, error) {
	sensors, err := s.sensorRepo.GetSensorsByDriver(ctx, sensorDriver)
	if err != nil {
		return nil, err
	}
	return enrichSensors(sensors, s.logger), nil
}

func (s *SensorService) ServiceGetSensorIdByName(ctx context.Context, name string) (int, error) {
	return s.sensorRepo.GetSensorIdByName(ctx, name)
}

func (s *SensorService) ServiceSensorExists(ctx context.Context, name string) (bool, error) {
	return s.sensorRepo.SensorExists(ctx, name)
}

func (s *SensorService) ServiceGetSensorByExternalId(ctx context.Context, externalId string) (*gen.Sensor, error) {
	if externalId == "" {
		return nil, fmt.Errorf("external_id cannot be empty")
	}
	sensor, err := s.sensorRepo.GetSensorByExternalId(ctx, externalId)
	if err != nil {
		return nil, err
	}
	if sensor == nil {
		return nil, nil
	}
	return enrichSensor(sensor, s.logger), nil
}

func (s *SensorService) ServiceSensorExistsByExternalId(ctx context.Context, externalId string) (bool, error) {
	return s.sensorRepo.SensorExistsByExternalId(ctx, externalId)
}

func (s *SensorService) ServiceCollectAndStoreAllSensorReadings(ctx context.Context) error {
	ctx, span := telemetry.Tracer("sensor-service").Start(ctx, "collect-all-sensors")
	defer span.End()

	sensors, err := s.sensorRepo.GetAllSensors(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to fetch sensors")
		return fmt.Errorf("error fetching sensors: %w", err)
	}
	span.SetAttributes(attribute.Int("sensor.count", len(sensors)))

	for _, sensor := range sensors {
		if !sensor.Enabled {
			s.logger.Debug("skipping disabled sensor", "name", sensor.Name)
			continue
		}
		driver, ok := drivers.Get(sensor.SensorDriver)
		if !ok {
			s.logger.Warn("no driver registered for sensor", "name", sensor.Name, "driver", sensor.SensorDriver)
			continue
		}

		pull, isPull := driver.(drivers.PullDriver)
		if !isPull {
			s.logger.Debug("skipping non-pull sensor in collection loop", "name", sensor.Name, "driver", sensor.SensorDriver)
			continue
		}

		sensorCtx, sensorSpan := telemetry.Tracer("sensor-service").Start(ctx, "collect-sensor",
			trace.WithAttributes(
				attribute.String("sensor.name", sensor.Name),
				attribute.String("sensor.driver", sensor.SensorDriver),
			),
		)

		readings, err := pull.CollectReadings(sensorCtx, sensor)
		if err != nil {
			sensorSpan.RecordError(err)
			sensorSpan.SetStatus(codes.Error, "collection failed")
			sensorSpan.End()
			s.ServiceUpdateSensorHealthById(ctx, sensor.Id, gen.Bad, fmt.Sprintf("error collecting readings: %v", err))
			s.logger.Error("error collecting readings from sensor", "name", sensor.Name, "error", err)
			continue
		}
		if err := s.pipeline.Process(sensorCtx, sensor, readings); err != nil {
			sensorSpan.RecordError(err)
			sensorSpan.SetStatus(codes.Error, "storage failed")
			sensorSpan.End()
			s.logger.Error("error storing readings", "sensor", sensor.Name, "error", err)
			continue
		}
		sensorSpan.SetAttributes(attribute.Int("readings.count", len(readings)))
		sensorSpan.End()
		s.logger.Debug("collected readings", "sensor", sensor.Name, "count", len(readings))
	}
	return nil
}

func (s *SensorService) ServiceCollectFromSensorByName(ctx context.Context, sensorName string) error {
	ctx, span := telemetry.Tracer("sensor-service").Start(ctx, "collect-sensor-by-name",
		trace.WithAttributes(attribute.String("sensor.name", sensorName)),
	)
	defer span.End()

	sensor, err := s.ServiceGetSensorByName(ctx, sensorName)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "sensor lookup failed")
		return fmt.Errorf("error retrieving sensor %s: %w", sensorName, err)
	}
	if sensor == nil {
		span.SetStatus(codes.Error, "sensor not found")
		return fmt.Errorf("sensor %s not found", sensorName)
	}

	span.SetAttributes(attribute.String("sensor.driver", sensor.SensorDriver))

	if !sensor.Enabled {
		span.SetStatus(codes.Error, "sensor disabled")
		return fmt.Errorf("sensor %s is disabled", sensorName)
	}

	switch sensor.SensorDriver {
	case "":
		span.SetStatus(codes.Error, "no driver configured")
		return fmt.Errorf("sensor %s has no driver configured", sensorName)
	default:
		driver, ok := drivers.Get(sensor.SensorDriver)
		if !ok {
			span.SetStatus(codes.Error, "unsupported driver")
			return fmt.Errorf("unsupported sensor driver %s for sensor %s", sensor.SensorDriver, sensorName)
		}
		pull, isPull := driver.(drivers.PullDriver)
		if !isPull {
			span.SetStatus(codes.Error, "not a pull driver")
			return fmt.Errorf("sensor %s uses driver %s which does not support on-demand collection", sensorName, sensor.SensorDriver)
		}
		readings, err := pull.CollectReadings(ctx, *sensor)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "collection failed")
			s.ServiceUpdateSensorHealthById(ctx, sensor.Id, gen.Bad, fmt.Sprintf("error collecting readings: %v", err))
			return fmt.Errorf("error collecting readings from sensor %s: %w", sensorName, err)
		}
		if err := s.pipeline.Process(ctx, *sensor, readings); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "storage failed")
			return err
		}
		span.SetAttributes(attribute.Int("readings.count", len(readings)))
		s.logger.Debug("collected readings", "sensor", sensorName, "count", len(readings))
	}
	return nil
}

func (s *SensorService) ServiceUpdateSensorHealthById(ctx context.Context, sensorId int, healthStatus gen.SensorHealthStatus, healthReason string) {
	s.liveView.RecordHealth(ctx, sensorId, healthStatus, healthReason)
}

func (s *SensorService) ServiceCollectReadingToValidateSensor(ctx context.Context, sensor gen.Sensor) error {
	driver, ok := drivers.Get(sensor.SensorDriver)
	if !ok {
		return fmt.Errorf("unsupported sensor driver %s for sensor %s", sensor.SensorDriver, sensor.Name)
	}
	return driver.ValidateSensor(ctx, sensor)
}

func (s *SensorService) ServiceStartPeriodicSensorCollection(ctx context.Context) {
	periodic.RunTask(ctx, periodic.TaskConfig{
		Name: "sensor_collection",
		Interval: func() time.Duration {
			return time.Duration(appProps.AppConfig().SensorCollectionInterval) * time.Second
		},
		Logger:         s.logger,
		RunImmediately: true,
	}, func(ctx context.Context) error {
		return s.ServiceCollectAndStoreAllSensorReadings(ctx)
	})
}

func (s *SensorService) ServiceSetEnabledSensorByName(ctx context.Context, name string, enabled bool) error {
	exists, err := s.sensorRepo.SensorExists(ctx, name)
	if err != nil {
		return fmt.Errorf("error checking if sensor exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("sensor with name %s does not exist", name)
	}
	err = s.sensorRepo.SetEnabledSensorByName(ctx, name, enabled)
	if err != nil {
		return fmt.Errorf("error setting enabled status for sensor: %w", err)
	}
	s.logger.Info("sensor enabled status changed", "name", name, "enabled", enabled)
	s.liveView.AnnounceSensors()
	if enabled {
		go func() {
			err := s.ServiceCollectFromSensorByName(context.Background(), name)
			if err != nil {
				s.logger.Error("error collecting initial reading from enabled sensor", "name", name, "error", err)
			}
		}()
	}
	return nil
}

func (s *SensorService) ServiceGetTotalReadingsForEachSensor() gen.TotalReadingsSample {
	return s.readingsSampler.LatestSample()
}

func (s *SensorService) ServiceGetSensorHealthHistoryByName(ctx context.Context, name string) ([]gen.SensorHealthHistory, error) {
	sensorId, err := s.ServiceGetSensorIdByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("error retrieving sensor ID for sensor %s: %w", name, err)
	}

	retentionDays := 0
	if cfg := appProps.AppConfig(); cfg != nil {
		retentionDays = cfg.HealthHistoryRetentionDays
	}
	since := time.Now().AddDate(0, 0, -retentionDays)

	history, err := s.sensorRepo.GetSensorHealthHistoryById(ctx, sensorId, since)
	if err != nil {
		return nil, fmt.Errorf("error retrieving health history for sensor %s: %w", name, err)
	}
	return history, nil
}

func (s *SensorService) ServiceValidateSensorConfig(ctx context.Context, sensor gen.Sensor) error {
	if sensor.Name == "" || sensor.SensorDriver == "" {
		return fmt.Errorf("sensor name and driver cannot be empty")
	}

	driver, ok := drivers.Get(sensor.SensorDriver)
	if !ok {
		return fmt.Errorf("unknown driver: %s", sensor.SensorDriver)
	}

	if sensor.Config == nil {
		sensor.Config = make(map[string]string)
	}

	for _, field := range driver.ConfigFields() {
		if field.Required {
			val, exists := sensor.Config[field.Key]
			if !exists || val == "" {
				return fmt.Errorf("config field '%s' is required for driver '%s'", field.Key, sensor.SensorDriver)
			}
		}
	}

	// Only pull drivers can be validated by trial collection
	if _, isPull := driver.(drivers.PullDriver); isPull {
		err := s.ServiceCollectReadingToValidateSensor(ctx, sensor)
		if err != nil {
			return fmt.Errorf("invalid sensor, failed to collect a reading: %w", err)
		}
	}
	return nil
}

func (s *SensorService) ServiceGetSensorsByStatus(ctx context.Context, status string) ([]gen.Sensor, error) {
	sensors, err := s.sensorRepo.GetSensorsByStatus(ctx, status)
	if err != nil {
		return nil, err
	}
	return enrichSensors(sensors, s.logger), nil
}

func (s *SensorService) ServiceApproveSensor(ctx context.Context, sensorId int) error {
	return s.sensorRepo.UpdateSensorStatus(ctx, sensorId, string(gen.SensorStatusActive))
}

func (s *SensorService) ServiceDismissSensor(ctx context.Context, sensorId int) error {
	return s.sensorRepo.UpdateSensorStatus(ctx, sensorId, string(gen.SensorStatusDismissed))
}

func (s *SensorService) ServiceProcessPushReadings(ctx context.Context, sensor gen.Sensor, readings []gen.Reading) error {
	return s.pipeline.Process(ctx, sensor, readings)
}

func (s *SensorService) ServiceGetMeasurementTypesForSensor(ctx context.Context, sensorId int) ([]gen.MeasurementType, error) {
	return s.mtRepo.GetMeasurementTypesWithReadings(ctx, sensorId)
}

func (s *SensorService) ServiceGetAllMeasurementTypes(ctx context.Context) ([]gen.MeasurementType, error) {
	return s.mtRepo.GetAll(ctx)
}

func (s *SensorService) ServiceGetAllMeasurementTypesWithReadings(ctx context.Context) ([]gen.MeasurementType, error) {
	return s.mtRepo.GetAllWithReadings(ctx)
}

func enrichSensor(sensor *gen.Sensor, logger *slog.Logger) *gen.Sensor {
	enriched := *sensor
	capabilities := resolveSensorCapabilities(enriched, logger)
	enriched.Capabilities = &capabilities
	return &enriched
}

func enrichSensors(sensors []gen.Sensor, logger *slog.Logger) []gen.Sensor {
	enriched := make([]gen.Sensor, len(sensors))
	for i := range sensors {
		capabilities := resolveSensorCapabilities(sensors[i], logger)
		enriched[i] = sensors[i]
		enriched[i].Capabilities = &capabilities
	}
	return enriched
}

func resolveSensorCapabilities(sensor gen.Sensor, logger *slog.Logger) []gen.Capability {
	commandDriver, ok := drivers.GetCommandDriver(sensor.SensorDriver)
	if !ok || sensor.Metadata == nil {
		return []gen.Capability{}
	}

	exposesValue, ok := (*sensor.Metadata)["exposes"]
	if !ok || exposesValue == nil {
		return []gen.Capability{}
	}

	exposesJSON, err := json.Marshal(exposesValue)
	if err != nil {
		logger.Warn("failed to marshal sensor exposes metadata", "sensor", sensor.Name, "error", err)
		return []gen.Capability{}
	}

	capabilities := commandDriver.ParseCapabilities(exposesJSON)
	if capabilities == nil {
		return []gen.Capability{}
	}

	return capabilities
}
