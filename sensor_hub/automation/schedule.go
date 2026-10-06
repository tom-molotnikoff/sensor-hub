package automation

import "time"

// Weekdays is a set of days of the week, one bit per time.Weekday.
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

// Schedule is a time of day on chosen weekdays, read on the hub's clock.
type Schedule struct {
	MinuteOfDay int
	Days        Weekdays
}

// NextAfter returns the first moment strictly after t at which the clocks in
// loc show the schedule's time on one of its days. Days must not be empty.
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

// wallClock returns the moment the clocks show minuteOfDay on day's date. A
// time the clocks skip when they go forward resolves to the first minute after
// the gap. A time that happens twice when they go back resolves to its first
// occurrence, so a schedule fires once that night.
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

// wallTime is what the clocks show at t, as a comparable value.
func wallTime(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
}
