package api

import (
	"errors"
	appProps "example/sensorHub/application_properties"
	"example/sensorHub/drivers"
	gen "example/sensorHub/gen"
	"example/sensorHub/service"
	"example/sensorHub/ws"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// computeEffectiveRetentionHours returns the sensor's custom retention if set, otherwise
// the global default (sensor.data.retention.days × 24 hours).
func computeEffectiveRetentionHours(sensor gen.Sensor) int {
	if sensor.RetentionHours != nil {
		return *sensor.RetentionHours
	}
	return appProps.AppConfig().SensorDataRetentionDays * 24
}

func (s *Server) AddSensor(c *gin.Context) {
	ctx := c.Request.Context()
	var sensor gen.Sensor
	if err := c.BindJSON(&sensor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}
	err := s.sensorService.ServiceAddSensor(ctx, sensor)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error adding sensor", "error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Sensor added successfully"})
}

// The id is extracted by the route closure; merge-patch semantics are preserved.
func (s *Server) UpdateSensorById(c *gin.Context, id int) {
	ctx := c.Request.Context()

	// Parse as raw map to support merge-patch semantics
	var body map[string]interface{}
	if err := c.BindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}

	// Load existing sensor so partial updates (e.g. retention_hours only) don't clobber other fields.
	existing, err := s.sensorService.ServiceGetSensorById(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Sensor not found", "error": err.Error()})
		return
	}

	// Start from the existing state and overlay whatever the caller provided.
	sensor := *existing
	if name, ok := body["name"].(string); ok {
		sensor.Name = name
	}
	if driver, ok := body["sensor_driver"].(string); ok {
		sensor.SensorDriver = driver
	}
	if enabled, ok := body["enabled"].(bool); ok {
		sensor.Enabled = enabled
	}

	// Handle config with merge-patch semantics
	if rawConfig, exists := body["config"]; exists {
		merged := make(map[string]string)
		for k, v := range existing.Config {
			merged[k] = v
		}
		if configMap, ok := rawConfig.(map[string]interface{}); ok {
			for k, v := range configMap {
				if v == nil {
					// null means delete key
					delete(merged, k)
					continue
				}
				if strVal, ok := v.(string); ok {
					// Skip "****" for sensitive fields — means "keep existing"
					if strVal == "****" {
						continue
					}
					merged[k] = strVal
				}
			}
		}
		sensor.Config = merged
	}

	// Handle retention_hours with explicit-presence semantics:
	// absent = no-op, null = clear custom value, positive integer = set custom value.
	retentionHoursPresent := false
	if rawRetention, exists := body["retention_hours"]; exists {
		retentionHoursPresent = true
		if rawRetention == nil {
			sensor.RetentionHours = nil
		} else if hours, ok := rawRetention.(float64); ok {
			if hours <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"message": "retention_hours must be a positive integer"})
				return
			}
			h := int(hours)
			sensor.RetentionHours = &h
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"message": "retention_hours must be a positive integer or null"})
			return
		}
	}

	err = s.sensorService.ServiceUpdateSensorById(ctx, sensor, retentionHoursPresent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error updating sensor", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Sensor updated successfully"})
}

func (s *Server) DeleteSensorByName(c *gin.Context, name string) {
	ctx := c.Request.Context()
	err := s.sensorService.ServiceDeleteSensorByName(ctx, name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error deleting sensor", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Sensor deleted successfully"})
}

// It returns the sensor with effective_retention_hours computed and set directly on gen.Sensor.
func (s *Server) GetSensorByName(c *gin.Context, name string) {
	ctx := c.Request.Context()
	sensor, err := s.sensorService.ServiceGetSensorByName(ctx, name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error retrieving sensor", "error": err.Error()})
		return
	}
	if sensor == nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Sensor not found"})
		return
	}
	masked := maskSensitiveConfig(*sensor)
	effectiveHours := computeEffectiveRetentionHours(masked)
	masked.EffectiveRetentionHours = &effectiveHours
	c.JSON(http.StatusOK, masked)
}

func (s *Server) GetSensorCapabilities(c *gin.Context, id int) {
	ctx := c.Request.Context()
	capabilities, err := s.sensorService.ServiceGetSensorCapabilities(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "no sensor found") || strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"message": "Sensor not found", "error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error retrieving sensor capabilities", "error": err.Error()})
		return
	}
	if capabilities == nil {
		capabilities = []gen.Capability{}
	}
	c.JSON(http.StatusOK, capabilities)
}

func (s *Server) GetSensorCommandHistory(c *gin.Context, id int) {
	if s.commandService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"message": "Command service unavailable"})
		return
	}

	history, err := s.commandService.GetHistory(c.Request.Context(), id)
	if err != nil {
		var commandErr *service.CommandError
		if errors.As(err, &commandErr) {
			c.JSON(commandErr.StatusCode, gin.H{"message": commandErr.Message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error retrieving sensor command history", "error": err.Error()})
		return
	}
	if history == nil {
		history = []gen.CommandHistoryEntry{}
	}
	c.JSON(http.StatusOK, history)
}

func (s *Server) SendSensorCommand(c *gin.Context, id int) {
	if s.commandService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"message": "Command service unavailable"})
		return
	}

	var body struct {
		Property string `json:"property"`
		Value    string `json:"value"`
	}
	if err := c.BindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}

	currentUser, exists := c.Get("currentUser")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Not authenticated"})
		return
	}
	actor, ok := currentUser.(*gen.User)
	if !ok || actor == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Not authenticated"})
		return
	}

	result, err := s.commandService.Send(c.Request.Context(), id, actor, body.Property, body.Value)
	if err != nil {
		var commandErr *service.CommandError
		if errors.As(err, &commandErr) {
			c.JSON(commandErr.StatusCode, gin.H{"message": commandErr.Message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error sending sensor command", "error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"id":       result.ID,
		"status":   result.Status,
		"property": result.Property,
		"value":    result.Value,
	})
}

func (s *Server) GetAllSensors(c *gin.Context) {
	ctx := c.Request.Context()
	sensors, err := s.sensorService.ServiceGetAllSensors(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error retrieving sensors", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, maskSensitiveConfigSlice(sensors))
}

func (s *Server) GetSensorsByDriver(c *gin.Context, driver string) {
	ctx := c.Request.Context()
	sensors, err := s.sensorService.ServiceGetSensorsByDriver(ctx, driver)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error retrieving sensors by driver", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, maskSensitiveConfigSlice(sensors))
}

func (s *Server) SensorExists(c *gin.Context, name string) {
	ctx := c.Request.Context()
	exists, err := s.sensorService.ServiceSensorExists(ctx, name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error checking sensor existence", "error": err.Error()})
		return
	}
	if exists {
		c.Status(http.StatusOK)
	} else {
		c.Status(http.StatusNotFound)
	}
}

func (s *Server) CollectAllSensorReadings(c *gin.Context) {
	ctx := c.Request.Context()
	err := s.sensorService.ServiceCollectAndStoreAllSensorReadings(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error collecting sensor readings", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Sensor readings collected and stored successfully"})
}

func (s *Server) CollectFromSensor(c *gin.Context, sensorName string) {
	ctx := c.Request.Context()
	err := s.sensorService.ServiceCollectFromSensorByName(ctx, sensorName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error collecting from sensor", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Sensor reading collected successfully"})
}

func (s *Server) DisableSensor(c *gin.Context, sensorName string) {
	ctx := c.Request.Context()
	err := s.sensorService.ServiceSetEnabledSensorByName(ctx, sensorName, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error disabling sensor", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Sensor disabled successfully"})
}

func (s *Server) EnableSensor(c *gin.Context, sensorName string) {
	ctx := c.Request.Context()
	err := s.sensorService.ServiceSetEnabledSensorByName(ctx, sensorName, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error enabling sensor", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Sensor enabled successfully"})
}

func (s *Server) SubscribeAllSensors(c *gin.Context) {
	ctx := c.Request.Context()
	topic := "sensors:all"
	conn := createPushWebSocket(c, topic)
	if conn == nil {
		return
	}

	sensors, err := s.sensorService.ServiceGetAllSensors(ctx)
	if err != nil {
		slog.Error("error retrieving all sensors for WebSocket broadcast", "error", err)
		return
	}

	active := make([]gen.Sensor, 0, len(sensors))
	for _, s := range sensors {
		if s.Status == gen.SensorStatusActive {
			active = append(active, s)
		}
	}
	ws.Send(conn, active)
}

func (s *Server) SubscribeSensorsByDriver(c *gin.Context, driver string) {
	ctx := c.Request.Context()

	topic := "sensors:" + driver
	conn := createPushWebSocket(c, topic)
	if conn == nil {
		return
	}

	sensors, err := s.sensorService.ServiceGetSensorsByDriver(ctx, driver)
	if err != nil {
		slog.Error("error retrieving sensors by driver for WebSocket broadcast", "driver", driver, "error", err)
		return
	}

	ws.Send(conn, sensors)
}

func (s *Server) GetSensorHealthHistoryByName(c *gin.Context, name string) {
	ctx := c.Request.Context()

	healthHistory, err := s.sensorService.ServiceGetSensorHealthHistoryByName(ctx, name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error retrieving sensor health history", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, healthHistory)
}

func (s *Server) GetTotalReadingsPerSensor(c *gin.Context) {
	c.JSON(http.StatusOK, s.sensorService.ServiceGetTotalReadingsForEachSensor())
}

// maskSensitiveConfig returns a copy of the sensor with sensitive config fields masked.
func maskSensitiveConfig(sensor gen.Sensor) gen.Sensor {
	driver, ok := drivers.Get(sensor.SensorDriver)
	if !ok {
		return sensor
	}
	sensitiveKeys := make(map[string]bool)
	for _, f := range driver.ConfigFields() {
		if f.Sensitive {
			sensitiveKeys[f.Key] = true
		}
	}
	if len(sensitiveKeys) > 0 && len(sensor.Config) > 0 {
		masked := make(map[string]string, len(sensor.Config))
		for k, v := range sensor.Config {
			if sensitiveKeys[k] && v != "" {
				masked[k] = "****"
			} else {
				masked[k] = v
			}
		}
		sensor.Config = masked
	}
	return sensor
}

// maskSensitiveConfigSlice masks sensitive config fields in a slice of sensors.
func maskSensitiveConfigSlice(sensors []gen.Sensor) []gen.Sensor {
	result := make([]gen.Sensor, len(sensors))
	for i, s := range sensors {
		result[i] = maskSensitiveConfig(s)
	}
	return result
}

func (s *Server) GetSensorsByStatus(c *gin.Context, status gen.GetSensorsByStatusParamsStatus) {
	ctx := c.Request.Context()
	sensors, err := s.sensorService.ServiceGetSensorsByStatus(ctx, string(status))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error retrieving sensors by status", "error": err.Error()})
		return
	}
	if sensors == nil {
		sensors = []gen.Sensor{}
	}
	c.JSON(http.StatusOK, maskSensitiveConfigSlice(sensors))
}

func (s *Server) ApproveSensor(c *gin.Context, id int) {
	ctx := c.Request.Context()
	if err := s.sensorService.ServiceApproveSensor(ctx, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Sensor approved"})
}

func (s *Server) DismissSensor(c *gin.Context, id int) {
	ctx := c.Request.Context()
	if err := s.sensorService.ServiceDismissSensor(ctx, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Sensor dismissed"})
}

func (s *Server) GetSensorMeasurementTypes(c *gin.Context, id int) {
	ctx := c.Request.Context()
	mts, err := s.sensorService.ServiceGetMeasurementTypesForSensor(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	if mts == nil {
		mts = []gen.MeasurementType{}
	}
	c.JSON(http.StatusOK, mts)
}

func (s *Server) GetAllMeasurementTypes(c *gin.Context, params gen.GetAllMeasurementTypesParams) {
	ctx := c.Request.Context()

	var mts []gen.MeasurementType
	var err error

	if params.HasReadings != nil && *params.HasReadings {
		mts, err = s.sensorService.ServiceGetAllMeasurementTypesWithReadings(ctx)
	} else {
		mts, err = s.sensorService.ServiceGetAllMeasurementTypes(ctx)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	if mts == nil {
		mts = []gen.MeasurementType{}
	}
	c.JSON(http.StatusOK, mts)
}
