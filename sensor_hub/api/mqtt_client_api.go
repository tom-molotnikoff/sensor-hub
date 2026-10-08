package api

import (
	"errors"
	"log/slog"
	"net/http"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/service"

	"github.com/gin-gonic/gin"
)

func (s *Server) ListMqttClients(c *gin.Context) {
	clients, err := s.mqttClientService.List(c.Request.Context())
	if err != nil {
		respondMQTTClientError(c, "Error listing MQTT clients", err)
		return
	}
	c.JSON(http.StatusOK, clients)
}

func (s *Server) GetMqttClient(c *gin.Context, id int) {
	client, err := s.mqttClientService.Get(c.Request.Context(), id)
	if err != nil {
		respondMQTTClientError(c, "Error getting MQTT client", err)
		return
	}
	c.JSON(http.StatusOK, client)
}

func (s *Server) CreateMqttClient(c *gin.Context) {
	var input gen.MQTTClientInput
	if err := c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}
	created, err := s.mqttClientService.Create(c.Request.Context(), input)
	if err != nil {
		respondMQTTClientError(c, "Error creating MQTT client", err)
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (s *Server) UpdateMqttClient(c *gin.Context, id int) {
	var input gen.MQTTClientInput
	if err := c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}
	client, err := s.mqttClientService.Update(c.Request.Context(), id, input)
	if err != nil {
		respondMQTTClientError(c, "Error updating MQTT client", err)
		return
	}
	c.JSON(http.StatusOK, client)
}

func (s *Server) RotateMqttClientPassword(c *gin.Context, id int) {
	rotated, err := s.mqttClientService.RotatePassword(c.Request.Context(), id)
	if err != nil {
		respondMQTTClientError(c, "Error rotating MQTT client password", err)
		return
	}
	c.JSON(http.StatusOK, rotated)
}

func (s *Server) DeleteMqttClient(c *gin.Context, id int) {
	if err := s.mqttClientService.Delete(c.Request.Context(), id); err != nil {
		respondMQTTClientError(c, "Error deleting MQTT client", err)
		return
	}
	c.Status(http.StatusNoContent)
}

func respondMQTTClientError(c *gin.Context, message string, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidMQTTClient):
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
	case errors.Is(err, database.ErrMQTTClientNotFound):
		c.JSON(http.StatusNotFound, gin.H{"message": "MQTT client not found"})
	case errors.Is(err, database.ErrMQTTClientNameTaken):
		c.JSON(http.StatusConflict, gin.H{"message": "An MQTT client with that name already exists"})
	default:
		slog.Error(message, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": message})
	}
}
