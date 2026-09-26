package main

import (
	"math"
	"math/rand/v2"
)

const (
	stateOn  = "true"
	stateOff = "false"
)

type value struct {
	number float64
	state  string
}

func (v value) binary() bool {
	return v.state != ""
}

func (v value) numberOrNil() any {
	if v.binary() {
		return nil
	}
	return v.number
}

func (v value) stateOrNil() any {
	if v.binary() {
		return v.state
	}
	return nil
}

type track struct {
	measurement string
	values      []value
}

type signal interface {
	generate(r *rand.Rand, points int, latest map[string]value) []track
}

type walk struct {
	measurement     string
	low, high, step float64
	decimals        int
	start           float64
}

func (w walk) generate(r *rand.Rand, points int, latest map[string]value) []track {
	values := make([]value, points)
	current := w.start
	if last, ok := latest[w.measurement]; ok && !last.binary() {
		current = last.number
	}
	for i := range values {
		if last := points - 1; i == last {
			current = w.start
		} else {
			reach := float64(last-i) * w.step
			current = round(w.towardStart(current+(r.Float64()*2-1)*w.step, current, reach), w.decimals)
		}
		values[i] = value{number: current}
	}
	return []track{{measurement: w.measurement, values: values}}
}

func (w walk) towardStart(candidate, current, reach float64) float64 {
	low := max(w.low, w.start-reach, current-w.step)
	high := min(w.high, w.start+reach, current+w.step)
	if low <= high {
		return min(max(candidate, low), high)
	}
	if current < w.start {
		return min(current+w.step, w.high)
	}
	return max(current-w.step, w.low)
}

type uniform struct {
	measurement string
	low, high   float64
	decimals    int
}

func (u uniform) generate(r *rand.Rand, points int, _ map[string]value) []track {
	values := make([]value, points)
	for i := range values {
		values[i] = value{number: round(u.low+r.Float64()*(u.high-u.low), u.decimals)}
	}
	return []track{{measurement: u.measurement, values: values}}
}

type steady struct {
	measurement string
	value       value
}

func (s steady) generate(_ *rand.Rand, points int, _ map[string]value) []track {
	values := make([]value, points)
	for i := range values {
		values[i] = s.value
	}
	return []track{{measurement: s.measurement, values: values}}
}

type toggle struct {
	measurement string
	chance      float64
	start       bool
}

func (t toggle) generate(r *rand.Rand, points int, latest map[string]value) []track {
	values := make([]value, points)
	on := t.start
	if last, ok := latest[t.measurement]; ok && last.binary() {
		on = last.state == stateOn
	}
	for i := range values {
		if r.Float64() < t.chance {
			on = !on
		}
		values[i] = binaryValue(on)
	}
	return []track{{measurement: t.measurement, values: values}}
}

type plugDraw struct {
	minPower, maxPower float64
	startEnergy        float64
}

const mainsVoltage = 230.0

func (p plugDraw) generate(r *rand.Rand, points int, _ map[string]value) []track {
	power := make([]value, points)
	energy := make([]value, points)
	current := make([]value, points)
	state := make([]value, points)
	kilowattHoursPerWattInterval := readingInterval.Seconds() / 3_600_000
	for i := range power {
		watts := round(p.minPower+r.Float64()*(p.maxPower-p.minPower), 1)
		power[i] = value{number: watts}
		current[i] = value{number: round(watts/mainsVoltage, 3)}
		state[i] = binaryValue(true)
	}
	total := p.startEnergy
	for i := points - 1; i >= 0; i-- {
		energy[i] = value{number: round(total, 4)}
		total = max(0, total-power[i].number*kilowattHoursPerWattInterval)
	}
	return []track{
		{measurement: "power", values: power},
		{measurement: "energy", values: energy},
		{measurement: "current", values: current},
		{measurement: "state", values: state},
	}
}

func binaryValue(on bool) value {
	if on {
		return value{state: stateOn}
	}
	return value{state: stateOff}
}

func round(number float64, decimals int) float64 {
	scale := math.Pow10(decimals)
	return math.Round(number*scale) / scale
}
