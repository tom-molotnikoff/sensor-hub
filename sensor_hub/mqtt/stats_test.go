package mqtt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatsTracker_SnapshotTimesAreUTC(t *testing.T) {
	st := NewStatsTracker()
	st.RecordConnected(1)
	st.RecordMessageReceived(1)

	stats := st.Snapshot()[1]

	require.NotNil(t, stats.LastMessageAt)
	require.NotNil(t, stats.ConnectedSince)
	assert.Equal(t, time.UTC, stats.LastMessageAt.Location())
	assert.Equal(t, time.UTC, stats.ConnectedSince.Location())
}
