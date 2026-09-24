package database

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

// SQLiteTime wraps time.Time to handle scanning from SQLite TEXT columns.
// modernc.org/sqlite returns datetime columns as strings, which database/sql
// cannot automatically convert to time.Time.
type SQLiteTime struct {
	time.Time
}

var timeFormats = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05Z",
	"2006-01-02 15:04:05.999999999 -0700 MST",
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05 -0700 MST",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func (t *SQLiteTime) Scan(value interface{}) error {
	if value == nil {
		t.Time = time.Time{}
		return nil
	}

	switch v := value.(type) {
	case time.Time:
		t.Time = v.UTC()
		return nil
	case string:
		// Strip Go monotonic clock suffix (e.g. " m=+0.004999326")
		if idx := strings.Index(v, " m="); idx != -1 {
			v = v[:idx]
		}
		for _, format := range timeFormats {
			if parsed, err := time.Parse(format, v); err == nil {
				t.Time = parsed.UTC()
				return nil
			}
		}
		return fmt.Errorf("cannot parse time string: %q", v)
	case int64:
		t.Time = time.Unix(v, 0).UTC()
		return nil
	default:
		return fmt.Errorf("unsupported time type: %T", value)
	}
}

func (t SQLiteTime) Value() (driver.Value, error) {
	if t.IsZero() {
		return nil, nil
	}
	return t.Time.Format(time.RFC3339Nano), nil
}

// NullSQLiteTime handles nullable datetime columns.
type NullSQLiteTime struct {
	Time  time.Time
	Valid bool
}

func (t *NullSQLiteTime) Scan(value interface{}) error {
	if value == nil {
		t.Time = time.Time{}
		t.Valid = false
		return nil
	}
	t.Valid = true
	st := &SQLiteTime{}
	if err := st.Scan(value); err != nil {
		return err
	}
	t.Time = st.Time
	return nil
}
