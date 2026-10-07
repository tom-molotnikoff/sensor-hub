package readings

import (
	"context"
	"fmt"
	"log/slog"

	gen "example/sensorHub/gen"
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
	logger    *slog.Logger
}

func NewPipeline(store Store, health HealthRecorder, logger *slog.Logger, consumers ...Consumer) *Pipeline {
	return &Pipeline{
		store:     store,
		health:    health,
		consumers: consumers,
		logger:    logger.With("component", "reading_pipeline"),
	}
}

func (p *Pipeline) Process(ctx context.Context, sensor gen.Sensor, readings []gen.Reading) error {
	if len(readings) == 0 {
		return nil
	}

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
