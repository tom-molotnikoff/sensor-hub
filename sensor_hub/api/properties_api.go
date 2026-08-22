package api

import (
	gen "example/sensorHub/gen"
	"example/sensorHub/ws"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) UpdateProperties(c *gin.Context) {
	ctx := c.Request.Context()
	var requestBody gen.UpdatePropertiesJSONRequestBody

	if err := c.BindJSON(&requestBody); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}

	err := s.propertiesService.ServiceUpdateProperties(ctx, requestBody)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "Error updating properties", "error": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusAccepted, gin.H{"message": "Property updated successfully"})
}

func (s *Server) GetProperties(c *gin.Context) {
	ctx := c.Request.Context()
	properties, err := s.propertiesService.ServiceGetProperties(ctx)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "Error fetching properties", "error": err.Error()})
		return
	}

	result := make(gen.PropertiesMap)
	for k, v := range properties {
		result[k] = fmt.Sprintf("%v", v)
	}
	c.IndentedJSON(http.StatusOK, result)
}

func (s *Server) PropertiesWebSocket(c *gin.Context) {
	ctx := c.Request.Context()
	properties, err := s.propertiesService.ServiceGetProperties(ctx)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "Error fetching properties", "error": err.Error()})
		return
	}

	createPushWebSocket(c, "properties")

	result := make(gen.PropertiesMap)
	for k, v := range properties {
		result[k] = fmt.Sprintf("%v", v)
	}
	ws.BroadcastToTopic("properties", result)
}
