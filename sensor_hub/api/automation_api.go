package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"example/sensorHub/automation"
	gen "example/sensorHub/gen"

	"github.com/gin-gonic/gin"
)

type AutomationServiceInterface interface {
	List(ctx context.Context) ([]gen.Automation, error)
	Get(ctx context.Context, id int) (gen.Automation, error)
	Create(ctx context.Context, input gen.AutomationInput) (gen.Automation, error)
	Update(ctx context.Context, id int, input gen.AutomationInput) (gen.Automation, error)
	SetEnabled(ctx context.Context, id int, enabled bool) (gen.Automation, error)
	Delete(ctx context.Context, id int) error
	Runs(ctx context.Context, id int) ([]gen.AutomationRun, error)
}

func (s *Server) ListAutomations(c *gin.Context) {
	automations, err := s.automationService.List(c.Request.Context())
	if err != nil {
		respondAutomationError(c, "Error fetching automations", err)
		return
	}
	c.JSON(http.StatusOK, automations)
}

func (s *Server) GetAutomation(c *gin.Context, id int) {
	automation, err := s.automationService.Get(c.Request.Context(), id)
	if err != nil {
		respondAutomationError(c, "Error fetching automation", err)
		return
	}
	c.JSON(http.StatusOK, automation)
}

func (s *Server) CreateAutomation(c *gin.Context) {
	var input gen.AutomationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body: " + err.Error()})
		return
	}
	automation, err := s.automationService.Create(c.Request.Context(), input)
	if err != nil {
		respondAutomationError(c, "Error creating automation", err)
		return
	}
	c.JSON(http.StatusCreated, automation)
}

func (s *Server) UpdateAutomation(c *gin.Context, id int) {
	var input gen.AutomationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body: " + err.Error()})
		return
	}
	automation, err := s.automationService.Update(c.Request.Context(), id, input)
	if err != nil {
		respondAutomationError(c, "Error updating automation", err)
		return
	}
	c.JSON(http.StatusOK, automation)
}

func (s *Server) SetAutomationEnabled(c *gin.Context, id int) {
	var body gen.SetAutomationEnabledRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body: " + err.Error()})
		return
	}
	automation, err := s.automationService.SetEnabled(c.Request.Context(), id, body.Enabled)
	if err != nil {
		respondAutomationError(c, "Error switching automation", err)
		return
	}
	c.JSON(http.StatusOK, automation)
}

func (s *Server) DeleteAutomation(c *gin.Context, id int) {
	if err := s.automationService.Delete(c.Request.Context(), id); err != nil {
		respondAutomationError(c, "Error deleting automation", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Automation deleted"})
}

func (s *Server) ListAutomationRuns(c *gin.Context, id int) {
	runs, err := s.automationService.Runs(c.Request.Context(), id)
	if err != nil {
		respondAutomationError(c, "Error fetching automation runs", err)
		return
	}
	c.JSON(http.StatusOK, runs)
}

func respondAutomationError(c *gin.Context, message string, err error) {
	var invalid *automation.ValidationError
	switch {
	case errors.As(err, &invalid):
		c.JSON(http.StatusBadRequest, gin.H{"message": invalid.Message})
	case errors.Is(err, automation.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"message": "Automation not found"})
	default:
		slog.Error(message, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": message, "error": err.Error()})
	}
}
