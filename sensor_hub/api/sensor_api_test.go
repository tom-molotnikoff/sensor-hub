package api

import (
	"bytes"
	"encoding/json"
	"errors"
	appProps "example/sensorHub/application_properties"
	gen "example/sensorHub/gen"
	servicepkg "example/sensorHub/service"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func init() {
	appProps.SetAppConfig(&appProps.ApplicationConfiguration{
		SensorDataRetentionDays: 30,
	})
}

func setupSensorRouter() (*gin.Engine, *gin.RouterGroup, *Server, *MockSensorService) {
	mockService := new(MockSensorService)
	s := &Server{sensorService: mockService}
	router := gin.New()
	apiGroup := router.Group("/api")
	return router, apiGroup, s, mockService
}

func TestAddSensorHandler(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.POST("/sensors", s.AddSensor)

	sensor := gen.Sensor{Name: "test-sensor", SensorDriver: "sensor-hub-http-temperature", Config: map[string]string{"url": "http://localhost:8080"}}
	jsonBody, _ := json.Marshal(sensor)

	mockService.On("ServiceAddSensor", mock.Anything, sensor).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors", bytes.NewBuffer(jsonBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestGetAllSensorsHandler(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.GET("/sensors", s.GetAllSensors)

	mockService.On("ServiceGetAllSensors", mock.Anything).Return([]gen.Sensor{{Name: "s1"}}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "s1")
}

func TestGetSensorByNameHandler(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.GET("/sensors/:name", func(c *gin.Context) {
		s.GetSensorByName(c, c.Param("name"))
	})

	mockService.On("ServiceGetSensorByName", mock.Anything, "s1").Return(&gen.Sensor{Name: "s1"}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors/s1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "s1")
	assert.Contains(t, w.Body.String(), "effective_retention_hours")
}

func TestUpdateSensorHandler(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.PUT("/sensors/:id", func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid sensor ID"})
			return
		}
		s.UpdateSensorById(c, id)
	})

	existing := gen.Sensor{Id: 1, Name: "s1", SensorDriver: "sensor-hub-http-temperature", Config: map[string]string{"url": "http://localhost:8080"}, Enabled: true}
	update := map[string]interface{}{"name": "s1-updated", "sensor_driver": "sensor-hub-http-temperature", "config": map[string]interface{}{"url": "http://localhost:8080"}}
	jsonBody, _ := json.Marshal(update)

	expected := existing
	expected.Name = "s1-updated"

	mockService.On("ServiceGetSensorById", mock.Anything, 1).Return(&existing, nil)
	mockService.On("ServiceUpdateSensorById", mock.Anything, expected, mock.AnythingOfType("bool")).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/sensors/1", bytes.NewBuffer(jsonBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateSensorHandler_IgnoresMetadataFromRequest(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.PUT("/sensors/:id", func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid sensor ID"})
			return
		}
		s.UpdateSensorById(c, id)
	})

	metadata := map[string]interface{}{"manufacturer": "Aqara"}
	existing := gen.Sensor{
		Id:           1,
		Name:         "s1",
		SensorDriver: "mqtt-zigbee2mqtt",
		Config:       map[string]string{},
		Enabled:      true,
		Metadata:     &metadata,
	}
	update := map[string]interface{}{
		"metadata": map[string]interface{}{"manufacturer": "Injected"},
	}
	jsonBody, _ := json.Marshal(update)

	mockService.On("ServiceGetSensorById", mock.Anything, 1).Return(&existing, nil)
	mockService.On("ServiceUpdateSensorById", mock.Anything, existing, false).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/sensors/1", bytes.NewBuffer(jsonBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteSensorHandler(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.DELETE("/sensors/:name", func(c *gin.Context) {
		s.DeleteSensorByName(c, c.Param("name"))
	})

	mockService.On("ServiceDeleteSensorByName", mock.Anything, "s1").Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/sensors/s1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCollectAndStoreAllSensorReadingsHandler(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.POST("/sensors/collect", s.CollectAllSensorReadings)

	mockService.On("ServiceCollectAndStoreAllSensorReadings", mock.Anything).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors/collect", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCollectFromSensorByNameHandler(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.POST("/sensors/:sensorName/collect", func(c *gin.Context) {
		s.CollectFromSensor(c, c.Param("sensorName"))
	})

	mockService.On("ServiceCollectFromSensorByName", mock.Anything, "s1").Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors/s1/collect", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEnableSensorHandler(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.POST("/sensors/:sensorName/enable", func(c *gin.Context) {
		s.EnableSensor(c, c.Param("sensorName"))
	})

	mockService.On("ServiceSetEnabledSensorByName", mock.Anything, "s1", true).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors/s1/enable", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDisableSensorHandler(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.POST("/sensors/:sensorName/disable", func(c *gin.Context) {
		s.DisableSensor(c, c.Param("sensorName"))
	})

	mockService.On("ServiceSetEnabledSensorByName", mock.Anything, "s1", false).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors/s1/disable", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTotalReadingsPerSensorHandler(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.GET("/sensors/readings/total", s.GetTotalReadingsPerSensor)

	sampledAt := time.Date(2026, 9, 8, 10, 30, 0, 0, time.UTC)
	mockService.On("ServiceGetTotalReadingsForEachSensor").Return(gen.TotalReadingsSample{
		SampledAt: sampledAt,
		Counts:    map[string]int{"s1": 10},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors/readings/total", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body gen.TotalReadingsSample
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, sampledAt, body.SampledAt)
	assert.Equal(t, map[string]int{"s1": 10}, body.Counts)
}

func TestGetSensorsByDriverHandler(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.GET("/sensors/driver/:driver", func(c *gin.Context) {
		s.GetSensorsByDriver(c, c.Param("driver"))
	})

	mockService.On("ServiceGetSensorsByDriver", mock.Anything, "sensor-hub-http-temperature").Return([]gen.Sensor{{Name: "s1"}}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors/driver/sensor-hub-http-temperature", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "s1")
}

func TestGetSensorCapabilitiesHandler(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.GET("/sensors/by-id/:id/capabilities", func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid sensor ID"})
			return
		}
		s.GetSensorCapabilities(c, id)
	})

	mockService.On("ServiceGetSensorCapabilities", mock.Anything, 7).Return([]gen.Capability{
		{Property: "state", Type: gen.CapabilityTypeBinary},
	}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors/by-id/7/capabilities", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "state")
}

func TestGetSensorCapabilitiesHandler_NotFound(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.GET("/sensors/by-id/:id/capabilities", func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid sensor ID"})
			return
		}
		s.GetSensorCapabilities(c, id)
	})

	mockService.On("ServiceGetSensorCapabilities", mock.Anything, 7).Return([]gen.Capability(nil), errors.New("sensor not found"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors/by-id/7/capabilities", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetSensorCommandHistoryHandler_EmptyHistory(t *testing.T) {
	router, api, s, _ := setupSensorRouter()
	mockCommandService := new(MockCommandService)
	s.commandService = mockCommandService
	api.GET("/sensors/by-id/:id/commands", func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid sensor ID"})
			return
		}
		s.GetSensorCommandHistory(c, id)
	})

	mockCommandService.On("GetHistory", mock.Anything, 7).Return([]gen.CommandHistoryEntry{}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors/by-id/7/commands", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `[]`, w.Body.String())
}

func TestGetSensorCommandHistoryHandler_NotFound(t *testing.T) {
	router, api, s, _ := setupSensorRouter()
	mockCommandService := new(MockCommandService)
	s.commandService = mockCommandService
	api.GET("/sensors/by-id/:id/commands", func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid sensor ID"})
			return
		}
		s.GetSensorCommandHistory(c, id)
	})

	mockCommandService.On("GetHistory", mock.Anything, 7).
		Return([]gen.CommandHistoryEntry(nil), &servicepkg.CommandError{StatusCode: http.StatusNotFound, Message: "sensor 7 not found"})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors/by-id/7/commands", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSendSensorCommandHandler(t *testing.T) {
	router, api, s, _ := setupSensorRouter()
	mockCommandService := new(MockCommandService)
	s.commandService = mockCommandService
	api.POST("/sensors/:id/command", func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid sensor ID"})
			return
		}
		c.Set("currentUser", &gen.User{Id: 99, Permissions: []string{"control_sensors"}})
		s.SendSensorCommand(c, id)
	})

	body := []byte(`{"property":"state","value":"ON"}`)
	mockCommandService.On("Send", mock.Anything, 7, mock.AnythingOfType("*gen.User"), "state", "ON").Return(servicepkg.SentCommandResult{
		ID:       42,
		Status:   "sent",
		Property: "state",
		Value:    "ON",
	}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors/7/command", bytes.NewBuffer(body))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Contains(t, w.Body.String(), `"id": 42`)
	assert.Contains(t, w.Body.String(), `"status": "sent"`)
}

func TestSendSensorCommandHandler_ServiceUnavailable(t *testing.T) {
	router, api, s, _ := setupSensorRouter()
	mockCommandService := new(MockCommandService)
	s.commandService = mockCommandService
	api.POST("/sensors/:id/command", func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid sensor ID"})
			return
		}
		c.Set("currentUser", &gen.User{Id: 99, Permissions: []string{"control_sensors"}})
		s.SendSensorCommand(c, id)
	})

	body := []byte(`{"property":"state","value":"ON"}`)
	mockCommandService.On("Send", mock.Anything, 7, mock.AnythingOfType("*gen.User"), "state", "ON").
		Return(servicepkg.SentCommandResult{}, &servicepkg.CommandError{StatusCode: http.StatusServiceUnavailable, Message: "broker 12 is not connected"})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors/7/command", bytes.NewBuffer(body))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "broker 12 is not connected")
}

func TestSensorExistsHandler(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.HEAD("/sensors/:name", func(c *gin.Context) {
		s.SensorExists(c, c.Param("name"))
	})

	mockService.On("ServiceSensorExists", mock.Anything, "s1").Return(true, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("HEAD", "/api/sensors/s1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetSensorHealthHistoryByNameHandler(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.GET("/sensors/:name/health", func(c *gin.Context) {
		s.GetSensorHealthHistoryByName(c, c.Param("name"))
	})

	mockService.On("ServiceGetSensorHealthHistoryByName", mock.Anything, "s1").Return([]gen.SensorHealthHistory{}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors/s1/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var history []gen.SensorHealthHistory
	err := json.Unmarshal(w.Body.Bytes(), &history)
	assert.NoError(t, err)
	assert.NotNil(t, history)
	assert.Empty(t, history)
}

func TestAddSensorHandler_InvalidJSON(t *testing.T) {
	router, api, s, _ := setupSensorRouter()
	api.POST("/sensors", s.AddSensor)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors", bytes.NewBufferString("invalid"))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAddSensorHandler_ServiceError(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.POST("/sensors", s.AddSensor)

	sensor := gen.Sensor{Name: "test-sensor", SensorDriver: "sensor-hub-http-temperature", Config: map[string]string{"url": "http://localhost:8080"}}
	jsonBody, _ := json.Marshal(sensor)

	mockService.On("ServiceAddSensor", mock.Anything, sensor).Return(errors.New("validation error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors", bytes.NewBuffer(jsonBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetSensorByNameHandler_NotFound(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.GET("/sensors/:name", func(c *gin.Context) {
		s.GetSensorByName(c, c.Param("name"))
	})

	mockService.On("ServiceGetSensorByName", mock.Anything, "notfound").Return((*gen.Sensor)(nil), nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors/notfound", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetSensorByNameHandler_ServiceError(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.GET("/sensors/:name", func(c *gin.Context) {
		s.GetSensorByName(c, c.Param("name"))
	})

	mockService.On("ServiceGetSensorByName", mock.Anything, "s1").Return((*gen.Sensor)(nil), errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors/s1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestUpdateSensorHandler_InvalidID(t *testing.T) {
	router, api, s, _ := setupSensorRouter()
	api.PUT("/sensors/:id", func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid sensor ID"})
			return
		}
		s.UpdateSensorById(c, id)
	})

	sensor := gen.Sensor{Name: "s1-updated", SensorDriver: "sensor-hub-http-temperature", Config: map[string]string{"url": "http://localhost:8080"}}
	jsonBody, _ := json.Marshal(sensor)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/sensors/invalid", bytes.NewBuffer(jsonBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateSensorHandler_InvalidJSON(t *testing.T) {
	router, api, s, _ := setupSensorRouter()
	api.PUT("/sensors/:id", func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid sensor ID"})
			return
		}
		s.UpdateSensorById(c, id)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/sensors/1", bytes.NewBufferString("invalid"))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateSensorHandler_ServiceError(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.PUT("/sensors/:id", func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid sensor ID"})
			return
		}
		s.UpdateSensorById(c, id)
	})

	existing := gen.Sensor{Id: 1, Name: "s1", SensorDriver: "sensor-hub-http-temperature", Config: map[string]string{"url": "http://localhost:8080"}}
	update := map[string]interface{}{"name": "s1-updated", "sensor_driver": "sensor-hub-http-temperature", "config": map[string]interface{}{"url": "http://localhost:8080"}}
	jsonBody, _ := json.Marshal(update)

	expected := existing
	expected.Name = "s1-updated"

	mockService.On("ServiceGetSensorById", mock.Anything, 1).Return(&existing, nil)
	mockService.On("ServiceUpdateSensorById", mock.Anything, expected, mock.AnythingOfType("bool")).Return(errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/sensors/1", bytes.NewBuffer(jsonBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDeleteSensorHandler_ServiceError(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.DELETE("/sensors/:name", func(c *gin.Context) {
		s.DeleteSensorByName(c, c.Param("name"))
	})

	mockService.On("ServiceDeleteSensorByName", mock.Anything, "s1").Return(errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/sensors/s1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetAllSensorsHandler_ServiceError(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.GET("/sensors", s.GetAllSensors)

	mockService.On("ServiceGetAllSensors", mock.Anything).Return([]gen.Sensor{}, errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetSensorsByDriverHandler_ServiceError(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.GET("/sensors/driver/:driver", func(c *gin.Context) {
		s.GetSensorsByDriver(c, c.Param("driver"))
	})

	mockService.On("ServiceGetSensorsByDriver", mock.Anything, "sensor-hub-http-temperature").Return([]gen.Sensor{}, errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors/driver/sensor-hub-http-temperature", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSensorExistsHandler_NotFound(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.HEAD("/sensors/:name", func(c *gin.Context) {
		s.SensorExists(c, c.Param("name"))
	})

	mockService.On("ServiceSensorExists", mock.Anything, "notfound").Return(false, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("HEAD", "/api/sensors/notfound", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSensorExistsHandler_ServiceError(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.HEAD("/sensors/:name", func(c *gin.Context) {
		s.SensorExists(c, c.Param("name"))
	})

	mockService.On("ServiceSensorExists", mock.Anything, "s1").Return(false, errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("HEAD", "/api/sensors/s1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCollectAndStoreAllSensorReadingsHandler_ServiceError(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.POST("/sensors/collect", s.CollectAllSensorReadings)

	mockService.On("ServiceCollectAndStoreAllSensorReadings", mock.Anything).Return(errors.New("collection error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors/collect", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCollectFromSensorByNameHandler_ServiceError(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.POST("/sensors/:sensorName/collect", func(c *gin.Context) {
		s.CollectFromSensor(c, c.Param("sensorName"))
	})

	mockService.On("ServiceCollectFromSensorByName", mock.Anything, "s1").Return(errors.New("sensor offline"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors/s1/collect", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestEnableSensorHandler_ServiceError(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.POST("/sensors/:sensorName/enable", func(c *gin.Context) {
		s.EnableSensor(c, c.Param("sensorName"))
	})

	mockService.On("ServiceSetEnabledSensorByName", mock.Anything, "s1", true).Return(errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors/s1/enable", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDisableSensorHandler_ServiceError(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.POST("/sensors/:sensorName/disable", func(c *gin.Context) {
		s.DisableSensor(c, c.Param("sensorName"))
	})

	mockService.On("ServiceSetEnabledSensorByName", mock.Anything, "s1", false).Return(errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors/s1/disable", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetSensorHealthHistoryByNameHandler_IgnoresLegacyLimitQuery(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.GET("/sensors/:name/health", func(c *gin.Context) {
		s.GetSensorHealthHistoryByName(c, c.Param("name"))
	})

	mockService.On("ServiceGetSensorHealthHistoryByName", mock.Anything, "s1").Return([]gen.SensorHealthHistory{}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors/s1/health?limit=invalid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetSensorHealthHistoryByNameHandler_ServiceError(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.GET("/sensors/:name/health", func(c *gin.Context) {
		s.GetSensorHealthHistoryByName(c, c.Param("name"))
	})

	mockService.On("ServiceGetSensorHealthHistoryByName", mock.Anything, "s1").Return([]gen.SensorHealthHistory{}, errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors/s1/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ============================================================================
// Sensor Status Handlers
// ============================================================================

func TestGetSensorsByStatusHandler(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.GET("/sensors/status/:status", func(c *gin.Context) {
		s.GetSensorsByStatus(c, gen.GetSensorsByStatusParamsStatus(c.Param("status")))
	})

	mockService.On("ServiceGetSensorsByStatus", mock.Anything, "pending").Return([]gen.Sensor{
		{Id: 1, Name: "auto-sensor", Status: "pending"},
	}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors/status/pending", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "auto-sensor")
}

func TestGetSensorsByStatusHandler_Empty(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.GET("/sensors/status/:status", func(c *gin.Context) {
		s.GetSensorsByStatus(c, gen.GetSensorsByStatusParamsStatus(c.Param("status")))
	})

	mockService.On("ServiceGetSensorsByStatus", mock.Anything, "pending").Return(nil, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors/status/pending", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "[]")
}

func TestGetSensorsByStatusHandler_Error(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.GET("/sensors/status/:status", func(c *gin.Context) {
		s.GetSensorsByStatus(c, gen.GetSensorsByStatusParamsStatus(c.Param("status")))
	})

	mockService.On("ServiceGetSensorsByStatus", mock.Anything, "pending").Return(nil, errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors/status/pending", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestApproveSensorHandler(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.POST("/sensors/approve/:id", func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid sensor ID"})
			return
		}
		s.ApproveSensor(c, id)
	})

	mockService.On("ServiceApproveSensor", mock.Anything, 1).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors/approve/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "approved")
}

func TestApproveSensorHandler_InvalidID(t *testing.T) {
	router, api, s, _ := setupSensorRouter()
	api.POST("/sensors/approve/:id", func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid sensor ID"})
			return
		}
		s.ApproveSensor(c, id)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors/approve/abc", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestApproveSensorHandler_Error(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.POST("/sensors/approve/:id", func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid sensor ID"})
			return
		}
		s.ApproveSensor(c, id)
	})

	mockService.On("ServiceApproveSensor", mock.Anything, 1).Return(errors.New("not found"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors/approve/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDismissSensorHandler(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.POST("/sensors/dismiss/:id", func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid sensor ID"})
			return
		}
		s.DismissSensor(c, id)
	})

	mockService.On("ServiceDismissSensor", mock.Anything, 1).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors/dismiss/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "dismissed")
}

func TestDismissSensorHandler_InvalidID(t *testing.T) {
	router, api, s, _ := setupSensorRouter()
	api.POST("/sensors/dismiss/:id", func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid sensor ID"})
			return
		}
		s.DismissSensor(c, id)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors/dismiss/abc", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDismissSensorHandler_Error(t *testing.T) {
	router, api, s, mockService := setupSensorRouter()
	api.POST("/sensors/dismiss/:id", func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid sensor ID"})
			return
		}
		s.DismissSensor(c, id)
	})

	mockService.On("ServiceDismissSensor", mock.Anything, 1).Return(errors.New("not found"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors/dismiss/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetAllMeasurementTypes(t *testing.T) {
	router, _, s, mockService := setupSensorRouter()
	router.GET("/api/measurement-types", func(c *gin.Context) {
		var params gen.GetAllMeasurementTypesParams
		s.GetAllMeasurementTypes(c, params)
	})

	mts := []gen.MeasurementType{{Name: "temperature"}}
	mockService.On("ServiceGetAllMeasurementTypes", mock.Anything).Return(mts, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/measurement-types", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "temperature")
}

func TestGetAllMeasurementTypes_HasReadings(t *testing.T) {
	router, _, s, mockService := setupSensorRouter()
	router.GET("/api/measurement-types", func(c *gin.Context) {
		hasReadings := c.Query("has_readings") == "true"
		params := gen.GetAllMeasurementTypesParams{HasReadings: &hasReadings}
		s.GetAllMeasurementTypes(c, params)
	})

	mts := []gen.MeasurementType{{Name: "humidity"}}
	mockService.On("ServiceGetAllMeasurementTypesWithReadings", mock.Anything).Return(mts, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/measurement-types?has_readings=true", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "humidity")
}

func TestGetSensorMeasurementTypes(t *testing.T) {
	router, _, s, mockService := setupSensorRouter()
	router.GET("/api/sensors/:id/measurement-types", func(c *gin.Context) {
		var id int
		fmt.Sscan(c.Param("id"), &id)
		s.GetSensorMeasurementTypes(c, id)
	})

	mts := []gen.MeasurementType{{Name: "temperature"}}
	mockService.On("ServiceGetMeasurementTypesForSensor", mock.Anything, 1).Return(mts, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors/1/measurement-types", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "temperature")
}
