package ws

import (
	"sync"

	gen "example/sensorHub/gen"
)

// ReadingsStore holds the latest reading per (sensorName, measurementType) series,
// last-write-wins. It is the authoritative source for the current-readings connect
// snapshot, so a client connecting never needs a database query to learn current state.
type ReadingsStore struct {
	mu     sync.RWMutex
	latest map[string]map[string]gen.Reading
}

func NewReadingsStore() *ReadingsStore {
	return &ReadingsStore{latest: make(map[string]map[string]gen.Reading)}
}

// Update records each reading as the latest value for its series.
func (s *ReadingsStore) Update(readings []gen.Reading) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, reading := range readings {
		byType, ok := s.latest[reading.SensorName]
		if !ok {
			byType = make(map[string]gen.Reading)
			s.latest[reading.SensorName] = byType
		}
		byType[reading.MeasurementType] = reading
	}
}

// Snapshot returns the current latest reading for every known series.
func (s *ReadingsStore) Snapshot() []gen.Reading {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]gen.Reading, 0)
	for _, byType := range s.latest {
		for _, reading := range byType {
			out = append(out, reading)
		}
	}
	return out
}

// currentReadingsTopic is the websocket topic carrying live reading updates and the
// connect snapshot.
const currentReadingsTopic = "current-readings"

// DefaultReadingsStore backs the application-wide current-readings snapshot.
var DefaultReadingsStore = NewReadingsStore()

// PublishReadings is the single entry point for emitting readings: it updates the
// in-memory store and broadcasts to the current-readings topic in one step, so the
// snapshot served on connect can never drift from the live stream. Only actual
// readings flow through here; non-reading messages (e.g. command status) broadcast
// directly via BroadcastToTopic and must not touch the store.
func PublishReadings(readings []gen.Reading) {
	DefaultReadingsStore.Update(readings)
	BroadcastToTopic(currentReadingsTopic, readings)
}

// CurrentReadingsSnapshot returns the latest readings held in memory, for serving
// to a freshly-connected client without a database query.
func CurrentReadingsSnapshot() []gen.Reading {
	return DefaultReadingsStore.Snapshot()
}

// SeedReadings primes the store at startup (e.g. from the database) so a freshly
// started server serves correct state before any new readings flow. It does not
// broadcast, since there are no clients to notify yet.
func SeedReadings(readings []gen.Reading) {
	DefaultReadingsStore.Update(readings)
}
