package automation

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type firing struct {
	key dueKey
	due time.Time
	at  time.Time
}

func startScheduler(t *testing.T) (*scheduler, chan firing) {
	t.Helper()
	fired := make(chan firing, 10)
	s := newScheduler(func(key dueKey, due time.Time) {
		fired <- firing{key: key, due: due, at: time.Now()}
	}, time.Now)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go s.run(ctx, func() {})
	return s, fired
}

func TestScheduler_FiresWithinASecondOfTheDueTime(t *testing.T) {
	s, fired := startScheduler(t)

	due := time.Now().Add(150 * time.Millisecond)
	s.set(triggerKey(1), due)

	select {
	case f := <-fired:
		assert.Equal(t, triggerKey(1), f.key)
		assert.True(t, due.Equal(f.due))
		assert.Less(t, f.at.Sub(due), time.Second)
		assert.False(t, f.at.Before(due), "fired before it was due")
	case <-time.After(2 * time.Second):
		t.Fatal("the entry never fired")
	}
}

func TestScheduler_RearmsWhenAnEarlierEntryArrivesAndSkipsRemovedOnes(t *testing.T) {
	s, fired := startScheduler(t)

	s.set(triggerKey(1), time.Now().Add(time.Hour))
	s.set(resumeKey(1), time.Now().Add(100*time.Millisecond))
	s.set(triggerKey(3), time.Now().Add(50*time.Millisecond))
	s.remove(triggerKey(3))

	select {
	case f := <-fired:
		assert.Equal(t, resumeKey(1), f.key)
	case <-time.After(2 * time.Second):
		t.Fatal("the earlier entry never fired")
	}

	due, ok := s.due(triggerKey(1))
	require.True(t, ok)
	assert.True(t, due.After(time.Now().Add(50*time.Minute)))
	_, ok = s.due(resumeKey(1))
	assert.False(t, ok, "a fired entry leaves the heap until it is set again")
}
