package automation

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type timedValue struct {
	at    time.Time
	value float64
}

// generatedWeek stands in for a week of a room's temperature readings: a
// reading about once a minute, quantised to the sensor's 0.1 °C step, on a
// daily swing with a slow drift, with sensor noise that flickers between
// neighbouring steps whenever the room sits near a step boundary. It chatters
// about ten times as often as the live hub's real week of 28 Sep - 5 Oct 2026,
// which is not committed.
func generatedWeek(seed uint64) []timedValue {
	random := rand.New(rand.NewPCG(seed, seed))
	var week []timedValue
	drift := 0.0
	for at := edgeStart; at.Before(edgeStart.Add(7 * 24 * time.Hour)); at = at.Add(time.Duration(50+random.IntN(30)) * time.Second) {
		drift = max(-1.5, min(1.5, drift+random.NormFloat64()*0.01))
		day := float64(at.Sub(edgeStart)) / float64(24*time.Hour)
		room := 18 + 2*math.Sin(2*math.Pi*day) + drift
		week = append(week, timedValue{at, math.Round((room+random.NormFloat64()*0.02)*10) / 10})
	}
	return week
}

func reversals(week []timedValue, margin float64, window time.Duration) int {
	low, high := week[0].value, week[0].value
	for _, reading := range week {
		low, high = min(low, reading.value), max(high, reading.value)
	}
	count := 0
	for threshold := math.Floor(low*2) / 2; threshold <= high; threshold += 0.5 {
		var e edge
		var last time.Time
		for _, reading := range week {
			wasFired := e.fired
			e.observe(fallsBelow(threshold, margin), number(reading.value), reading.at)
			if e.fired == wasFired {
				continue
			}
			if !last.IsZero() && reading.at.Sub(last) < window {
				count++
			}
			last = reading.at
		}
	}
	return count
}

func TestReplay_AMarginOfAFifthOfADegreeLeavesNoCrossingThatReversesWithinFifteenMinutes(t *testing.T) {
	for seed := uint64(1); seed <= 4; seed++ {
		week := generatedWeek(seed)

		assert.Positive(t, reversals(week, 0, 15*time.Minute), "seed %d: the generated noise does not chatter at all without a margin", seed)
		assert.Zero(t, reversals(week, 0.2, 15*time.Minute), "seed %d", seed)
	}
}
