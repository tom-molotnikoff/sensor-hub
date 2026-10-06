package automation

import "time"

type Weekdays uint8

func (w Weekdays) Has(day time.Weekday) bool {
	return w&(1<<day) != 0
}

func WeekdaysOf(days ...time.Weekday) Weekdays {
	var w Weekdays
	for _, day := range days {
		w |= 1 << day
	}
	return w
}

type Schedule struct {
	MinuteOfDay int
	Days        Weekdays
}

// Days must not be empty, or NextAfter never returns.
func (s Schedule) NextAfter(t time.Time, loc *time.Location) time.Time {
	local := t.In(loc)
	for offset := 0; ; offset++ {
		// Noon is never inside a clock change, so it names the day safely.
		noon := time.Date(local.Year(), local.Month(), local.Day()+offset, 12, 0, 0, 0, loc)
		if !s.Days.Has(noon.Weekday()) {
			continue
		}
		due := wallClock(noon, s.MinuteOfDay)
		if due.After(t) {
			return due
		}
	}
}

// A time the clocks skip when they go forward resolves to the first minute
// after the gap. A time that happens twice when they go back resolves to its
// first occurrence, so a schedule fires once that night.
func wallClock(day time.Time, minuteOfDay int) time.Time {
	year, month, date := day.Date()
	hour, minute := minuteOfDay/60, minuteOfDay%60
	t := time.Date(year, month, date, hour, minute, 0, 0, day.Location())

	if t.Hour() != hour || t.Minute() != minute {
		// time.Date placed the skipped time on one side of the gap. The gap's
		// end is where that side's zone starts, or where the other side's ends.
		start, end := t.ZoneBounds()
		if wallTime(t).After(time.Date(year, month, date, hour, minute, 0, 0, time.UTC)) {
			return start
		}
		return end
	}

	start, _ := t.ZoneBounds()
	if start.IsZero() {
		return t
	}
	_, previousOffset := start.Add(-time.Nanosecond).Zone()
	_, offset := t.Zone()
	if previousOffset > offset {
		earlier := t.Add(-time.Duration(previousOffset-offset) * time.Second)
		if earlier.Before(start) && wallTime(earlier).Equal(wallTime(t)) {
			return earlier
		}
	}
	return t
}

func wallTime(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
}
