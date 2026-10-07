package actuation

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/readings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAckOnReading_MarksAcknowledged(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	repo := newFakeCommandTrackerRepository(database.PendingCommandRecord{
		ID:             42,
		SensorID:       7,
		Property:       "state",
		Value:          "ON",
		TimeoutSeconds: 10,
		SentAt:         now.Add(-2 * time.Second),
	})
	broadcaster := &fakeCommandStatusBroadcaster{}
	tracker := NewCommandTracker(repo, broadcaster, slog.Default())
	tracker.now = func() time.Time { return now }
	defer tracker.Close()

	outcome := tracker.Track(context.Background(), repo.mustGet(42))
	tracker.Consume(context.Background(), gen.Sensor{Id: 7}, batch(gen.Reading{
		MeasurementType: "state",
		TextState:       ptrString("OFF"),
	}))

	assert.Equal(t, CommandStatusAcknowledged, <-outcome)
	command := repo.mustGet(42)
	require.Equal(t, CommandStatusAcknowledged, command.Status)
	require.NotNil(t, command.AcknowledgedAt)
	require.Equal(t, now, *command.AcknowledgedAt)
	require.NotNil(t, command.AcknowledgedValue)
	assert.Equal(t, "OFF", *command.AcknowledgedValue)

	require.Len(t, broadcaster.sent(), 1)
	assert.Equal(t, "command_status", broadcaster.sent()[0].Type)
	assert.Equal(t, CommandStatusAcknowledged, broadcaster.sent()[0].Status)
	assert.Equal(t, 42, broadcaster.sent()[0].ID)
}

func TestAckTimeout_MarksTimedOut(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	repo := newFakeCommandTrackerRepository(database.PendingCommandRecord{
		ID:             43,
		SensorID:       7,
		Property:       "state",
		Value:          "ON",
		TimeoutSeconds: 10,
		SentAt:         now,
	})
	broadcaster := &fakeCommandStatusBroadcaster{}
	tracker := NewCommandTracker(repo, broadcaster, slog.Default())
	tracker.now = func() time.Time { return now }
	tracker.schedule = func(_ time.Duration, fn func()) func() {
		go fn()
		return func() {}
	}

	outcome := tracker.Track(context.Background(), repo.mustGet(43))

	require.Eventually(t, func() bool {
		command := repo.mustGet(43)
		return command.Status == CommandStatusTimedOut && len(broadcaster.sent()) == 1
	}, time.Second, 10*time.Millisecond)
	assert.Equal(t, CommandStatusTimedOut, <-outcome)

	assert.Equal(t, "command_status", broadcaster.sent()[0].Type)
	assert.Equal(t, CommandStatusTimedOut, broadcaster.sent()[0].Status)
	assert.Equal(t, 43, broadcaster.sent()[0].ID)
}

func TestAckOnReading_MatchesPropertyOnly(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	repo := newFakeCommandTrackerRepository(database.PendingCommandRecord{
		ID:             44,
		SensorID:       7,
		Property:       "state",
		Value:          "ON",
		TimeoutSeconds: 10,
		SentAt:         now,
	})
	broadcaster := &fakeCommandStatusBroadcaster{}
	tracker := NewCommandTracker(repo, broadcaster, slog.Default())
	tracker.now = func() time.Time { return now }
	defer tracker.Close()

	tracker.Track(context.Background(), repo.mustGet(44))
	tracker.Consume(context.Background(), gen.Sensor{Id: 7}, batch(gen.Reading{
		MeasurementType: "state",
		TextState:       ptrString("OFF"),
	}))

	command := repo.mustGet(44)
	require.Equal(t, CommandStatusAcknowledged, command.Status)
	require.NotNil(t, command.AcknowledgedValue)
	assert.Equal(t, "OFF", *command.AcknowledgedValue)
}

func TestRecoverPending_TimesOutExpiredCommandsAndTracksRemainingOnes(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	repo := newFakeCommandTrackerRepository(
		database.PendingCommandRecord{
			ID:             45,
			SensorID:       7,
			Property:       "state",
			Value:          "ON",
			TimeoutSeconds: 1,
			SentAt:         now.Add(-2 * time.Second),
		},
		database.PendingCommandRecord{
			ID:             46,
			SensorID:       8,
			Property:       "state",
			Value:          "OFF",
			TimeoutSeconds: 10,
			SentAt:         now,
		},
	)
	broadcaster := &fakeCommandStatusBroadcaster{}
	tracker := NewCommandTracker(repo, broadcaster, slog.Default())
	tracker.now = func() time.Time { return now }
	tracker.schedule = func(_ time.Duration, _ func()) func() { return func() {} }
	defer tracker.Close()

	require.NoError(t, tracker.RecoverPending(context.Background()))

	expired := repo.mustGet(45)
	assert.Equal(t, CommandStatusTimedOut, expired.Status)

	tracker.Consume(context.Background(), gen.Sensor{Id: 8}, batch(gen.Reading{
		MeasurementType: "state",
		TextState:       ptrString("OFF"),
	}))

	recovered := repo.mustGet(46)
	assert.Equal(t, CommandStatusAcknowledged, recovered.Status)
	require.Len(t, broadcaster.sent(), 2)
	assert.Equal(t, CommandStatusTimedOut, broadcaster.sent()[0].Status)
	assert.Equal(t, CommandStatusAcknowledged, broadcaster.sent()[1].Status)
}

func TestAwait_GivesTheOutcomeOfACommandRecoveredAfterARestart(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	repo := newFakeCommandTrackerRepository(database.PendingCommandRecord{
		ID:             47,
		SensorID:       7,
		Property:       "state",
		Value:          "ON",
		TimeoutSeconds: 10,
		SentAt:         now,
	})
	tracker := NewCommandTracker(repo, &fakeCommandStatusBroadcaster{}, slog.Default())
	tracker.now = func() time.Time { return now }
	defer tracker.Close()
	require.NoError(t, tracker.RecoverPending(context.Background()))

	outcome, ok := tracker.Await(47)
	require.True(t, ok)
	tracker.Consume(context.Background(), gen.Sensor{Id: 7}, batch(gen.Reading{MeasurementType: "state", TextState: ptrString("ON")}))

	assert.Equal(t, CommandStatusAcknowledged, <-outcome)
	_, ok = tracker.Await(47)
	assert.False(t, ok, "a settled command is no longer tracked")
}

func TestAwait_GivesTheOutcomeToTheSenderToo(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	repo := newFakeCommandTrackerRepository(database.PendingCommandRecord{
		ID:             48,
		SensorID:       7,
		Property:       "state",
		Value:          "ON",
		TimeoutSeconds: 10,
		SentAt:         now,
	})
	tracker := NewCommandTracker(repo, &fakeCommandStatusBroadcaster{}, slog.Default())
	tracker.now = func() time.Time { return now }
	defer tracker.Close()

	sent := tracker.Track(context.Background(), repo.mustGet(48))
	awaited, ok := tracker.Await(48)
	require.True(t, ok)
	tracker.Consume(context.Background(), gen.Sensor{Id: 7}, batch(gen.Reading{MeasurementType: "state", TextState: ptrString("ON")}))

	assert.Equal(t, CommandStatusAcknowledged, <-sent)
	assert.Equal(t, CommandStatusAcknowledged, <-awaited)
}

type fakeCommandTrackerRepository struct {
	mu       sync.Mutex
	commands map[int]database.PendingCommandRecord
}

func newFakeCommandTrackerRepository(commands ...database.PendingCommandRecord) *fakeCommandTrackerRepository {
	repo := &fakeCommandTrackerRepository{
		commands: make(map[int]database.PendingCommandRecord, len(commands)),
	}
	for _, command := range commands {
		command.Status = CommandStatusSent
		repo.commands[command.ID] = command
	}
	return repo
}

func (r *fakeCommandTrackerRepository) MarkAcknowledged(_ context.Context, id int, acknowledgedValue string, acknowledgedAt time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	command, ok := r.commands[id]
	if !ok || command.Status != CommandStatusSent {
		return false, nil
	}

	command.Status = CommandStatusAcknowledged
	command.AcknowledgedAt = &acknowledgedAt
	command.AcknowledgedValue = &acknowledgedValue
	r.commands[id] = command
	return true, nil
}

func (r *fakeCommandTrackerRepository) MarkTimedOut(_ context.Context, id int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	command, ok := r.commands[id]
	if !ok || command.Status != CommandStatusSent {
		return false, nil
	}

	command.Status = CommandStatusTimedOut
	r.commands[id] = command
	return true, nil
}

func (r *fakeCommandTrackerRepository) MarkFailed(_ context.Context, id int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	command, ok := r.commands[id]
	if !ok || command.Status != CommandStatusSent {
		return false, nil
	}

	command.Status = CommandStatusFailed
	r.commands[id] = command
	return true, nil
}

func (r *fakeCommandTrackerRepository) ListPendingCommands(_ context.Context) ([]database.PendingCommandRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	commands := make([]database.PendingCommandRecord, 0, len(r.commands))
	for _, command := range r.commands {
		if command.Status == CommandStatusSent {
			commands = append(commands, command)
		}
	}
	return commands, nil
}

func (r *fakeCommandTrackerRepository) mustGet(id int) database.PendingCommandRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.commands[id]
}

type fakeCommandStatusBroadcaster struct {
	mu       sync.Mutex
	messages []CommandStatusMessage
}

func (b *fakeCommandStatusBroadcaster) BroadcastCommandStatus(message CommandStatusMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.messages = append(b.messages, message)
}

func (b *fakeCommandStatusBroadcaster) sent() []CommandStatusMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.messages)
}

func ptrString(value string) *string {
	return &value
}

func batch(stored ...gen.Reading) []readings.Reading {
	passed := make([]readings.Reading, len(stored))
	for i, reading := range stored {
		passed[i] = readings.Reading{Reading: reading}
	}
	return passed
}

func TestAckOnReading_AttachesTheRunThatSentTheCommandToTheReading(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	runID := 12
	repo := newFakeCommandTrackerRepository(
		database.PendingCommandRecord{ID: 50, SensorID: 7, AutomationRunID: &runID, Property: "state", Value: "ON", TimeoutSeconds: 10, SentAt: now},
		database.PendingCommandRecord{ID: 51, SensorID: 7, Property: "brightness", Value: "100", TimeoutSeconds: 10, SentAt: now},
	)
	tracker := NewCommandTracker(repo, &fakeCommandStatusBroadcaster{}, slog.Default())
	tracker.now = func() time.Time { return now }
	defer tracker.Close()
	tracker.Track(context.Background(), repo.mustGet(50))
	tracker.Track(context.Background(), repo.mustGet(51))

	brightness := 100.0
	consumed := batch(
		gen.Reading{MeasurementType: "state", TextState: ptrString("ON")},
		gen.Reading{MeasurementType: "brightness", NumericValue: &brightness},
		gen.Reading{MeasurementType: "linkquality", NumericValue: &brightness},
	)
	tracker.Consume(context.Background(), gen.Sensor{Id: 7}, consumed)

	assert.Equal(t, &runID, consumed[0].CauseRunID)
	assert.Nil(t, consumed[1].CauseRunID, "a person's command is no automation's")
	assert.Nil(t, consumed[2].CauseRunID, "the reading acknowledged no command")
}
