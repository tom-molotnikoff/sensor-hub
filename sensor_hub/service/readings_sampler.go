package service

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"sync"
	"time"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/telemetry"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var sensorAttributeKey = attribute.Key("sensor")

type readingsSampler struct {
	readingsRepo database.ReadingsRepository
	logger       *slog.Logger
	rows         metric.Int64Gauge

	mu     sync.RWMutex
	sample gen.TotalReadingsSample
}

func NewReadingsSampler(readingsRepo database.ReadingsRepository, logger *slog.Logger) ReadingsSamplerInterface {
	meter := telemetry.Meter("sqlite")

	rows, _ := meter.Int64Gauge("sqlite.readings.rows",
		metric.WithDescription("Rows in the SQLite readings table, per sensor and in total"),
		metric.WithUnit("{row}"))

	return &readingsSampler{
		readingsRepo: readingsRepo,
		logger:       logger.With("component", "readings_sampler"),
		rows:         rows,
		sample:       gen.TotalReadingsSample{Counts: map[string]int{}},
	}
}

func (s *readingsSampler) Sample(ctx context.Context) error {
	counts, err := s.readingsRepo.CountReadingsPerActiveSensor(ctx)
	if err != nil {
		return fmt.Errorf("failed to count readings per sensor: %w", err)
	}

	var total int64
	for name, count := range counts {
		s.rows.Record(ctx, int64(count), metric.WithAttributes(sensorAttributeKey.String(name)))
		total += int64(count)
	}
	s.rows.Record(ctx, total)

	sample := gen.TotalReadingsSample{SampledAt: time.Now().UTC(), Counts: counts}

	s.mu.Lock()
	s.sample = sample
	s.mu.Unlock()

	s.logger.Debug("sampled readings row counts", "sensors", len(counts), "total_rows", total)
	return nil
}

func (s *readingsSampler) LatestSample() gen.TotalReadingsSample {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return gen.TotalReadingsSample{
		SampledAt: s.sample.SampledAt,
		Counts:    maps.Clone(s.sample.Counts),
	}
}
