package readings

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	gen "example/sensorHub/gen"
	"example/sensorHub/telemetry"

	"go.opentelemetry.io/otel/metric"
)

const successfulReadingReason = "successful reading"

type ReadingBatch struct {
	SensorName   string
	HealthReason string
	Readings     []gen.Reading
}

type Reading struct {
	gen.Reading
	// The command tracker sets CauseRunID, so only the consumers after it see
	// it.
	CauseRunID *int
}

type Consumer interface {
	Consume(ctx context.Context, sensor gen.Sensor, batch []Reading)
}

type Store interface {
	Ingest(ctx context.Context, batch ReadingBatch) ([]gen.Reading, error)
}

type HealthRecorder interface {
	RecordHealth(ctx context.Context, sensorID int, status gen.SensorHealthStatus, reason string)
}

type Pipeline struct {
	store     Store
	health    HealthRecorder
	consumers []Consumer
	duration  metric.Float64Histogram
	logger    *slog.Logger
}

func NewPipeline(store Store, health HealthRecorder, logger *slog.Logger, consumers ...Consumer) *Pipeline {
	// Most batches take well under the default buckets' first boundary of 5 ms.
	duration, _ := telemetry.Meter("readings").Float64Histogram("readings.process.duration",
		metric.WithDescription("Time to ingest a reading batch and pass it to every consumer"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(0.1, 0.25, 0.5, 1, 2.5, 5, 10, 25, 50, 100, 250, 500, 1000))
	return &Pipeline{
		store:     store,
		health:    health,
		consumers: consumers,
		duration:  duration,
		logger:    logger.With("component", "reading_pipeline"),
	}
}

func (p *Pipeline) Process(ctx context.Context, sensor gen.Sensor, readings []gen.Reading) error {
	if len(readings) == 0 {
		return nil
	}
	started := time.Now()
	defer func() {
		p.duration.Record(ctx, float64(time.Since(started).Microseconds())/1000)
	}()

	batch := make([]gen.Reading, len(readings))
	for i, reading := range readings {
		reading.SensorName = sensor.Name
		batch[i] = reading
	}

	stored, err := p.store.Ingest(ctx, ReadingBatch{
		SensorName:   sensor.Name,
		HealthReason: successfulReadingReason,
		Readings:     batch,
	})
	if err != nil {
		p.health.RecordHealth(ctx, sensor.Id, gen.Bad, fmt.Sprintf("error storing readings: %v", err))
		return fmt.Errorf("error storing readings from sensor %s: %w", sensor.Name, err)
	}

	passed := make([]Reading, len(stored))
	for i, reading := range stored {
		passed[i] = Reading{Reading: reading}
	}
	for _, consumer := range p.consumers {
		consumer.Consume(ctx, sensor, passed)
	}
	p.logger.Debug("processed readings", "sensor", sensor.Name, "count", len(stored))
	return nil
}
