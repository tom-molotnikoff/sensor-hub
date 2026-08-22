package mqtt

import (
	"sync"
	"sync/atomic"
	"time"
)

// BrokerStats holds runtime statistics for a single MQTT broker connection.
type BrokerStats struct {
	BrokerID          int        `json:"broker_id"`
	BrokerName        string     `json:"broker_name"`
	Connected         bool       `json:"connected"`
	MessagesReceived  int64      `json:"messages_received"`
	ParseErrors       int64      `json:"parse_errors"`
	ProcessingErrors  int64      `json:"processing_errors"`
	DevicesDiscovered int64      `json:"devices_discovered"`
	LastMessageAt     *time.Time `json:"last_message_at"`
	ConnectedSince    *time.Time `json:"connected_since"`
}

// brokerCounters holds atomic counters for one broker. Thread-safe via atomics.
type brokerCounters struct {
	messagesReceived  atomic.Int64
	parseErrors       atomic.Int64
	processingErrors  atomic.Int64
	devicesDiscovered atomic.Int64
	lastMessageAt     atomic.Int64 // unix nano, 0 = never
	connectedSince    atomic.Int64 // unix nano, 0 = disconnected
}

// StatsTracker tracks per-broker runtime statistics.
type StatsTracker struct {
	mu      sync.RWMutex
	brokers map[int]*brokerCounters
	names   map[int]string
}

// NewStatsTracker creates a new stats tracker.
func NewStatsTracker() *StatsTracker {
	return &StatsTracker{
		brokers: make(map[int]*brokerCounters),
		names:   make(map[int]string),
	}
}

func (st *StatsTracker) getOrCreate(brokerID int) *brokerCounters {
	st.mu.RLock()
	bc, ok := st.brokers[brokerID]
	st.mu.RUnlock()
	if ok {
		return bc
	}

	st.mu.Lock()
	defer st.mu.Unlock()
	// Double-check under write lock
	if bc, ok = st.brokers[brokerID]; ok {
		return bc
	}
	bc = &brokerCounters{}
	st.brokers[brokerID] = bc
	return bc
}

// SetBrokerName records the display name for a broker.
func (st *StatsTracker) SetBrokerName(brokerID int, name string) {
	st.mu.Lock()
	st.names[brokerID] = name
	st.mu.Unlock()
}

// RecordMessageReceived increments the messages received counter.
func (st *StatsTracker) RecordMessageReceived(brokerID int) {
	bc := st.getOrCreate(brokerID)
	bc.messagesReceived.Add(1)
	bc.lastMessageAt.Store(time.Now().UnixNano())
}

// RecordParseError increments the parse error counter.
func (st *StatsTracker) RecordParseError(brokerID int) {
	st.getOrCreate(brokerID).parseErrors.Add(1)
}

// RecordProcessingError increments the processing error counter.
func (st *StatsTracker) RecordProcessingError(brokerID int) {
	st.getOrCreate(brokerID).processingErrors.Add(1)
}

// RecordDeviceDiscovered increments the device discovered counter.
func (st *StatsTracker) RecordDeviceDiscovered(brokerID int) {
	st.getOrCreate(brokerID).devicesDiscovered.Add(1)
}

// RecordConnected marks a broker as connected.
func (st *StatsTracker) RecordConnected(brokerID int) {
	st.getOrCreate(brokerID).connectedSince.Store(time.Now().UnixNano())
}

// RecordDisconnected marks a broker as disconnected.
func (st *StatsTracker) RecordDisconnected(brokerID int) {
	st.getOrCreate(brokerID).connectedSince.Store(0)
}

// RemoveBroker removes all tracked stats for a broker.
func (st *StatsTracker) RemoveBroker(brokerID int) {
	st.mu.Lock()
	delete(st.brokers, brokerID)
	delete(st.names, brokerID)
	st.mu.Unlock()
}

// Snapshot returns a point-in-time copy of all broker stats.
func (st *StatsTracker) Snapshot() map[int]BrokerStats {
	st.mu.RLock()
	defer st.mu.RUnlock()

	result := make(map[int]BrokerStats, len(st.brokers))
	for id, bc := range st.brokers {
		bs := BrokerStats{
			BrokerID:          id,
			BrokerName:        st.names[id],
			MessagesReceived:  bc.messagesReceived.Load(),
			ParseErrors:       bc.parseErrors.Load(),
			ProcessingErrors:  bc.processingErrors.Load(),
			DevicesDiscovered: bc.devicesDiscovered.Load(),
		}

		if ns := bc.lastMessageAt.Load(); ns > 0 {
			t := time.Unix(0, ns)
			bs.LastMessageAt = &t
		}
		if ns := bc.connectedSince.Load(); ns > 0 {
			t := time.Unix(0, ns)
			bs.ConnectedSince = &t
			bs.Connected = true
		}

		result[id] = bs
	}
	return result
}
