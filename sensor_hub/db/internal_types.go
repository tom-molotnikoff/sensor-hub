package database

import "time"

func cacheKeyForName(name string) string {
	lowered := []byte(name)
	for i := range lowered {
		if lowered[i] >= 'A' && lowered[i] <= 'Z' {
			lowered[i] += 'a' - 'A'
		}
	}
	return string(lowered)
}

// ============================================================================
// Table name constants
// ============================================================================

const (
	TableReadings                    = "readings"
	TableSensorHealthHistory         = "sensor_health_history"
	TableSensorCommandHistory        = "sensor_command_history"
	TableMeasurementTypes            = "measurement_types"
	TableSensorMeasurementTypes      = "sensor_measurement_types"
	TableMQTTBrokers                 = "mqtt_brokers"
	TableMQTTSubscriptions           = "mqtt_subscriptions"
	TableMeasurementTypeAggregations = "measurement_type_aggregations"
)

// ============================================================================
// Aggregation types — internal to the service/db boundary
// ============================================================================

// AggregationInterval represents the time-bucket size applied to readings.
// "raw" means no aggregation; otherwise an ISO 8601 duration like "PT5M".
type AggregationInterval string

const (
	AggregationRaw   AggregationInterval = "raw"
	AggregationPT10S AggregationInterval = "PT10S"
	AggregationPT1M  AggregationInterval = "PT1M"
	AggregationPT5M  AggregationInterval = "PT5M"
	AggregationPT15M AggregationInterval = "PT15M"
	AggregationPT1H  AggregationInterval = "PT1H"
	AggregationP1D   AggregationInterval = "P1D"
)

// AggregationFunction represents the SQL aggregate function applied within each bucket.
type AggregationFunction string

const (
	AggregationFunctionNone  AggregationFunction = "none"
	AggregationFunctionAvg   AggregationFunction = "avg"
	AggregationFunctionCount AggregationFunction = "count"
	AggregationFunctionLast  AggregationFunction = "last"
)

// ============================================================================
// Measurement type helpers — internal to the db/service boundary
// ============================================================================

// MeasurementTypeAggregation describes which aggregation functions are
// available for a given measurement type.
type MeasurementTypeAggregation struct {
	MeasurementType    string   `json:"measurement_type"`
	DefaultFunction    string   `json:"default_function"`
	SupportedFunctions []string `json:"supported_functions"`
}

// SensorMeasurementType links a sensor to one of its measurement types.
type SensorMeasurementType struct {
	SensorId          int    `json:"sensor_id"`
	MeasurementTypeId int    `json:"measurement_type_id"`
	MeasurementType   string `json:"measurement_type"`
	Unit              string `json:"unit"`
}

// ============================================================================
// Time helper — used when scanning nullable timestamps from the db
// ============================================================================

// toTimePtr converts a *time.Time pointer from database nulls back to a concrete value.
// Used to bridge the generated gen.MQTTBroker / gen.MQTTSubscription pointer fields.
func toTimePtr(t time.Time) *time.Time { return &t }
