package api

import (
	"bytes"
	"context"
	"encoding/json"
	"example/sensorHub/alerting"
	gen "example/sensorHub/gen"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockAlertManagementService struct {
	mock.Mock
}

func (m *mockAlertManagementService) ServiceGetAllAlertRules(ctx context.Context) ([]alerting.AlertRule, error) {
	args := m.Called(ctx)
	return args.Get(0).([]alerting.AlertRule), args.Error(1)
}

func (m *mockAlertManagementService) ServiceGetAlertRuleByID(ctx context.Context, ruleID int) (*alerting.AlertRule, error) {
	args := m.Called(ctx, ruleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*alerting.AlertRule), args.Error(1)
}

func (m *mockAlertManagementService) ServiceGetAlertRuleBySensorID(ctx context.Context, sensorID int) (*alerting.AlertRule, error) {
	args := m.Called(ctx, sensorID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*alerting.AlertRule), args.Error(1)
}

func (m *mockAlertManagementService) ServiceGetAlertRulesBySensorID(ctx context.Context, sensorID int) ([]alerting.AlertRule, error) {
	args := m.Called(ctx, sensorID)
	return args.Get(0).([]alerting.AlertRule), args.Error(1)
}

func (m *mockAlertManagementService) ServiceCreateAlertRule(ctx context.Context, rule *alerting.AlertRule) error {
	args := m.Called(ctx, rule)
	return args.Error(0)
}

func (m *mockAlertManagementService) ServiceUpdateAlertRule(ctx context.Context, rule *alerting.AlertRule) error {
	args := m.Called(ctx, rule)
	return args.Error(0)
}

func (m *mockAlertManagementService) ServiceDeleteAlertRule(ctx context.Context, ruleID int) error {
	args := m.Called(ctx, ruleID)
	return args.Error(0)
}

func (m *mockAlertManagementService) ServiceGetAlertHistory(ctx context.Context, sensorID int, limit int) ([]gen.AlertHistoryEntry, error) {
	args := m.Called(ctx, sensorID, limit)
	return args.Get(0).([]gen.AlertHistoryEntry), args.Error(1)
}

// setupAlertByIDRoute wraps GetAlertRuleById in the route closure that parses the path id.
func setupAlertByIDRoute(s *Server) *gin.Engine {
	router := gin.New()
	apiGroup := router.Group("/api")
	apiGroup.GET("/alerts/:id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid alert rule ID"})
			return
		}
		s.GetAlertRuleById(c, id)
	})
	return router
}

// setupAlertsBySensorRoute wraps GetAlertRulesBySensorId in the route closure.
func setupAlertsBySensorRoute(s *Server) *gin.Engine {
	router := gin.New()
	apiGroup := router.Group("/api")
	apiGroup.GET("/alerts/sensor/:sensorId", func(c *gin.Context) {
		sensorID, err := strconv.Atoi(c.Param("sensorId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid sensor ID"})
			return
		}
		s.GetAlertRulesBySensorId(c, sensorID)
	})
	return router
}

// setupUpdateAlertRoute wraps UpdateAlertRule in the route closure that parses the path id.
func setupUpdateAlertRoute(s *Server) *gin.Engine {
	router := gin.New()
	apiGroup := router.Group("/api")
	apiGroup.PUT("/alerts/:id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid alert rule ID"})
			return
		}
		s.UpdateAlertRule(c, id)
	})
	return router
}

// setupDeleteAlertRoute wraps DeleteAlertRule in the route closure that parses the path id.
func setupDeleteAlertRoute(s *Server) *gin.Engine {
	router := gin.New()
	apiGroup := router.Group("/api")
	apiGroup.DELETE("/alerts/:id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid alert rule ID"})
			return
		}
		s.DeleteAlertRule(c, id)
	})
	return router
}

// setupAlertHistoryRoute wraps GetAlertHistory in the route closure that parses sensorId and limit.
func setupAlertHistoryRoute(s *Server) *gin.Engine {
	router := gin.New()
	apiGroup := router.Group("/api")
	apiGroup.GET("/alerts/sensor/:sensorId/history", func(c *gin.Context) {
		sensorID, err := strconv.Atoi(c.Param("sensorId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid sensor ID"})
			return
		}
		var params gen.GetAlertHistoryParams
		if limitStr := c.Query("limit"); limitStr != "" {
			limit, err := strconv.Atoi(limitStr)
			if err != nil || limit < 1 || limit > 100 {
				limit = 50
			}
			params.Limit = &limit
		}
		s.GetAlertHistory(c, sensorID, params)
	})
	return router
}

// ─── GetAllAlertRules ───────────────────────────────────────────────────────

func TestGetAllAlertRules(t *testing.T) {
	mockService := new(mockAlertManagementService)
	s := &Server{alertService: mockService}

	expectedRules := []alerting.AlertRule{
		{SensorID: 1, SensorName: "Sensor1", AlertType: alerting.AlertTypeNumericRange, HighThreshold: 30.0, LowThreshold: 10.0},
		{SensorID: 2, SensorName: "Sensor2", AlertType: alerting.AlertTypeStatusBased, TriggerStatus: "open"},
	}

	mockService.On("ServiceGetAllAlertRules", mock.Anything).Return(expectedRules, nil)

	router := setupTestRouter("/alerts", s.GetAllAlertRules)
	req := httptest.NewRequest("GET", "/api/alerts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Sensor1")
	assert.Contains(t, w.Body.String(), "Sensor2")
	mockService.AssertExpectations(t)
}

func TestGetAllAlertRules_ServiceError(t *testing.T) {
	mockService := new(mockAlertManagementService)
	s := &Server{alertService: mockService}

	mockService.On("ServiceGetAllAlertRules", mock.Anything).Return([]alerting.AlertRule{}, fmt.Errorf("database error"))

	router := setupTestRouter("/alerts", s.GetAllAlertRules)
	req := httptest.NewRequest("GET", "/api/alerts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Error fetching alert rules")
	mockService.AssertExpectations(t)
}

// ─── GetAlertRuleById ────────────────────────────────────────────────────────

func TestGetAlertRuleById(t *testing.T) {
	mockService := new(mockAlertManagementService)
	s := &Server{alertService: mockService}

	expectedRule := &alerting.AlertRule{
		ID: 1, SensorID: 1, SensorName: "TestSensor",
		AlertType: alerting.AlertTypeNumericRange, HighThreshold: 30.0, LowThreshold: 10.0,
		RateLimitSeconds: 1, Enabled: true,
	}

	mockService.On("ServiceGetAlertRuleByID", mock.Anything, 1).Return(expectedRule, nil)

	router := setupAlertByIDRoute(s)
	req := httptest.NewRequest("GET", "/api/alerts/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "TestSensor")
	mockService.AssertExpectations(t)
}

func TestGetAlertRuleById_InvalidID(t *testing.T) {
	s := new(Server)
	router := setupAlertByIDRoute(s)

	req := httptest.NewRequest("GET", "/api/alerts/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid alert rule ID")
}

func TestGetAlertRuleById_NotFound(t *testing.T) {
	mockService := new(mockAlertManagementService)
	s := &Server{alertService: mockService}

	mockService.On("ServiceGetAlertRuleByID", mock.Anything, 999).Return(nil, nil)

	router := setupAlertByIDRoute(s)
	req := httptest.NewRequest("GET", "/api/alerts/999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockService.AssertExpectations(t)
}

// ─── GetAlertRulesBySensorId ─────────────────────────────────────────────────

func TestGetAlertRulesBySensorId(t *testing.T) {
	mockService := new(mockAlertManagementService)
	s := &Server{alertService: mockService}

	expectedRules := []alerting.AlertRule{
		{ID: 1, SensorID: 1, SensorName: "TestSensor", AlertType: alerting.AlertTypeNumericRange},
		{ID: 2, SensorID: 1, SensorName: "TestSensor", AlertType: alerting.AlertTypeStatusBased},
	}

	mockService.On("ServiceGetAlertRulesBySensorID", mock.Anything, 1).Return(expectedRules, nil)

	router := setupAlertsBySensorRoute(s)
	req := httptest.NewRequest("GET", "/api/alerts/sensor/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "TestSensor")
	mockService.AssertExpectations(t)
}

func TestGetAlertRulesBySensorId_InvalidID(t *testing.T) {
	s := new(Server)
	router := setupAlertsBySensorRoute(s)

	req := httptest.NewRequest("GET", "/api/alerts/sensor/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid sensor ID")
}

func TestGetAlertRulesBySensorId_ServiceError(t *testing.T) {
	mockService := new(mockAlertManagementService)
	s := &Server{alertService: mockService}

	mockService.On("ServiceGetAlertRulesBySensorID", mock.Anything, 1).Return([]alerting.AlertRule{}, fmt.Errorf("db error"))

	router := setupAlertsBySensorRoute(s)
	req := httptest.NewRequest("GET", "/api/alerts/sensor/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Error fetching alert rules")
	mockService.AssertExpectations(t)
}

// ─── CreateAlertRule ─────────────────────────────────────────────────────────

func TestCreateAlertRule(t *testing.T) {
	mockService := new(mockAlertManagementService)
	s := &Server{alertService: mockService}

	newRule := gen.AlertRule{
		SensorID:          1,
		MeasurementTypeID: 1,
		AlertType:         gen.NumericRange,
		HighThreshold:     30.0,
		LowThreshold:      10.0,
		RateLimitSeconds:  1,
		Enabled:           true,
	}

	mockService.On("ServiceCreateAlertRule", mock.Anything, mock.AnythingOfType("*alerting.AlertRule")).Return(nil)

	router := gin.New()
	router.Group("/api").POST("/alerts", s.CreateAlertRule)

	body, _ := json.Marshal(newRule)
	req := httptest.NewRequest("POST", "/api/alerts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "Alert rule created successfully")
	mockService.AssertExpectations(t)
}

func TestCreateAlertRule_InvalidJSON(t *testing.T) {
	s := new(Server)
	router := gin.New()
	router.Group("/api").POST("/alerts", s.CreateAlertRule)

	req := httptest.NewRequest("POST", "/api/alerts", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid request body")
}

func TestCreateAlertRule_ValidationError(t *testing.T) {
	s := new(Server)
	router := gin.New()
	router.Group("/api").POST("/alerts", s.CreateAlertRule)

	invalidRule := gen.AlertRule{
		SensorID:      1,
		AlertType:     gen.NumericRange,
		HighThreshold: 10.0, // lower than low threshold
		LowThreshold:  30.0,
	}

	body, _ := json.Marshal(invalidRule)
	req := httptest.NewRequest("POST", "/api/alerts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid alert rule")
}

func TestCreateAlertRule_InvalidAlertType(t *testing.T) {
	s := new(Server)
	router := gin.New()
	router.Group("/api").POST("/alerts", s.CreateAlertRule)

	invalidRule := gen.AlertRule{
		SensorID:          1,
		MeasurementTypeID: 1,
		AlertType:         "invalid_type",
		HighThreshold:     30.0,
		LowThreshold:      10.0,
		RateLimitSeconds:  1,
		Enabled:           true,
	}

	body, _ := json.Marshal(invalidRule)
	req := httptest.NewRequest("POST", "/api/alerts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid alert rule")
}

func TestCreateAlertRule_ServiceError(t *testing.T) {
	mockService := new(mockAlertManagementService)
	s := &Server{alertService: mockService}

	newRule := gen.AlertRule{
		SensorID:          1,
		MeasurementTypeID: 1,
		AlertType:         gen.NumericRange,
		HighThreshold:     30.0,
		LowThreshold:      10.0,
		RateLimitSeconds:  1,
		Enabled:           true,
	}

	mockService.On("ServiceCreateAlertRule", mock.Anything, mock.AnythingOfType("*alerting.AlertRule")).Return(fmt.Errorf("db error"))

	router := gin.New()
	router.Group("/api").POST("/alerts", s.CreateAlertRule)

	body, _ := json.Marshal(newRule)
	req := httptest.NewRequest("POST", "/api/alerts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockService.AssertExpectations(t)
}

// ─── UpdateAlertRule ─────────────────────────────────────────────────────────

func TestUpdateAlertRule(t *testing.T) {
	mockService := new(mockAlertManagementService)
	s := &Server{alertService: mockService}

	updatedRule := gen.AlertRule{
		SensorID:          1,
		MeasurementTypeID: 1,
		AlertType:         gen.NumericRange,
		HighThreshold:     35.0,
		LowThreshold:      12.0,
		RateLimitSeconds:  2,
		Enabled:           false,
	}

	existing := &alerting.AlertRule{ID: 1, SensorID: 1, MeasurementTypeId: 1, AlertType: alerting.AlertTypeNumericRange, HighThreshold: 30, LowThreshold: 10}
	mockService.On("ServiceGetAlertRuleByID", mock.Anything, 1).Return(existing, nil)
	mockService.On("ServiceUpdateAlertRule", mock.Anything, mock.AnythingOfType("*alerting.AlertRule")).Return(nil)

	router := setupUpdateAlertRoute(s)

	body, _ := json.Marshal(updatedRule)
	req := httptest.NewRequest("PUT", "/api/alerts/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Alert rule updated successfully")
	mockService.AssertExpectations(t)
}

func TestUpdateAlertRule_InvalidID(t *testing.T) {
	s := new(Server)
	router := setupUpdateAlertRoute(s)

	req := httptest.NewRequest("PUT", "/api/alerts/invalid", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid alert rule ID")
}

func TestUpdateAlertRule_InvalidJSON(t *testing.T) {
	s := new(Server)
	router := setupUpdateAlertRoute(s)

	req := httptest.NewRequest("PUT", "/api/alerts/1", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid request body")
}

func TestUpdateAlertRule_ValidationError(t *testing.T) {
	mockService := new(mockAlertManagementService)
	s := &Server{alertService: mockService}

	existing := &alerting.AlertRule{ID: 1, SensorID: 1, MeasurementTypeId: 1, AlertType: alerting.AlertTypeNumericRange, HighThreshold: 30, LowThreshold: 10}
	mockService.On("ServiceGetAlertRuleByID", mock.Anything, 1).Return(existing, nil)

	router := setupUpdateAlertRoute(s)

	invalidRule := gen.AlertRule{
		AlertType:        gen.NumericRange,
		HighThreshold:    10.0,
		LowThreshold:     30.0,
		RateLimitSeconds: -1,
		Enabled:          true,
	}

	body, _ := json.Marshal(invalidRule)
	req := httptest.NewRequest("PUT", "/api/alerts/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid alert rule")
}

func TestUpdateAlertRule_ServiceError(t *testing.T) {
	mockService := new(mockAlertManagementService)
	s := &Server{alertService: mockService}

	validRule := gen.AlertRule{
		SensorID:          1,
		MeasurementTypeID: 1,
		AlertType:         gen.NumericRange,
		HighThreshold:     30.0,
		LowThreshold:      10.0,
		RateLimitSeconds:  1,
		Enabled:           true,
	}

	existing := &alerting.AlertRule{ID: 1, SensorID: 1, MeasurementTypeId: 1, AlertType: alerting.AlertTypeNumericRange, HighThreshold: 30, LowThreshold: 10}
	mockService.On("ServiceGetAlertRuleByID", mock.Anything, 1).Return(existing, nil)
	mockService.On("ServiceUpdateAlertRule", mock.Anything, mock.AnythingOfType("*alerting.AlertRule")).Return(fmt.Errorf("db error"))

	router := setupUpdateAlertRoute(s)

	body, _ := json.Marshal(validRule)
	req := httptest.NewRequest("PUT", "/api/alerts/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockService.AssertExpectations(t)
}

func TestUpdateAlertRule_PreservesImmutableIDsFromExistingRule(t *testing.T) {
	// Regression for #51: clients editing an alert rule send only the mutable
	// fields (thresholds, rate limit, enabled, etc.). The server must look up
	// the existing rule and preserve SensorID and MeasurementTypeID, which are
	// immutable for the lifetime of an Alert Rule.
	mockService := new(mockAlertManagementService)
	s := &Server{alertService: mockService}

	existingRule := &alerting.AlertRule{
		ID:                1,
		SensorID:          42,
		MeasurementTypeId: 7,
		AlertType:         alerting.AlertTypeNumericRange,
		HighThreshold:     30.0,
		LowThreshold:      10.0,
		Enabled:           true,
	}
	mockService.On("ServiceGetAlertRuleByID", mock.Anything, 1).Return(existingRule, nil)

	// Body omits SensorID and MeasurementTypeID — exactly what EditAlertDialog sends.
	bodyOnlyMutable := map[string]any{
		"AlertType":        "numeric_range",
		"HighThreshold":    35.0,
		"LowThreshold":     12.0,
		"RateLimitSeconds": 60,
		"Enabled":          false,
	}

	var captured *alerting.AlertRule
	mockService.On("ServiceUpdateAlertRule", mock.Anything, mock.AnythingOfType("*alerting.AlertRule")).
		Run(func(args mock.Arguments) { captured = args.Get(1).(*alerting.AlertRule) }).
		Return(nil)

	router := setupUpdateAlertRoute(s)
	body, _ := json.Marshal(bodyOnlyMutable)
	req := httptest.NewRequest("PUT", "/api/alerts/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	if assert.NotNil(t, captured) {
		assert.Equal(t, 1, captured.ID)
		assert.Equal(t, 42, captured.SensorID, "SensorID must be preserved from existing rule")
		assert.Equal(t, 7, captured.MeasurementTypeId, "MeasurementTypeId must be preserved from existing rule")
		assert.Equal(t, 35.0, captured.HighThreshold)
		assert.Equal(t, 12.0, captured.LowThreshold)
		assert.Equal(t, 60, captured.RateLimitSeconds)
		assert.False(t, captured.Enabled)
	}
	mockService.AssertExpectations(t)
}

func TestUpdateAlertRule_NotFound(t *testing.T) {
	mockService := new(mockAlertManagementService)
	s := &Server{alertService: mockService}

	mockService.On("ServiceGetAlertRuleByID", mock.Anything, 999).Return(nil, nil)

	router := setupUpdateAlertRoute(s)
	body, _ := json.Marshal(map[string]any{"AlertType": "numeric_range", "HighThreshold": 30.0, "LowThreshold": 10.0})
	req := httptest.NewRequest("PUT", "/api/alerts/999", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockService.AssertExpectations(t)
}

// ─── DeleteAlertRule ─────────────────────────────────────────────────────────

func TestDeleteAlertRule(t *testing.T) {
	mockService := new(mockAlertManagementService)
	s := &Server{alertService: mockService}

	mockService.On("ServiceDeleteAlertRule", mock.Anything, 1).Return(nil)

	router := setupDeleteAlertRoute(s)
	req := httptest.NewRequest("DELETE", "/api/alerts/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Alert rule deleted successfully")
	mockService.AssertExpectations(t)
}

func TestDeleteAlertRule_InvalidID(t *testing.T) {
	s := new(Server)
	router := setupDeleteAlertRoute(s)

	req := httptest.NewRequest("DELETE", "/api/alerts/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid alert rule ID")
}

func TestDeleteAlertRule_ServiceError(t *testing.T) {
	mockService := new(mockAlertManagementService)
	s := &Server{alertService: mockService}

	mockService.On("ServiceDeleteAlertRule", mock.Anything, 1).Return(fmt.Errorf("db error"))

	router := setupDeleteAlertRoute(s)
	req := httptest.NewRequest("DELETE", "/api/alerts/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockService.AssertExpectations(t)
}

// ─── GetAlertHistory ─────────────────────────────────────────────────────────

func TestGetAlertHistory(t *testing.T) {
	mockService := new(mockAlertManagementService)
	s := &Server{alertService: mockService}

	expectedHistory := []gen.AlertHistoryEntry{
		{SensorId: 1, AlertType: "numeric_range", ReadingValue: "35.5", SentAt: time.Now()},
		{SensorId: 1, AlertType: "numeric_range", ReadingValue: "40.0", SentAt: time.Now().Add(-2 * time.Hour)},
	}

	mockService.On("ServiceGetAlertHistory", mock.Anything, 1, 10).Return(expectedHistory, nil)

	router := setupAlertHistoryRoute(s)
	req := httptest.NewRequest("GET", "/api/alerts/sensor/1/history?limit=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "35.5")
	assert.Contains(t, w.Body.String(), "numeric_range")
	mockService.AssertExpectations(t)
}

func TestGetAlertHistory_DefaultLimit(t *testing.T) {
	mockService := new(mockAlertManagementService)
	s := &Server{alertService: mockService}

	mockService.On("ServiceGetAlertHistory", mock.Anything, 1, 50).Return([]gen.AlertHistoryEntry{}, nil)

	router := setupAlertHistoryRoute(s)
	req := httptest.NewRequest("GET", "/api/alerts/sensor/1/history", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockService.AssertExpectations(t)
}

func TestGetAlertHistory_InvalidSensorID(t *testing.T) {
	s := new(Server)
	router := setupAlertHistoryRoute(s)

	req := httptest.NewRequest("GET", "/api/alerts/sensor/invalid/history", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid sensor ID")
}

func TestGetAlertHistory_ServiceError(t *testing.T) {
	mockService := new(mockAlertManagementService)
	s := &Server{alertService: mockService}

	mockService.On("ServiceGetAlertHistory", mock.Anything, 1, 50).Return([]gen.AlertHistoryEntry{}, fmt.Errorf("db error"))

	router := setupAlertHistoryRoute(s)
	req := httptest.NewRequest("GET", "/api/alerts/sensor/1/history", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockService.AssertExpectations(t)
}
