package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLiteTime_ScanConvertsOffsetsToUTC(t *testing.T) {
	var st SQLiteTime
	require.NoError(t, st.Scan("2026-09-08T23:06:55.543241511+01:00"))

	assert.Equal(t, time.UTC, st.Location())
	assert.Equal(t, "2026-09-08T22:06:55.543241511Z", st.Format(time.RFC3339Nano))
}
