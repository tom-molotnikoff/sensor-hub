package api

import (
	"errors"
	appProps "example/sensorHub/application_properties"
	gen "example/sensorHub/gen"
	"example/sensorHub/ws"
	"fmt"
	"net/http"
	"reflect"

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
		var vErr *appProps.ValidationError
		if errors.As(err, &vErr) {
			c.IndentedJSON(http.StatusBadRequest, gen.PropertiesErrorResponse{Message: vErr.Message, Key: &vErr.Key})
			return
		}
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

// GetPropertyDefinitions serves the registry metadata. The data is static
// registry state, so it comes straight from appProps with no service between.
func (s *Server) GetPropertyDefinitions(c *gin.Context) {
	defs := appProps.Definitions()
	groups := appProps.PropertyGroups()

	resp := gen.PropertyDefinitionsResponse{
		Definitions: make([]gen.PropertyDefinition, 0, len(defs)),
		Groups:      make([]gen.PropertyGroup, 0, len(groups)),
	}

	for _, def := range defs {
		d := gen.PropertyDefinition{
			Key:         def.Key,
			Label:       def.Label,
			Description: def.Description,
			Type:        definitionType(def.Kind),
			Default:     def.Default,
			Group:       def.Group,
			Apply:       string(def.Apply),
			ReadOnly:    def.ReadOnly,
		}
		if def.Unit != "" {
			d.Unit = &def.Unit
		}
		if len(def.Enum) > 0 {
			d.Enum = &def.Enum
		}
		if def.Validate != "" {
			d.Validate = &def.Validate
		}
		resp.Definitions = append(resp.Definitions, d)
	}

	for _, g := range groups {
		resp.Groups = append(resp.Groups, gen.PropertyGroup{
			Id:          g.ID,
			Label:       g.Label,
			Description: g.Description,
			Order:       g.Order,
		})
	}

	c.IndentedJSON(http.StatusOK, resp)
}

func definitionType(kind reflect.Kind) gen.PropertyDefinitionType {
	switch kind {
	case reflect.Int:
		return gen.Int
	case reflect.Bool:
		return gen.Bool
	default:
		return gen.String
	}
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
