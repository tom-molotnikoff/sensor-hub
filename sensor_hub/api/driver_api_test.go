package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "example/sensorHub/drivers" // register built-in drivers
	gen "example/sensorHub/gen"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDriverRouter() *gin.Engine {
	return setupGenRouter(new(Server))
}

func TestListDrivers(t *testing.T) {
	router := setupDriverRouter()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/drivers", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result []gen.DriverInfo
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// Verify the built-in HTTP temperature driver is present
	var found *gen.DriverInfo
	for i, d := range result {
		if d.Type == "sensor-hub-http-temperature" {
			found = &result[i]
			break
		}
	}
	require.NotNil(t, found, "sensor-hub-http-temperature driver should be listed")
	assert.Equal(t, "Sensor Hub HTTP Temperature", found.DisplayName)
	require.NotNil(t, found.Description)
	assert.NotEmpty(t, *found.Description)
	require.NotNil(t, found.SupportedMeasurementTypes)
	assert.Contains(t, *found.SupportedMeasurementTypes, "temperature")
	require.NotEmpty(t, found.ConfigFields)
	assert.Equal(t, "url", found.ConfigFields[0].Key)
	assert.True(t, found.ConfigFields[0].Required)
}

func TestListDrivers_ResponseShape(t *testing.T) {
	router := setupDriverRouter()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/drivers", nil)
	router.ServeHTTP(w, req)

	// Verify the JSON contains expected keys
	var raw []map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &raw)
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	first := raw[0]
	assert.Contains(t, first, "type")
	assert.Contains(t, first, "display_name")
	assert.Contains(t, first, "description")
	assert.Contains(t, first, "supported_measurement_types")
	assert.Contains(t, first, "config_fields")
}
