package automation

import (
	"strings"
	"time"

	gen "example/sensorHub/gen"
)

// An edge is one reading trigger's state. A zero edge is armed, so the first
// reading that meets the condition fires it.
type edge struct {
	fired     bool
	holding   bool
	holdUntil time.Time
}

type edgeAction int

const (
	edgeNothing edgeAction = iota
	edgeFire
	edgeHold
	edgeRelease
)

// edgeHold means the trigger fires at holdUntil unless edgeRelease follows
// first.
func (e *edge) observe(condition ReadingCondition, reading gen.Reading, now time.Time) edgeAction {
	met, rearmed, ok := condition.test(reading)
	switch {
	case !ok:
		return edgeNothing
	case e.fired:
		e.fired = !rearmed
		return edgeNothing
	case !met && e.holding:
		e.holding = false
		return edgeRelease
	case !met || e.holding:
		return edgeNothing
	case condition.Hold == 0:
		e.fired = true
		return edgeFire
	default:
		e.holding = true
		e.holdUntil = now.Add(condition.Hold)
		return edgeHold
	}
}

// A due from a hold that has since been released, or released and started
// again, no longer stands.
func (e *edge) holdElapsed(due time.Time) bool {
	if !e.holding || !e.holdUntil.Equal(due) {
		return false
	}
	e.holding = false
	e.fired = true
	return true
}

// ok is false for a reading without a value of the series' kind.
func (c ReadingCondition) test(reading gen.Reading) (met, rearmed, ok bool) {
	if c.Operator == Becomes {
		if reading.TextState == nil {
			return false, false, false
		}
		// Binary values are matched without regard to case, as commands are.
		met = strings.EqualFold(*reading.TextState, c.Value)
		return met, !met, true
	}
	if reading.NumericValue == nil {
		return false, false, false
	}
	value := *reading.NumericValue
	if c.Operator == FallsBelow {
		return value < c.Threshold, value >= c.Threshold+c.Margin-rearmTolerance, true
	}
	return value > c.Threshold, value <= c.Threshold-c.Margin+rearmTolerance, true
}

// threshold + margin can land a rounding error away from the decimal it
// stands for, and a reading of exactly that decimal must still re-arm.
const rearmTolerance = 1e-9
