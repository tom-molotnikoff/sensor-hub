package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	gen "example/sensorHub/gen"
	mqttpkg "example/sensorHub/mqtt"
	"example/sensorHub/service"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ============================================================================
// Mock MQTT service
// ============================================================================

type mockMQTTService struct{ mock.Mock }

func (m *mockMQTTService) AddBroker(ctx context.Context, broker gen.MQTTBroker) (int, error) {
	args := m.Called(ctx, broker)
	return args.Int(0), args.Error(1)
}
func (m *mockMQTTService) GetBrokerByID(ctx context.Context, id int) (*gen.MQTTBroker, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.MQTTBroker), args.Error(1)
}
func (m *mockMQTTService) GetBrokerByName(ctx context.Context, name string) (*gen.MQTTBroker, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.MQTTBroker), args.Error(1)
}
func (m *mockMQTTService) GetAllBrokers(ctx context.Context) ([]gen.MQTTBroker, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]gen.MQTTBroker), args.Error(1)
}
func (m *mockMQTTService) GetEnabledBrokers(ctx context.Context) ([]gen.MQTTBroker, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]gen.MQTTBroker), args.Error(1)
}
func (m *mockMQTTService) UpdateBroker(ctx context.Context, broker gen.MQTTBroker) error {
	return m.Called(ctx, broker).Error(0)
}
func (m *mockMQTTService) DeleteBroker(ctx context.Context, id int) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockMQTTService) AddSubscription(ctx context.Context, sub gen.MQTTSubscription) (int, error) {
	args := m.Called(ctx, sub)
	return args.Int(0), args.Error(1)
}
func (m *mockMQTTService) GetSubscriptionByID(ctx context.Context, id int) (*gen.MQTTSubscription, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.MQTTSubscription), args.Error(1)
}
func (m *mockMQTTService) GetAllSubscriptions(ctx context.Context) ([]gen.MQTTSubscription, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]gen.MQTTSubscription), args.Error(1)
}
func (m *mockMQTTService) GetSubscriptionsByBrokerID(ctx context.Context, brokerID int) ([]gen.MQTTSubscription, error) {
	args := m.Called(ctx, brokerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]gen.MQTTSubscription), args.Error(1)
}
func (m *mockMQTTService) GetEnabledSubscriptionsByBrokerID(ctx context.Context, brokerID int) ([]gen.MQTTSubscription, error) {
	args := m.Called(ctx, brokerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]gen.MQTTSubscription), args.Error(1)
}
func (m *mockMQTTService) UpdateSubscription(ctx context.Context, sub gen.MQTTSubscription) error {
	return m.Called(ctx, sub).Error(0)
}
func (m *mockMQTTService) DeleteSubscription(ctx context.Context, id int) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockMQTTService) SetSubscriptionNotifier(n service.SubscriptionNotifier) {}

// ============================================================================
// Mock MQTT stats provider
// ============================================================================

type mockMQTTStatsProvider struct{ mock.Mock }

func (m *mockMQTTStatsProvider) Stats() map[int]mqttpkg.BrokerStats {
	args := m.Called()
	return args.Get(0).(map[int]mqttpkg.BrokerStats)
}

func (m *mockMQTTStatsProvider) IsConnected(brokerID int) bool {
	args := m.Called(brokerID)
	return args.Bool(0)
}

// ============================================================================
// Test helpers
// ============================================================================

func setupMQTTRouter(method, path string, handler gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	apiGroup := router.Group("/api")
	apiGroup.Handle(method, path, handler)
	return router
}

func newMQTTMock() (*Server, *mockMQTTService) {
	m := new(mockMQTTService)
	s := &Server{mqttService: m}
	return s, m
}

func withBrokerID(s *Server, h func(*gin.Context, int)) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid broker ID"})
			return
		}
		h(c, id)
	}
}

func withSubscriptionID(s *Server, h func(*gin.Context, int)) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid subscription ID"})
			return
		}
		h(c, id)
	}
}

// listSubscriptionsClosureHandler simulates the route closure for ListMqttSubscriptions.
func listSubscriptionsClosureHandler(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var params gen.ListMqttSubscriptionsParams
		if brokerParam := c.Query("broker_id"); brokerParam != "" {
			id, err := strconv.Atoi(brokerParam)
			if err != nil {
				c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid broker_id parameter"})
				return
			}
			params.BrokerId = &id
		}
		s.ListMqttSubscriptions(c, params)
	}
}

// ============================================================================
// Broker tests
// ============================================================================

func TestListBrokersHandler_Success(t *testing.T) {
	s, svc := newMQTTMock()
	expected := []gen.MQTTBroker{{Id: ptrInt(1), Name: "b1"}, {Id: ptrInt(2), Name: "b2"}}
	svc.On("GetAllBrokers", mock.Anything).Return(expected, nil)

	router := setupMQTTRouter("GET", "/mqtt/brokers", s.ListMqttBrokers)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/mqtt/brokers", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "b1")
	svc.AssertExpectations(t)
}

func TestListBrokersHandler_Empty(t *testing.T) {
	s, svc := newMQTTMock()
	svc.On("GetAllBrokers", mock.Anything).Return(nil, nil)

	router := setupMQTTRouter("GET", "/mqtt/brokers", s.ListMqttBrokers)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/mqtt/brokers", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "[]", w.Body.String())
}

func TestListBrokersHandler_Error(t *testing.T) {
	s, svc := newMQTTMock()
	svc.On("GetAllBrokers", mock.Anything).Return(nil, errors.New("db error"))

	router := setupMQTTRouter("GET", "/mqtt/brokers", s.ListMqttBrokers)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/mqtt/brokers", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetBrokerHandler_Success(t *testing.T) {
	s, svc := newMQTTMock()
	broker := &gen.MQTTBroker{Id: ptrInt(1), Name: "test-broker", Host: "mqtt.local", Port: 1883}
	svc.On("GetBrokerByID", mock.Anything, 1).Return(broker, nil)

	router := setupMQTTRouter("GET", "/mqtt/brokers/:id", withBrokerID(s, s.GetMqttBroker))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/mqtt/brokers/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "test-broker")
}

func TestGetBrokerHandler_NotFound(t *testing.T) {
	s, svc := newMQTTMock()
	svc.On("GetBrokerByID", mock.Anything, 99).Return(nil, nil)

	router := setupMQTTRouter("GET", "/mqtt/brokers/:id", withBrokerID(s, s.GetMqttBroker))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/mqtt/brokers/99", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetBrokerHandler_InvalidID(t *testing.T) {
	s, _ := newMQTTMock()

	router := setupMQTTRouter("GET", "/mqtt/brokers/:id", withBrokerID(s, s.GetMqttBroker))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/mqtt/brokers/abc", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateBrokerHandler_Success(t *testing.T) {
	s, svc := newMQTTMock()
	svc.On("AddBroker", mock.Anything, mock.AnythingOfType("gen.MQTTBroker")).Return(1, nil)

	body, _ := json.Marshal(gen.MQTTBroker{Name: "new-broker", Type: "external", Host: "mqtt.local", Port: 1883})
	router := setupMQTTRouter("POST", "/mqtt/brokers", s.CreateMqttBroker)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/mqtt/brokers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestCreateBrokerHandler_InvalidBody(t *testing.T) {
	s, _ := newMQTTMock()

	router := setupMQTTRouter("POST", "/mqtt/brokers", s.CreateMqttBroker)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/mqtt/brokers", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateBrokerHandler_ServiceError(t *testing.T) {
	s, svc := newMQTTMock()
	svc.On("AddBroker", mock.Anything, mock.AnythingOfType("gen.MQTTBroker")).Return(0, errors.New("validation failed"))

	body, _ := json.Marshal(gen.MQTTBroker{Name: "bad"})
	router := setupMQTTRouter("POST", "/mqtt/brokers", s.CreateMqttBroker)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/mqtt/brokers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "validation failed")
}

func TestUpdateBrokerHandler_Success(t *testing.T) {
	s, svc := newMQTTMock()
	svc.On("UpdateBroker", mock.Anything, mock.AnythingOfType("gen.MQTTBroker")).Return(nil)

	body, _ := json.Marshal(gen.MQTTBroker{Name: "updated", Type: "external", Host: "mqtt.local", Port: 1883})
	router := setupMQTTRouter("PUT", "/mqtt/brokers/:id", withBrokerID(s, s.UpdateMqttBroker))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/mqtt/brokers/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestDeleteBrokerHandler_Success(t *testing.T) {
	s, svc := newMQTTMock()
	svc.On("DeleteBroker", mock.Anything, 1).Return(nil)

	router := setupMQTTRouter("DELETE", "/mqtt/brokers/:id", withBrokerID(s, s.DeleteMqttBroker))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/mqtt/brokers/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

// ============================================================================
// Subscription tests
// ============================================================================

func TestListSubscriptionsHandler_All(t *testing.T) {
	s, svc := newMQTTMock()
	expected := []gen.MQTTSubscription{{Id: ptrInt(1), TopicPattern: "zigbee2mqtt/+"}}
	svc.On("GetAllSubscriptions", mock.Anything).Return(expected, nil)

	router := setupMQTTRouter("GET", "/mqtt/subscriptions", listSubscriptionsClosureHandler(s))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/mqtt/subscriptions", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "zigbee2mqtt")
}

func TestListSubscriptionsHandler_ByBroker(t *testing.T) {
	s, svc := newMQTTMock()
	expected := []gen.MQTTSubscription{{Id: ptrInt(1), BrokerId: 2, TopicPattern: "rtl_433/+"}}
	svc.On("GetSubscriptionsByBrokerID", mock.Anything, 2).Return(expected, nil)

	router := setupMQTTRouter("GET", "/mqtt/subscriptions", listSubscriptionsClosureHandler(s))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/mqtt/subscriptions?broker_id=2", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "rtl_433")
}

func TestListSubscriptionsHandler_InvalidBrokerID(t *testing.T) {
	s, _ := newMQTTMock()

	router := setupMQTTRouter("GET", "/mqtt/subscriptions", listSubscriptionsClosureHandler(s))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/mqtt/subscriptions?broker_id=abc", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetSubscriptionHandler_Success(t *testing.T) {
	s, svc := newMQTTMock()
	sub := &gen.MQTTSubscription{Id: ptrInt(1), TopicPattern: "zigbee2mqtt/+", DriverType: "mqtt-zigbee2mqtt"}
	svc.On("GetSubscriptionByID", mock.Anything, 1).Return(sub, nil)

	router := setupMQTTRouter("GET", "/mqtt/subscriptions/:id", withSubscriptionID(s, s.GetMqttSubscription))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/mqtt/subscriptions/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "zigbee2mqtt")
}

func TestGetSubscriptionHandler_NotFound(t *testing.T) {
	s, svc := newMQTTMock()
	svc.On("GetSubscriptionByID", mock.Anything, 99).Return(nil, nil)

	router := setupMQTTRouter("GET", "/mqtt/subscriptions/:id", withSubscriptionID(s, s.GetMqttSubscription))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/mqtt/subscriptions/99", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreateSubscriptionHandler_Success(t *testing.T) {
	s, svc := newMQTTMock()
	svc.On("AddSubscription", mock.Anything, mock.AnythingOfType("gen.MQTTSubscription")).Return(1, nil)

	body, _ := json.Marshal(gen.MQTTSubscription{BrokerId: 1, TopicPattern: "zigbee2mqtt/+", DriverType: "mqtt-zigbee2mqtt"})
	router := setupMQTTRouter("POST", "/mqtt/subscriptions", s.CreateMqttSubscription)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/mqtt/subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestCreateSubscriptionHandler_ServiceError(t *testing.T) {
	s, svc := newMQTTMock()
	svc.On("AddSubscription", mock.Anything, mock.AnythingOfType("gen.MQTTSubscription")).Return(0, errors.New("driver not found"))

	body, _ := json.Marshal(gen.MQTTSubscription{BrokerId: 1, TopicPattern: "test/+", DriverType: "bad"})
	router := setupMQTTRouter("POST", "/mqtt/subscriptions", s.CreateMqttSubscription)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/mqtt/subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "driver not found")
}

func TestUpdateSubscriptionHandler_Success(t *testing.T) {
	s, svc := newMQTTMock()
	svc.On("UpdateSubscription", mock.Anything, mock.AnythingOfType("gen.MQTTSubscription")).Return(nil)

	body, _ := json.Marshal(gen.MQTTSubscription{BrokerId: 1, TopicPattern: "updated/+", DriverType: "mqtt-zigbee2mqtt"})
	router := setupMQTTRouter("PUT", "/mqtt/subscriptions/:id", withSubscriptionID(s, s.UpdateMqttSubscription))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/mqtt/subscriptions/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestDeleteSubscriptionHandler_Success(t *testing.T) {
	s, svc := newMQTTMock()
	svc.On("DeleteSubscription", mock.Anything, 1).Return(nil)

	router := setupMQTTRouter("DELETE", "/mqtt/subscriptions/:id", withSubscriptionID(s, s.DeleteMqttSubscription))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/mqtt/subscriptions/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

func TestValidateTopicPattern_ViaAPI(t *testing.T) {
	s, svc := newMQTTMock()
	svc.On("AddSubscription", mock.Anything, mock.AnythingOfType("gen.MQTTSubscription")).Return(0, errors.New("topic pattern must not contain spaces"))

	body, _ := json.Marshal(gen.MQTTSubscription{BrokerId: 1, TopicPattern: "bad topic", DriverType: "mqtt-test"})
	router := setupMQTTRouter("POST", "/mqtt/subscriptions", s.CreateMqttSubscription)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/mqtt/subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "topic pattern must not contain spaces")
}

// ============================================================================
// Stats tests
// ============================================================================

func TestGetMqttStatsHandler_Success(t *testing.T) {
	statsProvider := new(mockMQTTStatsProvider)
	s := &Server{mqttStatsProvider: statsProvider}

	statsMap := map[int]mqttpkg.BrokerStats{
		1: {BrokerID: 1, Connected: true},
	}
	statsProvider.On("Stats").Return(statsMap)

	router := setupMQTTRouter("GET", "/mqtt/stats", s.GetMqttStats)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/mqtt/stats", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "broker_id")
	statsProvider.AssertExpectations(t)
}

func TestGetMqttStatsHandler_Unavailable(t *testing.T) {
	s := &Server{mqttStatsProvider: nil}

	router := setupMQTTRouter("GET", "/mqtt/stats", s.GetMqttStats)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/mqtt/stats", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func ptrInt(i int) *int { return &i }
