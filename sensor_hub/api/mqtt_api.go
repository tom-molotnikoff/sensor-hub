package api

import (
	"net/http"
	"strings"

	gen "example/sensorHub/gen"
	mqttpkg "example/sensorHub/mqtt"

	"github.com/gin-gonic/gin"
)

// MQTTStatsProvider is the subset of ConnectionManager the API layer needs.
type MQTTStatsProvider interface {
	Stats() map[int]mqttpkg.BrokerStats
	IsConnected(brokerID int) bool
}

// isValidationError returns true if the error is a validation error from the
// service layer rather than an infrastructure failure. This is used to return
// 400 Bad Request instead of 500 Internal Server Error.
func isValidationError(err error) bool {
	msg := err.Error()
	validationPrefixes := []string{
		"broker name", "broker host", "broker port", "broker type", "broker id",
		"topic pattern", "driver type", "driver ", "unknown driver",
		"subscription id", "broker not found",
		"multi-level wildcard",
		"broker host:port",
	}
	for _, prefix := range validationPrefixes {
		if strings.HasPrefix(msg, prefix) {
			return true
		}
	}
	return false
}

func isNotFoundError(err error) bool {
	return strings.Contains(err.Error(), "no MQTT broker found") ||
		strings.Contains(err.Error(), "no MQTT subscription found")
}

func isDuplicateError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "already in use")
}

// ============================================================================
// Broker handlers
// ============================================================================

func (s *Server) ListMqttBrokers(c *gin.Context) {
	ctx := c.Request.Context()

	brokers, err := s.mqttService.GetAllBrokers(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error listing brokers"})
		return
	}
	if brokers == nil {
		brokers = []gen.MQTTBroker{}
	}
	c.JSON(http.StatusOK, brokers)
}

func (s *Server) GetMqttBroker(c *gin.Context, id int) {
	ctx := c.Request.Context()

	broker, err := s.mqttService.GetBrokerByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error getting broker"})
		return
	}
	if broker == nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Broker not found"})
		return
	}
	c.JSON(http.StatusOK, broker)
}

func (s *Server) CreateMqttBroker(c *gin.Context) {
	ctx := c.Request.Context()

	var broker gen.MQTTBroker
	if err := c.BindJSON(&broker); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}

	id, err := s.mqttService.AddBroker(ctx, broker)
	if err != nil {
		if isDuplicateError(err) {
			c.JSON(http.StatusConflict, gin.H{"message": "A broker with that name already exists"})
			return
		}
		if isValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (s *Server) UpdateMqttBroker(c *gin.Context, id int) {
	ctx := c.Request.Context()

	var broker gen.MQTTBroker
	if err := c.BindJSON(&broker); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}
	broker.Id = &id

	if err := s.mqttService.UpdateBroker(ctx, broker); err != nil {
		if isValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Broker updated"})
}

func (s *Server) DeleteMqttBroker(c *gin.Context, id int) {
	ctx := c.Request.Context()

	if err := s.mqttService.DeleteBroker(ctx, id); err != nil {
		if isNotFoundError(err) {
			c.JSON(http.StatusNotFound, gin.H{"message": "Broker not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ============================================================================
// Subscription handlers
// ============================================================================

func (s *Server) ListMqttSubscriptions(c *gin.Context, params gen.ListMqttSubscriptionsParams) {
	ctx := c.Request.Context()

	if params.BrokerId != nil {
		subs, err := s.mqttService.GetSubscriptionsByBrokerID(ctx, *params.BrokerId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error listing subscriptions"})
			return
		}
		if subs == nil {
			subs = []gen.MQTTSubscription{}
		}
		c.JSON(http.StatusOK, subs)
		return
	}

	subs, err := s.mqttService.GetAllSubscriptions(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error listing subscriptions"})
		return
	}
	if subs == nil {
		subs = []gen.MQTTSubscription{}
	}
	c.JSON(http.StatusOK, subs)
}

func (s *Server) GetMqttSubscription(c *gin.Context, id int) {
	ctx := c.Request.Context()

	sub, err := s.mqttService.GetSubscriptionByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error getting subscription"})
		return
	}
	if sub == nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Subscription not found"})
		return
	}
	c.JSON(http.StatusOK, sub)
}

func (s *Server) CreateMqttSubscription(c *gin.Context) {
	ctx := c.Request.Context()

	var sub gen.MQTTSubscription
	if err := c.BindJSON(&sub); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}

	id, err := s.mqttService.AddSubscription(ctx, sub)
	if err != nil {
		if isValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (s *Server) UpdateMqttSubscription(c *gin.Context, id int) {
	ctx := c.Request.Context()

	var sub gen.MQTTSubscription
	if err := c.BindJSON(&sub); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}
	sub.Id = &id

	if err := s.mqttService.UpdateSubscription(ctx, sub); err != nil {
		if isValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Subscription updated"})
}

func (s *Server) DeleteMqttSubscription(c *gin.Context, id int) {
	ctx := c.Request.Context()

	if err := s.mqttService.DeleteSubscription(ctx, id); err != nil {
		if isNotFoundError(err) {
			c.JSON(http.StatusNotFound, gin.H{"message": "Subscription not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ============================================================================
// Stats handler
// ============================================================================

func (s *Server) GetMqttStats(c *gin.Context) {
	if s.mqttStatsProvider == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"message": "MQTT stats not available"})
		return
	}

	statsMap := s.mqttStatsProvider.Stats()

	result := make([]mqttpkg.BrokerStats, 0, len(statsMap))
	for _, bs := range statsMap {
		result = append(result, bs)
	}

	c.JSON(http.StatusOK, result)
}
