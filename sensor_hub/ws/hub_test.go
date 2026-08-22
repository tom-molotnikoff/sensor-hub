package ws

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func createTestWebSocketConn(t *testing.T) (*websocket.Conn, *websocket.Conn, func()) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Failed to upgrade: %v", err)
		}
		defer conn.Close()
		for {
			if _, _, err := conn.NextReader(); err != nil {
				break
			}
		}
	}))

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}

	cleanup := func() {
		clientConn.Close()
		server.Close()
	}

	return clientConn, nil, cleanup
}

func TestNewHub(t *testing.T) {
	hub := NewHub(slog.Default())
	assert.NotNil(t, hub)
	assert.NotNil(t, hub.conns)
	assert.Equal(t, 0, len(hub.conns))
	assert.Equal(t, 5*time.Second, hub.writeTimeout)
}

func TestHub_Count(t *testing.T) {
	hub := NewHub(slog.Default())
	assert.Equal(t, 0, hub.Count())

	mockConn1 := &websocket.Conn{}
	mockConn2 := &websocket.Conn{}

	hub.conns[mockConn1] = &connInfo{
		conn:   mockConn1,
		send:   make(chan any, 16),
		topics: make(map[string]bool),
	}
	assert.Equal(t, 1, hub.Count())

	hub.conns[mockConn2] = &connInfo{
		conn:   mockConn2,
		send:   make(chan any, 16),
		topics: make(map[string]bool),
	}
	assert.Equal(t, 2, hub.Count())

	delete(hub.conns, mockConn1)
	assert.Equal(t, 1, hub.Count())
}

func TestHub_Subscribe(t *testing.T) {
	hub := NewHub(slog.Default())
	mockConn := &websocket.Conn{}

	hub.conns[mockConn] = &connInfo{
		conn:   mockConn,
		send:   make(chan any, 16),
		topics: make(map[string]bool),
	}

	hub.Subscribe(mockConn, "test-topic")
	assert.True(t, hub.conns[mockConn].topics["test-topic"])

	hub.Subscribe(mockConn, "another-topic")
	assert.True(t, hub.conns[mockConn].topics["test-topic"])
	assert.True(t, hub.conns[mockConn].topics["another-topic"])
}

func TestHub_Subscribe_EmptyTopic(t *testing.T) {
	hub := NewHub(slog.Default())
	mockConn := &websocket.Conn{}

	hub.conns[mockConn] = &connInfo{
		conn:   mockConn,
		send:   make(chan any, 16),
		topics: make(map[string]bool),
	}

	hub.Subscribe(mockConn, "")
	assert.False(t, hub.conns[mockConn].topics[""])
	assert.Equal(t, 0, len(hub.conns[mockConn].topics))
}

func TestHub_Subscribe_NonExistentConnection(t *testing.T) {
	hub := NewHub(slog.Default())
	mockConn := &websocket.Conn{}

	hub.Subscribe(mockConn, "test-topic")
	assert.Nil(t, hub.conns[mockConn])
}

func TestHub_Unsubscribe(t *testing.T) {
	hub := NewHub(slog.Default())
	mockConn := &websocket.Conn{}

	hub.conns[mockConn] = &connInfo{
		conn:   mockConn,
		send:   make(chan any, 16),
		topics: map[string]bool{"topic1": true, "topic2": true},
	}

	hub.Unsubscribe(mockConn, "topic1")
	assert.False(t, hub.conns[mockConn].topics["topic1"])
	assert.True(t, hub.conns[mockConn].topics["topic2"])
}

func TestHub_Unsubscribe_EmptyTopic(t *testing.T) {
	hub := NewHub(slog.Default())
	mockConn := &websocket.Conn{}

	hub.conns[mockConn] = &connInfo{
		conn:   mockConn,
		send:   make(chan any, 16),
		topics: map[string]bool{"topic1": true},
	}

	hub.Unsubscribe(mockConn, "")
	assert.True(t, hub.conns[mockConn].topics["topic1"])
}

func TestHub_Unsubscribe_NonExistentConnection(t *testing.T) {
	hub := NewHub(slog.Default())
	mockConn := &websocket.Conn{}

	hub.Unsubscribe(mockConn, "test-topic")
	assert.Nil(t, hub.conns[mockConn])
}

func TestHub_BroadcastToTopic_EmptyTopic(t *testing.T) {
	hub := NewHub(slog.Default())
	mockConn := &websocket.Conn{}

	hub.conns[mockConn] = &connInfo{
		conn:   mockConn,
		send:   make(chan any, 16),
		topics: map[string]bool{"topic1": true},
	}

	hub.BroadcastToTopic("", "test message")
}

func TestHub_BroadcastToTopic_NoSubscribers(t *testing.T) {
	hub := NewHub(slog.Default())
	mockConn := &websocket.Conn{}

	hub.conns[mockConn] = &connInfo{
		conn:   mockConn,
		send:   make(chan any, 16),
		topics: map[string]bool{"other-topic": true},
	}

	hub.BroadcastToTopic("test-topic", "test message")

	select {
	case msg := <-hub.conns[mockConn].send:
		t.Errorf("expected no message, got %v", msg)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestHub_BroadcastToTopic_WithSubscribers(t *testing.T) {
	hub := NewHub(slog.Default())
	mockConn1 := &websocket.Conn{}
	mockConn2 := &websocket.Conn{}

	hub.conns[mockConn1] = &connInfo{
		conn:   mockConn1,
		send:   make(chan any, 16),
		topics: map[string]bool{"test-topic": true},
	}
	hub.conns[mockConn2] = &connInfo{
		conn:   mockConn2,
		send:   make(chan any, 16),
		topics: map[string]bool{"other-topic": true},
	}

	hub.BroadcastToTopic("test-topic", "test message")

	select {
	case msg := <-hub.conns[mockConn1].send:
		assert.Equal(t, "test message", msg)
	case <-time.After(100 * time.Millisecond):
		t.Error("expected message for subscriber, got none")
	}

	select {
	case msg := <-hub.conns[mockConn2].send:
		t.Errorf("expected no message for non-subscriber, got %v", msg)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestHub_BroadcastToTopic_MultipleSubscribers(t *testing.T) {
	hub := NewHub(slog.Default())
	mockConn1 := &websocket.Conn{}
	mockConn2 := &websocket.Conn{}
	mockConn3 := &websocket.Conn{}

	hub.conns[mockConn1] = &connInfo{
		conn:   mockConn1,
		send:   make(chan any, 16),
		topics: map[string]bool{"test-topic": true},
	}
	hub.conns[mockConn2] = &connInfo{
		conn:   mockConn2,
		send:   make(chan any, 16),
		topics: map[string]bool{"test-topic": true, "other-topic": true},
	}
	hub.conns[mockConn3] = &connInfo{
		conn:   mockConn3,
		send:   make(chan any, 16),
		topics: map[string]bool{"other-topic": true},
	}

	hub.BroadcastToTopic("test-topic", "broadcast message")

	select {
	case msg := <-hub.conns[mockConn1].send:
		assert.Equal(t, "broadcast message", msg)
	case <-time.After(100 * time.Millisecond):
		t.Error("expected message for conn1")
	}

	select {
	case msg := <-hub.conns[mockConn2].send:
		assert.Equal(t, "broadcast message", msg)
	case <-time.After(100 * time.Millisecond):
		t.Error("expected message for conn2")
	}

	select {
	case msg := <-hub.conns[mockConn3].send:
		t.Errorf("expected no message for conn3, got %v", msg)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestHub_BroadcastToTopic_FullBuffer(t *testing.T) {
	hub := NewHub(slog.Default())
	mockConn := &websocket.Conn{}

	sendChan := make(chan any, 2)
	hub.conns[mockConn] = &connInfo{
		conn:   mockConn,
		send:   sendChan,
		topics: map[string]bool{"test-topic": true},
	}

	sendChan <- "msg1"
	sendChan <- "msg2"

	initialCount := hub.Count()

	defer func() {
		if r := recover(); r != nil {
			t.Logf("Expected panic when closing mock connection: %v", r)
		}
	}()

	hub.BroadcastToTopic("test-topic", "msg3")

	assert.Equal(t, initialCount-1, hub.Count())
	assert.Nil(t, hub.conns[mockConn])
}

func TestHub_Unregister(t *testing.T) {
	hub := NewHub(slog.Default())
	clientConn, _, cleanup := createTestWebSocketConn(t)
	defer cleanup()

	sendChan := make(chan any, 16)
	hub.conns[clientConn] = &connInfo{
		conn:   clientConn,
		send:   sendChan,
		topics: map[string]bool{"test-topic": true},
	}

	assert.Equal(t, 1, hub.Count())

	hub.Unregister(clientConn)

	assert.Equal(t, 0, hub.Count())
	assert.Nil(t, hub.conns[clientConn])

	select {
	case _, ok := <-sendChan:
		assert.False(t, ok, "expected send channel to be closed")
	case <-time.After(100 * time.Millisecond):
		t.Error("expected send channel to be closed")
	}
}

func TestHub_Unregister_NonExistentConnection(t *testing.T) {
	hub := NewHub(slog.Default())
	mockConn := &websocket.Conn{}

	assert.Equal(t, 0, hub.Count())
	hub.Unregister(mockConn)
	assert.Equal(t, 0, hub.Count())
}

func TestHub_Unregister_AlreadyUnregistered(t *testing.T) {
	hub := NewHub(slog.Default())
	clientConn, _, cleanup := createTestWebSocketConn(t)
	defer cleanup()

	hub.conns[clientConn] = &connInfo{
		conn:   clientConn,
		send:   make(chan any, 16),
		topics: make(map[string]bool),
	}

	hub.Unregister(clientConn)
	assert.Equal(t, 0, hub.Count())

	hub.Unregister(clientConn)
	assert.Equal(t, 0, hub.Count())
}

func TestHub_ConcurrentOperations(t *testing.T) {
	hub := NewHub(slog.Default())
	var wg sync.WaitGroup

	numGoroutines := 10
	wg.Add(numGoroutines * 3)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			mockConn := &websocket.Conn{}
			hub.mu.Lock()
			hub.conns[mockConn] = &connInfo{
				conn:   mockConn,
				send:   make(chan any, 16),
				topics: make(map[string]bool),
			}
			hub.mu.Unlock()
		}()

		go func() {
			defer wg.Done()
			hub.Count()
		}()

		go func() {
			defer wg.Done()
			hub.BroadcastToTopic("test-topic", "message")
		}()
	}

	wg.Wait()
}

func TestHub_SubscribeMultipleTopics(t *testing.T) {
	hub := NewHub(slog.Default())
	mockConn := &websocket.Conn{}

	hub.conns[mockConn] = &connInfo{
		conn:   mockConn,
		send:   make(chan any, 16),
		topics: make(map[string]bool),
	}

	topics := []string{"topic1", "topic2", "topic3", "topic4", "topic5"}
	for _, topic := range topics {
		hub.Subscribe(mockConn, topic)
	}

	for _, topic := range topics {
		assert.True(t, hub.conns[mockConn].topics[topic])
	}
	assert.Equal(t, len(topics), len(hub.conns[mockConn].topics))
}

func TestHub_BroadcastToTopic_DifferentMessageTypes(t *testing.T) {
	hub := NewHub(slog.Default())
	mockConn := &websocket.Conn{}

	hub.conns[mockConn] = &connInfo{
		conn:   mockConn,
		send:   make(chan any, 16),
		topics: map[string]bool{"test-topic": true},
	}

	testCases := []any{
		"string message",
		123,
		45.67,
		true,
		map[string]any{"key": "value"},
		[]string{"item1", "item2"},
	}

	for _, testMsg := range testCases {
		hub.BroadcastToTopic("test-topic", testMsg)

		select {
		case msg := <-hub.conns[mockConn].send:
			assert.Equal(t, testMsg, msg)
		case <-time.After(100 * time.Millisecond):
			t.Errorf("expected message %v, got none", testMsg)
		}
	}
}

func TestDefaultHub(t *testing.T) {
	assert.NotNil(t, DefaultHub)
	assert.IsType(t, &Hub{}, DefaultHub)
}

func TestHub_RegisterWithEmptyTopics(t *testing.T) {
	hub := NewHub(slog.Default())
	clientConn, _, cleanup := createTestWebSocketConn(t)
	defer cleanup()

	go func() {
		time.Sleep(50 * time.Millisecond)
		hub.Unregister(clientConn)
	}()

	hub.Register(clientConn, []string{})

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, hub.Count())
}

func TestHub_RegisterWithMixedTopics(t *testing.T) {
	hub := NewHub(slog.Default())
	clientConn, _, cleanup := createTestWebSocketConn(t)
	defer cleanup()

	go func() {
		time.Sleep(50 * time.Millisecond)
		hub.Unregister(clientConn)
	}()

	hub.Register(clientConn, []string{"topic1", "", "topic2", "", "topic3"})

	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, 1, hub.Count())

	hub.mu.Lock()
	ci := hub.conns[clientConn]
	hub.mu.Unlock()

	assert.NotNil(t, ci)
	assert.True(t, ci.topics["topic1"])
	assert.True(t, ci.topics["topic2"])
	assert.True(t, ci.topics["topic3"])
	assert.False(t, ci.topics[""])
	assert.Equal(t, 3, len(ci.topics))

	time.Sleep(100 * time.Millisecond)
}

func TestHub_RegisterDuplicateConnection(t *testing.T) {
	hub := NewHub(slog.Default())
	clientConn, _, cleanup := createTestWebSocketConn(t)
	defer cleanup()

	go func() {
		time.Sleep(50 * time.Millisecond)
		hub.Unregister(clientConn)
	}()

	hub.Register(clientConn, []string{"topic1"})
	time.Sleep(20 * time.Millisecond)
	firstCount := hub.Count()

	hub.Register(clientConn, []string{"topic2"})
	secondCount := hub.Count()

	assert.Equal(t, firstCount, secondCount)

	time.Sleep(100 * time.Millisecond)
}
