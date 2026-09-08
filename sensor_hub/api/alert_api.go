package api

import (
	"example/sensorHub/alerting"
	gen "example/sensorHub/gen"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func toAlertingRule(r gen.AlertRule) alerting.AlertRule {
	return alerting.AlertRule{
		ID:                r.ID,
		SensorID:          r.SensorID,
		SensorName:        r.SensorName,
		MeasurementTypeId: r.MeasurementTypeID,
		MeasurementType:   r.MeasurementType,
		AlertType:         alerting.AlertType(r.AlertType),
		HighThreshold:     r.HighThreshold,
		LowThreshold:      r.LowThreshold,
		TriggerStatus:     r.TriggerStatus,
		Enabled:           r.Enabled,
		RateLimitSeconds:  r.RateLimitSeconds,
		LastAlertSentAt:   r.LastAlertSentAt,
	}
}

func (s *Server) GetAllAlertRules(c *gin.Context) {
	ctx := c.Request.Context()
	rules, err := s.alertService.ServiceGetAllAlertRules(ctx)
	if err != nil {
		slog.Error("error fetching alert rules", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error fetching alert rules", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (s *Server) GetAlertRuleById(c *gin.Context, id int) {
	ctx := c.Request.Context()
	rule, err := s.alertService.ServiceGetAlertRuleByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error fetching alert rule", "error": err.Error()})
		return
	}
	if rule == nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Alert rule not found"})
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (s *Server) GetAlertRulesBySensorId(c *gin.Context, sensorId int) {
	ctx := c.Request.Context()
	rules, err := s.alertService.ServiceGetAlertRulesBySensorID(ctx, sensorId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error fetching alert rules", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (s *Server) CreateAlertRule(c *gin.Context) {
	ctx := c.Request.Context()
	var genRule gen.AlertRule
	if err := c.ShouldBindJSON(&genRule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body", "error": err.Error()})
		return
	}

	rule := toAlertingRule(genRule)
	if err := rule.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid alert rule", "error": err.Error()})
		return
	}

	if err := s.alertService.ServiceCreateAlertRule(ctx, &rule); err != nil {
		slog.Error("error creating alert rule", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error creating alert rule", "error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Alert rule created successfully"})
}

func (s *Server) UpdateAlertRule(c *gin.Context, id int) {
	ctx := c.Request.Context()
	var genRule gen.AlertRule
	if err := c.ShouldBindJSON(&genRule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body", "error": err.Error()})
		return
	}

	existing, err := s.alertService.ServiceGetAlertRuleByID(ctx, id)
	if err != nil {
		slog.Error("error fetching alert rule for update", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error fetching alert rule", "error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Alert rule not found"})
		return
	}

	rule := toAlertingRule(genRule)
	rule.ID = id
	// SensorID and MeasurementTypeId are immutable for the lifetime of an Alert Rule.
	// Preserve them from the existing rule so clients only need to send mutable fields.
	rule.SensorID = existing.SensorID
	rule.MeasurementTypeId = existing.MeasurementTypeId

	if err := rule.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid alert rule", "error": err.Error()})
		return
	}

	if err := s.alertService.ServiceUpdateAlertRule(ctx, &rule); err != nil {
		slog.Error("error updating alert rule", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error updating alert rule", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Alert rule updated successfully"})
}

func (s *Server) DeleteAlertRule(c *gin.Context, id int) {
	ctx := c.Request.Context()
	if err := s.alertService.ServiceDeleteAlertRule(ctx, id); err != nil {
		slog.Error("error deleting alert rule", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error deleting alert rule", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Alert rule deleted successfully"})
}

func (s *Server) GetAlertHistory(c *gin.Context, sensorId int, params gen.GetAlertHistoryParams) {
	ctx := c.Request.Context()
	limit := 50
	if params.Limit != nil {
		limit = *params.Limit
	}

	history, err := s.alertService.ServiceGetAlertHistory(ctx, sensorId, limit)
	if err != nil {
		slog.Error("error fetching alert history", "sensor_id", sensorId, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error fetching alert history", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, history)
}
