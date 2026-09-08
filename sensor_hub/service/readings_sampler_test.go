package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestReadingsSampler_LatestSample_IsEmptyBeforeTheFirstSample(t *testing.T) {
	sampler := NewReadingsSampler(new(MockReadingsRepository), slog.Default())

	sample := sampler.LatestSample()

	assert.True(t, sample.SampledAt.IsZero())
	assert.Empty(t, sample.Counts)
}

func TestReadingsSampler_Sample_HoldsTheCountsAndTheTime(t *testing.T) {
	repo := new(MockReadingsRepository)
	repo.On("CountReadingsPerActiveSensor", mock.Anything).Return(map[string]int{"Office": 12, "Loft": 30}, nil)
	sampler := NewReadingsSampler(repo, slog.Default())

	before := time.Now().UTC()
	require.NoError(t, sampler.Sample(context.Background()))

	sample := sampler.LatestSample()
	assert.Equal(t, map[string]int{"Office": 12, "Loft": 30}, sample.Counts)
	assert.False(t, sample.SampledAt.Before(before))
	repo.AssertExpectations(t)
}

func TestReadingsSampler_Sample_KeepsThePreviousSampleOnError(t *testing.T) {
	repo := new(MockReadingsRepository)
	repo.On("CountReadingsPerActiveSensor", mock.Anything).Return(map[string]int{"Office": 12}, nil).Once()
	repo.On("CountReadingsPerActiveSensor", mock.Anything).Return(nil, errors.New("database error")).Once()
	sampler := NewReadingsSampler(repo, slog.Default())

	require.NoError(t, sampler.Sample(context.Background()))
	first := sampler.LatestSample()

	assert.Error(t, sampler.Sample(context.Background()))
	assert.Equal(t, first, sampler.LatestSample())
}

func TestReadingsSampler_LatestSample_DoesNotShareItsMap(t *testing.T) {
	repo := new(MockReadingsRepository)
	repo.On("CountReadingsPerActiveSensor", mock.Anything).Return(map[string]int{"Office": 12}, nil)
	sampler := NewReadingsSampler(repo, slog.Default())
	require.NoError(t, sampler.Sample(context.Background()))

	sampler.LatestSample().Counts["Office"] = 999

	assert.Equal(t, 12, sampler.LatestSample().Counts["Office"])
}

func collectRowGauge(t *testing.T, reader sdkmetric.Reader) map[string]int64 {
	t.Helper()
	var collected metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &collected))

	series := make(map[string]int64)
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != "sqlite.readings.rows" {
				continue
			}
			gauge, ok := m.Data.(metricdata.Gauge[int64])
			require.True(t, ok, "sqlite.readings.rows is an int64 gauge")
			for _, point := range gauge.DataPoints {
				sensor, _ := point.Attributes.Value(sensorAttributeKey)
				series[sensor.AsString()] = point.Value
			}
		}
	}
	return series
}

func TestReadingsSampler_Sample_RecordsARowGaugePerSensorAndATotal(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	t.Cleanup(func() { otel.SetMeterProvider(previous) })

	repo := new(MockReadingsRepository)
	repo.On("CountReadingsPerActiveSensor", mock.Anything).Return(map[string]int{"Office": 12, "Loft": 30}, nil)
	sampler := NewReadingsSampler(repo, slog.Default())

	require.NoError(t, sampler.Sample(context.Background()))

	assert.Equal(t, map[string]int64{"Office": 12, "Loft": 30, "": 42}, collectRowGauge(t, reader),
		"one series per sensor, and a total carrying no sensor attribute")
}
