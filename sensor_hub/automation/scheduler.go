package automation

import (
	"container/heap"
	"context"
	"sync"
	"time"
)

const maxSleep = time.Minute

// Trigger IDs and run IDs overlap, so each kind of entry has its own key space.
type dueKey struct {
	kind dueKind
	id   int
}

type dueKind int

const (
	dueTrigger dueKind = iota
	dueResume
)

func triggerKey(triggerID int) dueKey { return dueKey{kind: dueTrigger, id: triggerID} }

func resumeKey(runID int) dueKey { return dueKey{kind: dueResume, id: runID} }

type scheduler struct {
	fire func(key dueKey, due time.Time)
	now  func() time.Time

	mu      sync.Mutex
	entries map[dueKey]*dueEntry
	queue   dueQueue
	wake    chan struct{}
}

func newScheduler(fire func(key dueKey, due time.Time), now func() time.Time) *scheduler {
	return &scheduler{
		fire:    fire,
		now:     now,
		entries: make(map[dueKey]*dueEntry),
		wake:    make(chan struct{}, 1),
	}
}

func (s *scheduler) set(key dueKey, due time.Time) {
	s.mu.Lock()
	if entry, ok := s.entries[key]; ok {
		entry.due = due
		heap.Fix(&s.queue, entry.index)
	} else {
		entry := &dueEntry{key: key, due: due}
		s.entries[key] = entry
		heap.Push(&s.queue, entry)
	}
	s.mu.Unlock()
	s.rearm()
}

func (s *scheduler) remove(key dueKey) {
	s.mu.Lock()
	if entry, ok := s.entries[key]; ok {
		heap.Remove(&s.queue, entry.index)
		delete(s.entries, key)
	}
	s.mu.Unlock()
	s.rearm()
}

func (s *scheduler) due(key dueKey) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[key]
	if !ok {
		return time.Time{}, false
	}
	return entry.due, true
}

func (s *scheduler) rearm() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *scheduler) run(ctx context.Context, healthy func()) {
	for {
		var timer *time.Timer
		var timerC <-chan time.Time
		if earliest, ok := s.earliest(); ok {
			// Timers measure elapsed time, not the wall clock, so a clock
			// that jumps forward would make a long sleep end late. Waking at
			// least once a minute re-reads the clock.
			timer = time.NewTimer(min(max(earliest.Sub(s.now()), 0), maxSleep))
			timerC = timer.C
		}

		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-s.wake:
			if timer != nil {
				timer.Stop()
			}
		case <-timerC:
			for _, entry := range s.popDue(s.now()) {
				s.fire(entry.key, entry.due)
			}
			healthy()
		}
	}
}

func (s *scheduler) earliest() (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.queue) == 0 {
		return time.Time{}, false
	}
	return s.queue[0].due, true
}

func (s *scheduler) popDue(now time.Time) []*dueEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	var due []*dueEntry
	for len(s.queue) > 0 && !s.queue[0].due.After(now) {
		entry := heap.Pop(&s.queue).(*dueEntry)
		delete(s.entries, entry.key)
		due = append(due, entry)
	}
	return due
}

type dueEntry struct {
	key   dueKey
	due   time.Time
	index int
}

type dueQueue []*dueEntry

func (q dueQueue) Len() int           { return len(q) }
func (q dueQueue) Less(i, j int) bool { return q[i].due.Before(q[j].due) }
func (q dueQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].index = i
	q[j].index = j
}

func (q *dueQueue) Push(x any) {
	entry := x.(*dueEntry)
	entry.index = len(*q)
	*q = append(*q, entry)
}

func (q *dueQueue) Pop() any {
	old := *q
	entry := old[len(old)-1]
	old[len(old)-1] = nil
	*q = old[:len(old)-1]
	return entry
}
