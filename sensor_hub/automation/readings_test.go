package automation

import (
	"context"
	"testing"
	"time"

	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func droppedReadings(t *testing.T, reader *sdkmetric.ManualReader) int64 {
	t.Helper()
	var collected metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &collected))
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name == "automation.readings.dropped" {
				var total int64
				for _, point := range m.Data.(metricdata.Sum[int64]).DataPoints {
					total += point.Value
				}
				return total
			}
		}
	}
	return 0
}

func TestReadingConsumer_KeepsTheLatestReadingOfEachSeriesOnceTheBufferIsFull(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	t.Cleanup(func() { otel.SetMeterProvider(previous) })
	consumer := NewReadingConsumer()
	lounge, kitchen := gen.Sensor{Id: 1}, gen.Sensor{Id: 2}
	values := func(values ...float64) []gen.Reading {
		readings := make([]gen.Reading, 0, len(values))
		for _, value := range values {
			readings = append(readings, number(value))
		}
		return readings
	}
	filler := make([]float64, readingBuffer)
	for i := range filler {
		filler[i] = 20
	}

	start := time.Now()
	consumer.Consume(context.Background(), lounge, values(filler...))
	consumer.Consume(context.Background(), kitchen, values(18, 17, 16))
	consumer.Consume(context.Background(), lounge, values(15, 14))
	assert.Less(t, time.Since(start), time.Second, "Consume blocked")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var observed []seriesReading
	done := make(chan struct{})
	go consumer.drain(ctx, func() {}, func(series Series, reading gen.Reading) {
		observed = append(observed, seriesReading{series, reading})
		if len(observed) == readingBuffer+2 {
			close(done)
		}
	})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.Fail(t, "the worker never saw every reading kept")
	}
	cancel()

	last := observed[readingBuffer:]
	assert.Equal(t, Series{SensorID: 2, MeasurementType: "temperature"}, last[0].series)
	assert.Equal(t, 16.0, *last[0].reading.NumericValue)
	assert.Equal(t, Series{SensorID: 1, MeasurementType: "temperature"}, last[1].series)
	assert.Equal(t, 14.0, *last[1].reading.NumericValue)
	assert.Equal(t, int64(3), droppedReadings(t, reader))
}
