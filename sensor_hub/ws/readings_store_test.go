package ws

import (
	"testing"
	"time"

	gen "example/sensorHub/gen"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func numericReading(sensor, measurementType string, value float64, ts string) gen.Reading {
	v := value
	return gen.Reading{SensorName: sensor, MeasurementType: measurementType, NumericValue: &v, Time: ts}
}

func snapshotBySeries(readings []gen.Reading) map[string]gen.Reading {
	out := make(map[string]gen.Reading, len(readings))
	for _, r := range readings {
		out[r.SensorName+"/"+r.MeasurementType] = r
	}
	return out
}

func TestReadingsStore_Snapshot_ReturnsLatestPerSeriesLastWriteWins(t *testing.T) {
	store := NewReadingsStore()

	store.Update([]gen.Reading{
		numericReading("office-plug", "power", 10, "2025-01-01T00:00:00Z"),
		numericReading("office-plug", "power", 20, "2025-01-01T00:01:00Z"), // newer for same series
		numericReading("attic", "temperature", 5, "2025-01-01T00:00:00Z"),
	})

	snap := store.Snapshot()
	assert.Len(t, snap, 2, "one entry per (sensor, measurement type) series")

	bySeries := snapshotBySeries(snap)
	assert.Equal(t, 20.0, *bySeries["office-plug/power"].NumericValue, "later write wins")
	assert.Equal(t, 5.0, *bySeries["attic/temperature"].NumericValue)
}

// registerSubscriber wires a connection into DefaultHub subscribed to the given topic,
// returning its send channel and a cleanup func. Mirrors the pattern in hub_test.go.
func registerSubscriber(topic string) (chan any, func()) {
	conn := &websocket.Conn{}
	send := make(chan any, 16)
	DefaultHub.mu.Lock()
	DefaultHub.conns[conn] = &connInfo{conn: conn, send: send, topics: map[string]bool{topic: true}}
	DefaultHub.mu.Unlock()
	return send, func() {
		DefaultHub.mu.Lock()
		delete(DefaultHub.conns, conn)
		DefaultHub.mu.Unlock()
	}
}

func TestPublishReadings_UpdatesStoreAndBroadcasts(t *testing.T) {
	send, cleanup := registerSubscriber("current-readings")
	defer cleanup()

	readings := []gen.Reading{numericReading("office-plug", "power", 42, "2025-01-01T00:00:00Z")}
	PublishReadings(readings)

	select {
	case msg := <-send:
		assert.Equal(t, readings, msg, "subscriber receives the published readings")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected broadcast to current-readings subscriber")
	}

	bySeries := snapshotBySeries(CurrentReadingsSnapshot())
	if assert.Contains(t, bySeries, "office-plug/power", "store reflects published reading") {
		assert.Equal(t, 42.0, *bySeries["office-plug/power"].NumericValue)
	}
}

func TestBroadcastToTopic_DoesNotUpdateReadingsStore(t *testing.T) {
	// Command-status messages are broadcast straight to current-readings; they are not
	// readings and must never appear in the snapshot store.
	before := len(CurrentReadingsSnapshot())

	BroadcastToTopic("current-readings", map[string]any{"type": "command_status", "id": 1})

	assert.Equal(t, before, len(CurrentReadingsSnapshot()), "non-reading broadcast must not change the store")
}

func TestSeedReadings_PrimesStoreWithoutBroadcasting(t *testing.T) {
	send, cleanup := registerSubscriber("current-readings")
	defer cleanup()

	SeedReadings([]gen.Reading{numericReading("attic-bulb", "state", 1, "2025-01-01T00:00:00Z")})

	bySeries := snapshotBySeries(CurrentReadingsSnapshot())
	assert.Contains(t, bySeries, "attic-bulb/state", "seed populates the store")

	select {
	case msg := <-send:
		t.Fatalf("seeding must not broadcast, got %v", msg)
	case <-time.After(50 * time.Millisecond):
	}
}
