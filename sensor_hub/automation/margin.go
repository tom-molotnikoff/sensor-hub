package automation

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	gen "example/sensorHub/gen"
)

const marginCheckInterval = 24 * time.Hour

type marginTier struct {
	readings   int
	confidence gen.MarginSuggestionConfidence
}

// Largest first: a series gets the first tier it has enough readings for.
var marginTiers = []marginTier{
	{1000, gen.MarginConfidenceHigh},
	{100, gen.MarginConfidenceMedium},
	{30, gen.MarginConfidenceLow},
}

// ReadingHistory must read through the reader pool.
type ReadingHistory interface {
	// LatestNumericValues returns up to limit of a series' values, newest first.
	LatestNumericValues(ctx context.Context, sensorID int, measurementTypeID int, limit int) ([]float64, error)
}

func (s *Service) MarginSuggestion(ctx context.Context, sensorID int, measurementType string) (gen.MarginSuggestion, error) {
	reported, err := reportedType(ctx, s.sensors, "", sensorID, measurementType)
	if err != nil {
		return gen.MarginSuggestion{}, err
	}
	if reported.Category == gen.MeasurementTypeCategoryBinary {
		return gen.MarginSuggestion{}, invalid("measurement_type: %s is binary, so it has no re-arm margin", reported.Name)
	}
	return s.suggestMargin(ctx, sensorID, reported.Id)
}

func (s *Service) suggestMargin(ctx context.Context, sensorID int, measurementTypeID int) (gen.MarginSuggestion, error) {
	latest, err := s.history.LatestNumericValues(ctx, sensorID, measurementTypeID, marginTiers[0].readings)
	if err != nil {
		return gen.MarginSuggestion{}, fmt.Errorf("read latest values of sensor %d: %w", sensorID, err)
	}
	return suggestMargin(latest), nil
}

func suggestMargin(latest []float64) gen.MarginSuggestion {
	suggestion := gen.MarginSuggestion{SampleCount: len(latest), Confidence: gen.MarginConfidenceNone}
	tier := slices.IndexFunc(marginTiers, func(tier marginTier) bool { return len(latest) >= tier.readings })
	if tier < 0 {
		return suggestion
	}
	sample := latest[:marginTiers[tier].readings]
	suggestion.SampleCount, suggestion.Confidence = len(sample), marginTiers[tier].confidence

	changes := make([]float64, 0, len(sample)-1)
	for i := 1; i < len(sample); i++ {
		changes = append(changes, roundChange(math.Abs(sample[i]-sample[i-1])))
	}
	slices.Sort(changes)
	p95 := changes[int(math.Ceil(0.95*float64(len(changes))))-1]
	margin := 0.0
	if smallest := slices.IndexFunc(changes, func(change float64) bool { return change > 0 }); smallest >= 0 {
		step := changes[smallest]
		// Less a hair, so that a quotient such as 7.000000000000001 stays 7.
		margin = roundChange(max(1, math.Ceil(p95/step-1e-9)) * step)
		suggestion.Step = &step
	}
	suggestion.P95Change, suggestion.SuggestedMargin = &p95, &margin
	return suggestion
}

// Readings are stored as floats, so 16.2 - 16.1 is 0.10000000000000142.
// Rounding to a billionth gives back the step the sensor reports in.
func roundChange(change float64) float64 {
	return math.Round(change*1e9) / 1e9
}

// checkMargins writes only margin hints, never a saved margin.
func (s *Service) checkMargins(ctx context.Context) error {
	automations, err := s.store.ListAutomations(ctx)
	if err != nil {
		return fmt.Errorf("load automations: %w", err)
	}
	type seriesKey struct{ sensorID, measurementTypeID int }
	suggestions := make(map[seriesKey]gen.MarginSuggestion)
	checkedAt := s.now()
	var errs []error
	checked, hinted := 0, 0
	for _, automation := range automations {
		for _, trigger := range automation.Triggers {
			condition := trigger.Reading
			if condition == nil || condition.Operator == Becomes {
				continue
			}
			key := seriesKey{condition.SensorID, condition.MeasurementTypeID}
			suggestion, ok := suggestions[key]
			if !ok {
				if suggestion, err = s.suggestMargin(ctx, key.sensorID, key.measurementTypeID); err != nil {
					errs = append(errs, err)
					continue
				}
				suggestions[key] = suggestion
			}
			checked++

			var hint *MarginHint
			switch {
			case suggestion.SuggestedMargin == nil:
				continue
			case condition.Margin >= *suggestion.SuggestedMargin:
				if condition.MarginHint == nil {
					continue
				}
			case suggestion.Confidence == gen.MarginConfidenceHigh || suggestion.Confidence == gen.MarginConfidenceMedium:
				hint = &MarginHint{Margin: *suggestion.SuggestedMargin, CheckedAt: checkedAt}
				hinted++
			default:
				continue
			}
			if err := s.store.SetMarginHint(ctx, trigger.ID, hint); err != nil {
				errs = append(errs, err)
			}
		}
	}
	s.logger.Info("automation margins checked", "triggers", checked, "hinted", hinted)
	return errors.Join(errs...)
}
