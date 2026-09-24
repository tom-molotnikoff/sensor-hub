package service

import (
	"errors"
	"time"
)

// ============================================================================
// Aggregation tier — internal config type for the readings service
// ============================================================================

// AggregationTier maps a maximum time-range span to the aggregation bucket size.
type AggregationTier struct {
	MaxSpan  time.Duration
	Interval string // "raw" or ISO 8601 duration like "PT5M"
}

// ============================================================================
// Aggregation error
// ============================================================================

var ErrMeasurementTypeRequiredForFunction = errors.New("a measurement type is required to override the aggregation function")

// ErrUnsupportedAggregationFunction is returned when a requested aggregation
// function is not supported for the given measurement type.
type ErrUnsupportedAggregationFunction struct {
	Function        string
	MeasurementType string
	Supported       []string
}

func (e *ErrUnsupportedAggregationFunction) Error() string {
	return "aggregation function \"" + e.Function + "\" is not supported for measurement type \"" +
		e.MeasurementType + "\"; supported: " + joinStrings(e.Supported)
}

func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}

// ============================================================================
// Role constants — auth-layer constants used by service code
// ============================================================================

const (
	RoleAdmin  = "admin"
	RoleUser   = "user"
	RoleViewer = "viewer"
)

// ============================================================================
// SensorServers — YAML config types used when loading sensor server configs
// ============================================================================

type SensorServers struct {
	Servers []SensorServerItem `yaml:"servers"`
}

type SensorServerItem struct {
	Url       string                                  `yaml:"url"`
	Variables map[string]SensorServerVariableProperty `yaml:"variables"`
}

type SensorServerVariableProperty struct {
	Default string `yaml:"default"`
}
