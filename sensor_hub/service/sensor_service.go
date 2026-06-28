package service

import (
	"context"
	"encoding/json"
	"errors"
	"example/sensorHub/actuation"
	"example/sensorHub/alerting"
	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	"example/sensorHub/drivers"
	gen "example/sensorHub/gen"
	"example/sensorHub/notifications"
	"example/sensorHub/periodic"
	"example/sensorHub/telemetry"
	"example/sensorHub/ws"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"
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

type SensorService struct {
	sensorRepo         database.SensorRepositoryInterface[gen.Sensor]
	readingsRepo       database.ReadingsRepository
	mtRepo             database.MeasurementTypeRepository
	thresholdProcessor *alerting.ThresholdAlertProcessor
	notifSvc           NotificationServiceInterface
	readingsObserver   actuation.ReadingsObserver
	logger             *slog.Logger
}

func NewSensorService(sensorRepo database.SensorRepositoryInterface[gen.Sensor], readingsRepo database.ReadingsRepository, mtRepo database.MeasurementTypeRepository, processor *alerting.ThresholdAlertProcessor, notifSvc NotificationServiceInterface, logger *slog.Logger) *SensorService {
	return &SensorService{
		sensorRepo:         sensorRepo,
		readingsRepo:       readingsRepo,
		mtRepo:             mtRepo,
		thresholdProcessor: processor,
		notifSvc:           notifSvc,
		logger:             logger.With("component", "sensor_service"),
	}
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

func (s *SensorService) SetReadingsObserver(observer actuation.ReadingsObserver) {
	s.readingsObserver = observer
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
	go s.broadcastSensors(context.Background())
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
	go s.broadcastSensors(context.Background())
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
	err = s.sensorRepo.DeleteSensorByName(ctx, name)
	if err != nil {
		return fmt.Errorf("error deleting sensor: %w", err)
	}
	s.logger.Info("sensor deleted", "name", name)
	go s.broadcastSensors(context.Background())
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
	return s.enrichSensor(sensor), nil
}

func (s *SensorService) ServiceGetSensorById(ctx context.Context, id int) (*gen.Sensor, error) {
	sensor, err := s.sensorRepo.GetSensorById(ctx, id)
	if err != nil {
		return nil, err
	}
	if sensor == nil {
		return nil, nil
	}
	return s.enrichSensor(sensor), nil
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
	return s.enrichSensors(sensors), nil
}

func (s *SensorService) ServiceGetSensorsByDriver(ctx context.Context, sensorDriver string) ([]gen.Sensor, error) {
	sensors, err := s.sensorRepo.GetSensorsByDriver(ctx, sensorDriver)
	if err != nil {
		return nil, err
	}
	return s.enrichSensors(sensors), nil
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
	return s.enrichSensor(sensor), nil
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

	var allReadings []gen.Reading
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
		err = s.readingsRepo.Add(sensorCtx, readings)
		if err != nil {
			sensorSpan.RecordError(err)
			sensorSpan.SetStatus(codes.Error, "storage failed")
			sensorSpan.End()
			s.logger.Error("error storing readings", "sensor", sensor.Name, "error", err)
			continue
		}
		sensorSpan.SetAttributes(attribute.Int("readings.count", len(readings)))
		sensorSpan.End()

		s.ServiceUpdateSensorHealthById(ctx, sensor.Id, gen.Good, "successful reading")
		allReadings = append(allReadings, readings...)
		s.logger.Debug("collected readings", "sensor", sensor.Name, "count", len(readings))

		// Process alerts for each reading
		for _, reading := range readings {
			numVal := 0.0
			textVal := ""
			if reading.NumericValue != nil {
				numVal = *reading.NumericValue
			}
			if reading.TextState != nil {
				textVal = *reading.TextState
			}
			if err := s.thresholdProcessor.ProcessReading(ctx, alerting.ReadingAlert{SensorID: sensor.Id, SensorName: sensor.Name, MeasurementType: reading.MeasurementType, NumericValue: numVal, StatusValue: textVal}); err != nil {
				s.logger.Error("failed to process alert", "sensor", sensor.Name, "error", err)
			}
		}
	}
	ws.PublishReadings(allReadings)
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
		err = s.readingsRepo.Add(ctx, readings)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "storage failed")
			s.ServiceUpdateSensorHealthById(ctx, sensor.Id, gen.Bad, fmt.Sprintf("error storing readings: %v", err))
			return fmt.Errorf("error storing readings from sensor %s: %w", sensorName, err)
		}
		span.SetAttributes(attribute.Int("readings.count", len(readings)))
		s.ServiceUpdateSensorHealthById(ctx, sensor.Id, gen.Good, "successful reading")
		s.logger.Debug("collected readings", "sensor", sensorName, "count", len(readings))
		ws.PublishReadings(readings)

		// Process alerts for each reading
		for _, reading := range readings {
			numVal := 0.0
			textVal := ""
			if reading.NumericValue != nil {
				numVal = *reading.NumericValue
			}
			if reading.TextState != nil {
				textVal = *reading.TextState
			}
			if err := s.thresholdProcessor.ProcessReading(ctx, alerting.ReadingAlert{SensorID: sensor.Id, SensorName: sensorName, MeasurementType: reading.MeasurementType, NumericValue: numVal, StatusValue: textVal}); err != nil {
				s.logger.Error("failed to process alert", "sensor", sensorName, "error", err)
			}
		}
	}
	return nil
}

func (s *SensorService) ServiceUpdateSensorHealthById(ctx context.Context, sensorId int, healthStatus gen.SensorHealthStatus, healthReason string) {
	err := s.sensorRepo.UpdateSensorHealthById(ctx, sensorId, healthStatus, healthReason)
	if err != nil {
		s.logger.Error("error updating sensor health", "error", err)
		return
	}
	go s.broadcastSensors(context.Background())
}

func (s *SensorService) ServiceCollectReadingToValidateSensor(ctx context.Context, sensor gen.Sensor) error {
	driver, ok := drivers.Get(sensor.SensorDriver)
	if !ok {
		return fmt.Errorf("unsupported sensor driver %s for sensor %s", sensor.SensorDriver, sensor.Name)
	}
	return driver.ValidateSensor(ctx, sensor)
}

func (s *SensorService) ServiceDiscoverSensors(ctx context.Context) error {
	shouldSkipDiscovery := appProps.AppConfig.SensorDiscoverySkip

	if shouldSkipDiscovery {
		s.logger.Info("skipping sensor discovery as per configuration")
		return nil
	}

	fileData, err := os.ReadFile(appProps.AppConfig.OpenAPILocation)
	if err != nil {
		return fmt.Errorf("cannot find the openapi.yaml file for the temperature sensors: %w", err)
	}
	var servers SensorServers

	err = yaml.Unmarshal(fileData, &servers)
	if err != nil {
		return fmt.Errorf("cannot unmarshal the yaml into a map: %w", err)
	}

	for _, value := range servers.Servers {
		sensorName := value.Variables["sensor_name"].Default
		url := value.Url
		sensorType := value.Variables["sensor_type"].Default

		sensor := gen.Sensor{
			Name:         sensorName,
			SensorDriver: sensorType,
			Config:       map[string]string{"url": url},
		}
		err = s.ServiceAddSensor(ctx, sensor)
		if err != nil {
			s.logger.Warn("error adding sensor during discovery", "sensor", sensorName, "error", err)
			var alreadyExistsErr *AlreadyExistsError
			if errors.As(err, &alreadyExistsErr) {
				s.logger.Info("sensor already exists, updating", "sensor", sensorName)
				err = s.ServiceUpdateSensorById(ctx, sensor, false)
				if err != nil {
					s.logger.Error("error updating sensor during discovery", "sensor", sensorName, "error", err)
				} else {
					s.logger.Info("sensor updated during discovery", "sensor", sensor.Name)
				}
			}
			continue
		}
		s.logger.Info("sensor discovered and added", "sensor", sensor.Name)
	}
	return nil
}

func (s *SensorService) ServiceStartPeriodicSensorCollection(ctx context.Context) {
	intervalSec := appProps.AppConfig.SensorCollectionInterval

	periodic.RunTask(ctx, periodic.TaskConfig{
		Name:           "sensor_collection",
		Interval:       time.Duration(intervalSec) * time.Second,
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
	go s.broadcastSensors(context.Background())
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

func (s *SensorService) ServiceGetTotalReadingsForEachSensor(ctx context.Context) (map[string]int, error) {
	sensors, err := s.sensorRepo.GetSensorsByStatus(ctx, "active")
	if err != nil {
		return nil, fmt.Errorf("error retrieving active sensors: %w", err)
	}

	totalReadings := make(map[string]int)
	for _, sensor := range sensors {
		count, err := s.readingsRepo.GetTotalReadingsBySensorId(ctx, sensor.Id)
		if err != nil {
			return nil, fmt.Errorf("error retrieving total readings for sensor %s: %w", sensor.Name, err)
		}
		totalReadings[sensor.Name] = count
	}
	return totalReadings, nil
}

func (s *SensorService) ServiceGetSensorHealthHistoryByName(ctx context.Context, name string) ([]gen.SensorHealthHistory, error) {
	sensorId, err := s.ServiceGetSensorIdByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("error retrieving sensor ID for sensor %s: %w", name, err)
	}

	retentionDays := 0
	if appProps.AppConfig != nil {
		retentionDays = appProps.AppConfig.HealthHistoryRetentionDays
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

func (s *SensorService) broadcastSensors(ctx context.Context) {
	sensors, err := s.sensorRepo.GetAllSensors(ctx)
	if err != nil {
		s.logger.Error("failed to fetch sensors for broadcast", "error", err)
		return
	}
	sensors = s.enrichSensors(sensors)

	// Per-driver broadcast (existing WebSocket subscribers)
	byType := make(map[string][]gen.Sensor)
	for _, sensor := range sensors {
		byType[sensor.SensorDriver] = append(byType[sensor.SensorDriver], sensor)
	}
	for t, list := range byType {
		topic := "sensors:" + t
		ws.BroadcastToTopic(topic, list)
	}

	// Unified broadcast — only active sensors
	active := make([]gen.Sensor, 0, len(sensors))
	for _, sensor := range sensors {
		if sensor.Status == gen.SensorStatusActive {
			active = append(active, sensor)
		}
	}
	ws.BroadcastToTopic("sensors:all", active)
}

func (s *SensorService) ServiceGetSensorsByStatus(ctx context.Context, status string) ([]gen.Sensor, error) {
	sensors, err := s.sensorRepo.GetSensorsByStatus(ctx, status)
	if err != nil {
		return nil, err
	}
	return s.enrichSensors(sensors), nil
}

func (s *SensorService) ServiceApproveSensor(ctx context.Context, sensorId int) error {
	return s.sensorRepo.UpdateSensorStatus(ctx, sensorId, string(gen.SensorStatusActive))
}

func (s *SensorService) ServiceDismissSensor(ctx context.Context, sensorId int) error {
	return s.sensorRepo.UpdateSensorStatus(ctx, sensorId, string(gen.SensorStatusDismissed))
}

// ServiceProcessPushReadings stores readings from push-based (MQTT) sensors,
// processes alerts, updates health, and broadcasts via WebSocket.
// This provides the same pipeline as the pull-based collector.
func (s *SensorService) ServiceProcessPushReadings(ctx context.Context, sensor gen.Sensor, readings []gen.Reading) error {
	if len(readings) == 0 {
		return nil
	}

	// Tag readings with sensor name
	for i := range readings {
		readings[i].SensorName = sensor.Name
	}

	if err := s.readingsRepo.Add(ctx, readings); err != nil {
		s.ServiceUpdateSensorHealthById(ctx, sensor.Id, gen.Bad, fmt.Sprintf("storage error: %v", err))
		return fmt.Errorf("failed to store push readings: %w", err)
	}

	s.ServiceUpdateSensorHealthById(ctx, sensor.Id, gen.Good, "MQTT reading received")

	// Process alerts
	for _, reading := range readings {
		numVal := 0.0
		textVal := ""
		if reading.NumericValue != nil {
			numVal = *reading.NumericValue
		}
		if reading.TextState != nil {
			textVal = *reading.TextState
		}
		if err := s.thresholdProcessor.ProcessReading(ctx, alerting.ReadingAlert{SensorID: sensor.Id, SensorName: sensor.Name, MeasurementType: reading.MeasurementType, NumericValue: numVal, StatusValue: textVal}); err != nil {
			s.logger.Error("failed to process alert for MQTT reading", "sensor", sensor.Name, "error", err)
		}
	}

	if s.readingsObserver != nil {
		s.readingsObserver.ObserveReadings(ctx, sensor.Id, readings)
	}

	// Broadcast
	ws.PublishReadings(readings)

	return nil
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

func (s *SensorService) enrichSensor(sensor *gen.Sensor) *gen.Sensor {
	enriched := *sensor
	capabilities := s.resolveSensorCapabilities(enriched)
	enriched.Capabilities = &capabilities
	return &enriched
}

func (s *SensorService) enrichSensors(sensors []gen.Sensor) []gen.Sensor {
	enriched := make([]gen.Sensor, len(sensors))
	for i := range sensors {
		capabilities := s.resolveSensorCapabilities(sensors[i])
		enriched[i] = sensors[i]
		enriched[i].Capabilities = &capabilities
	}
	return enriched
}

func (s *SensorService) resolveSensorCapabilities(sensor gen.Sensor) []gen.Capability {
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
		s.logger.Warn("failed to marshal sensor exposes metadata", "sensor", sensor.Name, "error", err)
		return []gen.Capability{}
	}

	capabilities := commandDriver.ParseCapabilities(exposesJSON)
	if capabilities == nil {
		return []gen.Capability{}
	}

	return capabilities
}
