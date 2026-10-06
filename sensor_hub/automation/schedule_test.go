package automation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduleNextAfter(t *testing.T) {
	london, err := time.LoadLocation("Europe/London")
	require.NoError(t, err)
	newYork, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	weekdays := WeekdaysOf(time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday)
	daily := WeekdaysOf(time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday)
	at := func(hour, minute int) int { return hour*60 + minute }

	tests := []struct {
		name     string
		schedule Schedule
		loc      *time.Location
		after    time.Time
		want     time.Time
	}{
		{
			name:     "a weekday trigger fires at 19:00 local on a Tuesday",
			schedule: Schedule{MinuteOfDay: at(19, 0), Days: weekdays},
			loc:      london,
			after:    time.Date(2026, 10, 6, 18, 59, 59, 0, london),
			want:     time.Date(2026, 10, 6, 18, 0, 0, 0, time.UTC),
		},
		{
			name:     "a weekday trigger skips Saturday and Sunday",
			schedule: Schedule{MinuteOfDay: at(19, 0), Days: weekdays},
			loc:      london,
			after:    time.Date(2026, 10, 9, 19, 0, 0, 0, london),
			want:     time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC),
		},
		{
			name:     "a time skipped when the clocks go forward fires at the next valid minute",
			schedule: Schedule{MinuteOfDay: at(1, 30), Days: daily},
			loc:      london,
			after:    time.Date(2026, 3, 29, 0, 0, 0, 0, london),
			want:     time.Date(2026, 3, 29, 1, 0, 0, 0, time.UTC),
		},
		{
			name:     "a skipped time resolves the same way in another zone",
			schedule: Schedule{MinuteOfDay: at(2, 30), Days: daily},
			loc:      newYork,
			after:    time.Date(2026, 3, 8, 0, 0, 0, 0, newYork),
			want:     time.Date(2026, 3, 8, 7, 0, 0, 0, time.UTC),
		},
		{
			name:     "a time repeated when the clocks go back fires at its first occurrence",
			schedule: Schedule{MinuteOfDay: at(1, 30), Days: daily},
			loc:      london,
			after:    time.Date(2026, 10, 25, 0, 0, 0, 0, london),
			want:     time.Date(2026, 10, 25, 0, 30, 0, 0, time.UTC),
		},
		{
			name:     "after the first occurrence of a repeated time the next fire is the next night",
			schedule: Schedule{MinuteOfDay: at(1, 30), Days: daily},
			loc:      london,
			after:    time.Date(2026, 10, 25, 0, 30, 0, 0, time.UTC),
			want:     time.Date(2026, 10, 26, 1, 30, 0, 0, time.UTC),
		},
		{
			name:     "a repeated time resolves the same way in another zone",
			schedule: Schedule{MinuteOfDay: at(1, 30), Days: daily},
			loc:      newYork,
			after:    time.Date(2026, 11, 1, 0, 0, 0, 0, newYork),
			want:     time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC),
		},
		{
			name:     "19:00 stays 19:00 local when the clocks go back",
			schedule: Schedule{MinuteOfDay: at(19, 0), Days: daily},
			loc:      london,
			after:    time.Date(2026, 10, 24, 19, 0, 0, 0, london),
			want:     time.Date(2026, 10, 25, 19, 0, 0, 0, time.UTC),
		},
		{
			name:     "19:00 stays 19:00 local when the clocks go forward",
			schedule: Schedule{MinuteOfDay: at(19, 0), Days: daily},
			loc:      london,
			after:    time.Date(2026, 3, 28, 19, 0, 0, 0, london),
			want:     time.Date(2026, 3, 29, 18, 0, 0, 0, time.UTC),
		},
		{
			name:     "the due time itself is not after it, so the next is a week on",
			schedule: Schedule{MinuteOfDay: at(7, 15), Days: WeekdaysOf(time.Wednesday)},
			loc:      time.UTC,
			after:    time.Date(2026, 10, 7, 7, 15, 0, 0, time.UTC),
			want:     time.Date(2026, 10, 14, 7, 15, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.schedule.NextAfter(tt.after, tt.loc)
			assert.True(t, tt.want.Equal(got), "want %s, got %s", tt.want, got.UTC())
		})
	}
}
