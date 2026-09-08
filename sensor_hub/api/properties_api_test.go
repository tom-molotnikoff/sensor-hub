package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	appProps "example/sensorHub/application_properties"
	gen "example/sensorHub/gen"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupPropertiesRouter() (*gin.Engine, *gin.RouterGroup, *Server, *MockPropertiesService) {
	mockService := new(MockPropertiesService)
	s := &Server{propertiesService: mockService}
	router := gin.New()
	apiGroup := router.Group("/api")
	return router, apiGroup, s, mockService
}

func TestUpdateProperties(t *testing.T) {
	router, api, s, mockService := setupPropertiesRouter()
	api.PATCH("/properties", s.UpdateProperties)

	props := map[string]string{"key": "value"}
	jsonBody, _ := json.Marshal(props)

	mockService.On("ServiceUpdateProperties", mock.Anything, props).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/properties", bytes.NewBuffer(jsonBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestGetProperties(t *testing.T) {
	router, api, s, mockService := setupPropertiesRouter()
	api.GET("/properties", s.GetProperties)

	mockService.On("ServiceGetProperties", mock.Anything).Return(map[string]interface{}{"key": "value"}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/properties", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "value")
}

func TestGetPropertyDefinitions_Shape(t *testing.T) {
	router, api, s, _ := setupPropertiesRouter()
	api.GET("/properties/definitions", s.GetPropertyDefinitions)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/properties/definitions", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp gen.PropertyDefinitionsResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Len(t, resp.Definitions, 30)
	assert.Len(t, resp.Groups, 7)

	for i, g := range resp.Groups {
		assert.NotEmpty(t, g.Id)
		assert.NotEmpty(t, g.Label)
		assert.NotEmpty(t, g.Description)
		assert.Equal(t, i+1, g.Order)
	}

	byKey := make(map[string]gen.PropertyDefinition)
	for _, d := range resp.Definitions {
		assert.NotEmpty(t, d.Key)
		assert.NotEmpty(t, d.Label)
		assert.NotEmpty(t, d.Description)
		assert.NotEmpty(t, d.Group)
		assert.NotEmpty(t, d.Apply)
		byKey[d.Key] = d
	}

	interval := byKey["sensor.collection.interval"]
	assert.Equal(t, "Collection interval", interval.Label)
	assert.Equal(t, gen.Int, interval.Type)
	assert.Equal(t, "300", interval.Default)
	assert.Equal(t, "sensors", interval.Group)
	assert.NotNil(t, interval.Unit)
	assert.Equal(t, "seconds", *interval.Unit)
	assert.Equal(t, "next-cycle", interval.Apply)
	assert.NotNil(t, interval.Validate)
	assert.Equal(t, "positive", *interval.Validate)
	assert.False(t, interval.ReadOnly)

	logLevel := byKey["log.level"]
	assert.NotNil(t, logLevel.Enum)
	assert.Equal(t, []string{"debug", "info", "warn", "error"}, *logLevel.Enum)

	dbPath := byKey["database.path"]
	assert.True(t, dbPath.ReadOnly)
	assert.Equal(t, "readonly", dbPath.Apply)
}

func TestUpdateProperties_InvalidJSON(t *testing.T) {
	router, api, s, _ := setupPropertiesRouter()
	api.PATCH("/properties", s.UpdateProperties)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/properties", bytes.NewBufferString("invalid"))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateProperties_ValidationFailureIs400NamingTheKey(t *testing.T) {
	router, api, s, mockService := setupPropertiesRouter()
	api.PATCH("/properties", s.UpdateProperties)

	props := map[string]string{"sensor.collection.interval": "abc"}
	jsonBody, _ := json.Marshal(props)

	mockService.On("ServiceUpdateProperties", mock.Anything, props).Return(
		&appProps.ValidationError{Key: "sensor.collection.interval", Message: "invalid sensor.collection.interval value: abc"})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/properties", bytes.NewBuffer(jsonBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var body struct {
		Message string `json:"message"`
		Key     string `json:"key"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "invalid sensor.collection.interval value: abc", body.Message)
	assert.Equal(t, "sensor.collection.interval", body.Key)
}

func TestUpdateProperties_ServiceError(t *testing.T) {
	router, api, s, mockService := setupPropertiesRouter()
	api.PATCH("/properties", s.UpdateProperties)

	props := map[string]string{"key": "value"}
	jsonBody, _ := json.Marshal(props)

	mockService.On("ServiceUpdateProperties", mock.Anything, props).Return(errors.New("validation error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/properties", bytes.NewBuffer(jsonBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetProperties_ServiceError(t *testing.T) {
	router, api, s, mockService := setupPropertiesRouter()
	api.GET("/properties", s.GetProperties)

	mockService.On("ServiceGetProperties", mock.Anything).Return(map[string]interface{}{}, errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/properties", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
