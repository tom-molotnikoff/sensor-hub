package service

import (
	"context"
	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"fmt"
	"log/slog"
	"time"
)

type ReadingsService struct {
	repo    database.ReadingsRepository
	mtRepo  database.MeasurementTypeRepository
	tiers   []AggregationTier
	enabled bool
	logger  *slog.Logger
}

func NewReadingsService(repo database.ReadingsRepository, mtRepo database.MeasurementTypeRepository, tiers []AggregationTier, enabled bool, logger *slog.Logger) *ReadingsService {
	return &ReadingsService{
		repo:    repo,
		mtRepo:  mtRepo,
		tiers:   tiers,
		enabled: enabled,
		logger:  logger.With("component", "readings_service"),
	}
}

func (s *ReadingsService) ServiceGetBetweenDates(ctx context.Context, startDate, endDate, sensorName, measurementType string, overrideInterval, overrideFunction string) (*gen.AggregatedReadingsResponse, error) {
	var interval database.AggregationInterval
	var aggFunc database.AggregationFunction
	var err error

	if !s.enabled {
		interval = database.AggregationRaw
		aggFunc = database.AggregationFunctionNone
	} else if overrideInterval != "" {
		interval = database.AggregationInterval(overrideInterval)
		aggFunc, err = s.resolveFunction(ctx, measurementType, overrideFunction)
		if err != nil {
			return nil, err
		}
	} else {
		span, err := computeSpan(startDate, endDate)
		if err != nil {
			return nil, err
		}
		resolved := ResolveAggregationInterval(span, s.tiers)
		interval = database.AggregationInterval(resolved)
		aggFunc, err = s.resolveFunction(ctx, measurementType, overrideFunction)
		if err != nil {
			return nil, err
		}
	}

	if interval == database.AggregationRaw {
		aggFunc = database.AggregationFunctionNone
	}

	readings, err := s.repo.GetBetweenDates(ctx, startDate, endDate, sensorName, measurementType, interval, aggFunc)
	if err != nil {
		return nil, err
	}

	return &gen.AggregatedReadingsResponse{
		AggregationInterval: gen.AggregatedReadingsResponseAggregationInterval(interval),
		AggregationFunction: gen.AggregatedReadingsResponseAggregationFunction(aggFunc),
		Readings:            readings,
	}, nil
}

func (s *ReadingsService) resolveFunction(ctx context.Context, measurementType, overrideFunction string) (database.AggregationFunction, error) {
	if measurementType == "" {
		if overrideFunction != "" {
			return "", ErrMeasurementTypeRequiredForFunction
		}
		return database.AggregationFunctionAvg, nil
	}

	agg, err := s.mtRepo.GetAggregationsForMeasurementType(ctx, measurementType)
	if err != nil {
		s.logger.Warn("failed to look up aggregation function, defaulting to avg",
			"measurement_type", measurementType, "error", err)
		if overrideFunction != "" {
			return database.AggregationFunction(overrideFunction), nil
		}
		return database.AggregationFunctionAvg, nil
	}

	if overrideFunction != "" {
		for _, fn := range agg.SupportedFunctions {
			if fn == overrideFunction {
				return database.AggregationFunction(overrideFunction), nil
			}
		}
		return "", &ErrUnsupportedAggregationFunction{
			Function:        overrideFunction,
			MeasurementType: measurementType,
			Supported:       agg.SupportedFunctions,
		}
	}

	return database.AggregationFunction(agg.DefaultFunction), nil
}

func (s *ReadingsService) ServiceGetLatest(ctx context.Context) ([]gen.Reading, error) {
	readings, err := s.repo.GetLatest(ctx)
	if err != nil {
		return nil, err
	}
	return readings, nil
}

func computeSpan(startDate, endDate string) (time.Duration, error) {
	const layout = "2006-01-02 15:04:05"
	start, err := time.Parse(layout, startDate)
	if err != nil {
		return 0, fmt.Errorf("parse start date: %w", err)
	}
	end, err := time.Parse(layout, endDate)
	if err != nil {
		return 0, fmt.Errorf("parse end date: %w", err)
	}
	return end.Sub(start), nil
}
