package drivers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"example/sensorHub/gen"
	"example/sensorHub/utils"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func init() {
	Register(&SensorHubHTTPTemperature{
		client: &http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
			Timeout:   10 * time.Second,
		},
	})
}

// SensorHubHTTPTemperature is the driver for the built-in Sensor Hub HTTP temperature sensors.
type SensorHubHTTPTemperature struct {
	client *http.Client
}

// Compile-time check: SensorHubHTTPTemperature satisfies PullDriver.
var _ PullDriver = (*SensorHubHTTPTemperature)(nil)

func (d *SensorHubHTTPTemperature) Type() string        { return "sensor-hub-http-temperature" }
func (d *SensorHubHTTPTemperature) DisplayName() string { return "Sensor Hub HTTP Temperature" }
func (d *SensorHubHTTPTemperature) Description() string {
	return "Built-in HTTP temperature sensor using the Sensor Hub protocol"
}

func (d *SensorHubHTTPTemperature) ConfigFields() []ConfigFieldSpec {
	return []ConfigFieldSpec{
		{Key: "url", Label: "Sensor URL", Description: "Base URL of the HTTP sensor (e.g. http://192.168.1.50:8080)", Required: true},
	}
}

func (d *SensorHubHTTPTemperature) SupportedMeasurementTypes() []gen.MeasurementType {
	return []gen.MeasurementType{
		{Name: "temperature", DisplayName: "Temperature", Unit: "°C", Category: "numeric"},
	}
}

type rawTempResponse struct {
	Temperature float64 `json:"temperature"`
	Time        string  `json:"time"`
}

func (d *SensorHubHTTPTemperature) CollectReadings(ctx context.Context, sensor gen.Sensor) ([]gen.Reading, error) {
	sensorURL := sensor.Config["url"]
	if sensorURL == "" {
		return nil, fmt.Errorf("sensor %s has no 'url' in config", sensor.Name)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", sensorURL+"/temperature", nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request to sensor at %s: %w", sensorURL, err)
	}
	response, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making GET request to sensor at %s: %w", sensorURL, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 response from sensor at %s: %d", sensorURL, response.StatusCode)
	}

	var raw rawTempResponse
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("error decoding JSON response from sensor at %s: %w", sensorURL, err)
	}

	raw.Time = utils.NormalizeTimeToSpaceFormat(raw.Time)
	temp := raw.Temperature

	return []gen.Reading{
		{
			SensorName:      sensor.Name,
			MeasurementType: "temperature",
			NumericValue:    &temp,
			Unit:            "°C",
			Time:            raw.Time,
		},
	}, nil
}

func (d *SensorHubHTTPTemperature) ValidateSensor(ctx context.Context, sensor gen.Sensor) error {
	_, err := d.CollectReadings(ctx, sensor)
	return err
}
