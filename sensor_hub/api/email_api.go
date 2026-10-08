package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"example/sensorHub/email"
	gen "example/sensorHub/gen"

	"github.com/gin-gonic/gin"
)

// EmailServiceInterface is what the API needs from the email service.
type EmailServiceInterface interface {
	Settings(ctx context.Context) (gen.EmailSettings, error)
	Update(ctx context.Context, settings gen.EmailSettings) (gen.EmailSettings, error)
	SendTest(ctx context.Context, recipient string) error
}

func (s *Server) GetEmailSettings(c *gin.Context) {
	settings, err := s.emailService.Settings(c.Request.Context())
	if err != nil {
		slog.Error("Error reading the email settings", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error reading the email settings"})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (s *Server) UpdateEmailSettings(c *gin.Context) {
	var input gen.UpdateEmailSettingsJSONRequestBody
	if err := c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}
	settings, err := s.emailService.Update(c.Request.Context(), input)
	if errors.Is(err, email.ErrInvalidSettings) {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if err != nil {
		slog.Error("Error saving the email settings", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error saving the email settings"})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (s *Server) SendTestEmail(c *gin.Context) {
	current, _ := c.Get("currentUser")
	user, ok := current.(*gen.User)
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Not authenticated"})
		return
	}
	err := s.emailService.SendTest(c.Request.Context(), user.Email)
	var sendErr *email.SendError
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{"message": "Test email sent to " + user.Email})
	case errors.Is(err, email.ErrNoRecipient), errors.Is(err, email.ErrNotConfigured):
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
	case errors.As(err, &sendErr):
		c.JSON(http.StatusBadGateway, gin.H{"message": sendErr.Error()})
	default:
		slog.Error("Error sending a test email", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error sending a test email"})
	}
}
