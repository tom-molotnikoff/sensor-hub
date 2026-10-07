package automation

import (
	"context"
	"sync"

	gen "example/sensorHub/gen"
	"example/sensorHub/readings"
	"example/sensorHub/telemetry"

	"go.opentelemetry.io/otel/metric"
)

const readingBuffer = 1024

type seriesReading struct {
	series  Series
	reading readings.Reading
}

// ReadingConsumer is the automation consumer on the reading pipeline. It never
// blocks ingest.
//
// Once the buffer is full, readings wait in overflow, the latest one per
// series, until the worker has emptied the buffer. Every reading goes to
// overflow until then, so a series' readings still reach the worker in order.
type ReadingConsumer struct {
	buffer  chan seriesReading
	dropped metric.Int64Counter

	mu       sync.Mutex
	overflow map[Series]readings.Reading
	order    []Series
}

func NewReadingConsumer() *ReadingConsumer {
	dropped, _ := telemetry.Meter("automation").Int64Counter("automation.readings.dropped",
		metric.WithDescription("Readings the automation worker never saw, because a newer reading of the same series replaced them while it was behind"),
		metric.WithUnit("{reading}"))
	return &ReadingConsumer{
		buffer:   make(chan seriesReading, readingBuffer),
		dropped:  dropped,
		overflow: make(map[Series]readings.Reading),
	}
}

func (c *ReadingConsumer) Consume(ctx context.Context, sensor gen.Sensor, batch []readings.Reading) {
	c.mu.Lock()
	replaced := 0
	for _, reading := range batch {
		series := Series{SensorID: sensor.Id, MeasurementType: reading.MeasurementType}
		if len(c.order) == 0 {
			select {
			case c.buffer <- seriesReading{series: series, reading: reading}:
				continue
			default:
			}
		}
		if _, waiting := c.overflow[series]; waiting {
			replaced++
		} else {
			c.order = append(c.order, series)
		}
		c.overflow[series] = reading
	}
	c.mu.Unlock()
	if replaced > 0 {
		c.dropped.Add(ctx, int64(replaced))
	}
}

func (c *ReadingConsumer) drain(ctx context.Context, healthy func(), observe func(Series, readings.Reading)) {
	for {
		select {
		case <-ctx.Done():
			return
		case next := <-c.buffer:
			observe(next.series, next.reading)
		}
		if len(c.buffer) == 0 {
			for _, next := range c.takeOverflow() {
				observe(next.series, next.reading)
			}
		}
		healthy()
	}
}

func (c *ReadingConsumer) takeOverflow() []seriesReading {
	c.mu.Lock()
	defer c.mu.Unlock()
	taken := make([]seriesReading, 0, len(c.order))
	for _, series := range c.order {
		taken = append(taken, seriesReading{series: series, reading: c.overflow[series]})
	}
	clear(c.overflow)
	c.order = c.order[:0]
	return taken
}
