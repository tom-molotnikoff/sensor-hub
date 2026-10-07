package automation

import (
	"testing"
	"time"

	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
)

var edgeStart = time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)

func number(value float64) gen.Reading {
	return gen.Reading{MeasurementType: "temperature", NumericValue: &value}
}

func state(value string) gen.Reading {
	return gen.Reading{MeasurementType: "contact", TextState: &value}
}

func fallsBelow(threshold, margin float64) ReadingCondition {
	return ReadingCondition{Operator: FallsBelow, Threshold: threshold, Margin: margin}
}

func fires(condition ReadingCondition, readings ...gen.Reading) []int {
	var e edge
	fired := []int{}
	for i, reading := range readings {
		if e.observe(condition, reading, edgeStart) == edgeFire {
			fired = append(fired, i)
		}
	}
	return fired
}

func TestEdge_FallsBelowFiresOnceForACrossing(t *testing.T) {
	assert.Equal(t, []int{1}, fires(fallsBelow(16, 0.2), number(16.2), number(15.9)))
}

func TestEdge_FallsBelowDoesNotFireAgainUntilTheValueReachesTheMargin(t *testing.T) {
	assert.Equal(t, []int{1}, fires(fallsBelow(16, 0.2), number(16.2), number(15.9), number(15.8), number(16.1), number(15.9)))
}

func TestEdge_FallsBelowFiresAgainOnceTheValueHasReachedTheMargin(t *testing.T) {
	assert.Equal(t, []int{1, 3}, fires(fallsBelow(16, 0.2), number(16.2), number(15.9), number(16.2), number(15.9)))
}

func TestEdge_FallsBelowMeansStrictlyBelow(t *testing.T) {
	assert.Equal(t, []int{}, fires(fallsBelow(16, 0.2), number(16)))
}

func TestEdge_RisesAboveFiresAboveTheThresholdAndReArmsAtTheThresholdLessTheMargin(t *testing.T) {
	risesAbove := ReadingCondition{Operator: RisesAbove, Threshold: 20, Margin: 0.3}
	assert.Equal(t, []int{1, 5}, fires(risesAbove, number(20), number(20.1), number(19.8), number(20.2), number(19.7), number(20.1)))
}

func TestEdge_TheFirstReadingFiresWhenItAlreadyMeetsTheCondition(t *testing.T) {
	assert.Equal(t, []int{0}, fires(fallsBelow(16, 0.2), number(15.2), number(15.1)))
}

func TestEdge_BecomesFiresWhenTheValueChangesToIt(t *testing.T) {
	becomesOpen := ReadingCondition{Operator: Becomes, Value: "false"}
	assert.Equal(t, []int{1, 4}, fires(becomesOpen, state("true"), state("false"), state("false"), state("true"), state("false")))
}

func TestEdge_ARepeatedValueDoesNotFireBecomes(t *testing.T) {
	becomesOpen := ReadingCondition{Operator: Becomes, Value: "false"}
	assert.Equal(t, []int{1}, fires(becomesOpen, state("true"), state("false"), state("false"), state("false")))
}

func TestEdge_AHeldConditionFiresWhenTheHoldEnds(t *testing.T) {
	condition := fallsBelow(16, 0.2)
	condition.Hold = 5 * time.Minute
	var e edge

	assert.Equal(t, edgeHold, e.observe(condition, number(15.9), edgeStart))
	assert.Equal(t, edgeNothing, e.observe(condition, number(15.8), edgeStart.Add(time.Minute)), "a reading during the hold does not restart it")
	assert.Equal(t, edgeStart.Add(5*time.Minute), e.holdUntil)
	assert.True(t, e.holdElapsed(edgeStart.Add(5*time.Minute)))
	assert.Equal(t, edgeNothing, e.observe(condition, number(15.7), edgeStart.Add(6*time.Minute)), "it fired, so it waits to re-arm")
}

func TestEdge_AReadingThatNoLongerMeetsTheConditionReleasesTheHold(t *testing.T) {
	condition := fallsBelow(16, 0.2)
	condition.Hold = 5 * time.Minute
	var e edge

	e.observe(condition, number(15.9), edgeStart)
	assert.Equal(t, edgeRelease, e.observe(condition, number(16), edgeStart.Add(2*time.Minute)))
	assert.False(t, e.holdElapsed(edgeStart.Add(5*time.Minute)))
	assert.Equal(t, edgeHold, e.observe(condition, number(15.9), edgeStart.Add(3*time.Minute)), "it never fired, so it is still armed")
}

func TestEdge_ABecomesThatDoesNotLastTheHoldDoesNotFire(t *testing.T) {
	condition := ReadingCondition{Operator: Becomes, Value: "false", Hold: 5 * time.Second}
	var e edge

	e.observe(condition, state("false"), edgeStart)
	assert.Equal(t, edgeRelease, e.observe(condition, state("true"), edgeStart.Add(3*time.Second)))
	assert.False(t, e.holdElapsed(edgeStart.Add(5*time.Second)))
}
