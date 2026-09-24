package utils

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// NormalizeTimeToSpaceFormat parses a time string in various formats and
// returns it in "YYYY-MM-DD HH:MM:SS" UTC. All timezone-aware inputs are
// converted to UTC so that stored timestamps are always comparable.
func NormalizeTimeToSpaceFormat(s string) string {
	return normalizeTime(s, "2006-01-02 15:04:05")
}

func NormalizeTimeToRFC3339(s string) string {
	return normalizeTime(s, time.RFC3339)
}

func normalizeTime(s, layout string) string {
	if s == "" {
		return s
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC().Format(layout)
		}
	}

	if sec, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(sec, 0).UTC().Format(layout)
	}
	return s
}

// NormalizeDateTimeParam parses an API date/datetime parameter and returns
// a "YYYY-MM-DD HH:MM:SS" UTC string suitable for SQL BETWEEN queries.
// Accepts: YYYY-MM-DD, YYYY-MM-DDTHH:MM:SS, YYYY-MM-DDTHH:MM:SSZ,
// YYYY-MM-DDTHH:MM:SS±HH:MM. For date-only input, useEndOfDay controls
// whether midnight (false) or 23:59:59 (true) is returned.
func NormalizeDateTimeParam(s string, useEndOfDay bool) (string, error) {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC().Format("2006-01-02 15:04:05"), nil
		}
	}
	// Date-only: expand to start or end of day
	if t, err := time.Parse("2006-01-02", s); err == nil {
		if useEndOfDay {
			t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		}
		return t.UTC().Format("2006-01-02 15:04:05"), nil
	}
	return "", fmt.Errorf("unrecognised date/time format: %s", s)
}

// ParseISO8601Duration parses a subset of ISO 8601 durations: P[n]D, PT[n]H, PT[n]M, PT[n]S
// and combinations (e.g. P1DT6H, PT1H30M).
func ParseISO8601Duration(s string) (time.Duration, error) {
	matches := iso8601Re.FindStringSubmatch(strings.ToUpper(s))
	if matches == nil {
		return 0, fmt.Errorf("invalid ISO 8601 duration: %q", s)
	}
	var d time.Duration
	if matches[1] != "" {
		var days int
		fmt.Sscanf(matches[1], "%d", &days)
		d += time.Duration(days) * 24 * time.Hour
	}
	if matches[2] != "" {
		var hours int
		fmt.Sscanf(matches[2], "%d", &hours)
		d += time.Duration(hours) * time.Hour
	}
	if matches[3] != "" {
		var mins int
		fmt.Sscanf(matches[3], "%d", &mins)
		d += time.Duration(mins) * time.Minute
	}
	if matches[4] != "" {
		var secs int
		fmt.Sscanf(matches[4], "%d", &secs)
		d += time.Duration(secs) * time.Second
	}
	if d == 0 {
		return 0, fmt.Errorf("ISO 8601 duration is zero: %q", s)
	}
	return d, nil
}

var iso8601Re = regexp.MustCompile(`^P(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$`)

var ReadPropertiesFile = func(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open properties file: %w", err)
	}
	defer file.Close()

	props := make(map[string]string)

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			props[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read properties file: %w", err)
	}
	return props, nil
}
