package database

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertRetentionRun(t *testing.T, h *Handles, automationID int, status string, startedAt time.Time, finishedAt *time.Time) int {
	t.Helper()
	result, err := h.Writer.Exec(`INSERT INTO automation_runs (automation_id, trigger_kind, status, steps_snapshot, started_at, finished_at)
		VALUES (?, 'manual', ?, '[]', ?, ?)`, automationID, status, startedAt.UTC(), finishedAt)
	require.NoError(t, err)
	id, err := result.LastInsertId()
	require.NoError(t, err)
	_, err = h.Writer.Exec(`INSERT INTO automation_run_steps (run_id, position, kind, outcome, started_at)
		VALUES (?, 0, 'wait', 'succeeded', ?)`, id, startedAt.UTC())
	require.NoError(t, err)
	return int(id)
}

func remainingIDs(t *testing.T, h *Handles, query string) []int {
	t.Helper()
	rows, err := h.Reader.Query(query)
	require.NoError(t, err)
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	require.NoError(t, rows.Err())
	return ids
}

func TestAutomationRepository_DeleteRunsFinishedBefore_KeepsActiveAndRecentRuns(t *testing.T) {
	h := openHandles(t, 1)
	result, err := h.Writer.Exec("INSERT INTO automations (name) VALUES ('lights')")
	require.NoError(t, err)
	automationID, err := result.LastInsertId()
	require.NoError(t, err)

	now := time.Now().UTC()
	longAgo := now.AddDate(0, 0, -40)
	recently := now.AddDate(0, 0, -10)
	oldFinished := insertRetentionRun(t, h, int(automationID), "succeeded", longAgo, &longAgo)
	recentFinished := insertRetentionRun(t, h, int(automationID), "failed", recently, &recently)
	oldRunning := insertRetentionRun(t, h, int(automationID), "running", longAgo, nil)
	oldWaiting := insertRetentionRun(t, h, int(automationID), "waiting", longAgo, nil)

	deleted, err := NewAutomationRepository(h, slog.Default()).DeleteRunsFinishedBefore(context.Background(), now.AddDate(0, 0, -30))

	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)
	assert.Equal(t, []int{recentFinished, oldRunning, oldWaiting},
		remainingIDs(t, h, "SELECT id FROM automation_runs ORDER BY id"))
	assert.NotContains(t, remainingIDs(t, h, "SELECT run_id FROM automation_run_steps"), oldFinished)
}

func TestSensorCommandHistoryRepository_DeleteCommandsSentBefore_DeletesOldCommandsFromPeopleAndAutomations(t *testing.T) {
	h := openHandles(t, 1)
	result, err := h.Writer.Exec("INSERT INTO sensors (name, sensor_driver, config) VALUES ('office-plug', 'mqtt-zigbee2mqtt', '{}')")
	require.NoError(t, err)
	sensorID, err := result.LastInsertId()
	require.NoError(t, err)
	result, err = h.Writer.Exec("INSERT INTO users (username, password_hash) VALUES ('alice', 'x')")
	require.NoError(t, err)
	userID, err := result.LastInsertId()
	require.NoError(t, err)
	result, err = h.Writer.Exec("INSERT INTO automations (name) VALUES ('lights')")
	require.NoError(t, err)
	automationID, err := result.LastInsertId()
	require.NoError(t, err)

	now := time.Now().UTC()
	runID := insertRetentionRun(t, h, int(automationID), "succeeded", now, &now)
	repo := NewSensorCommandHistoryRepository(h, slog.Default())
	send := func(userID *int, runID *int, sentAt time.Time) int {
		id, err := repo.AddSentCommand(context.Background(), NewCommand{
			SensorID: int(sensorID), UserID: userID, AutomationRunID: runID, Property: "state", Value: "ON",
			MQTTTopic: "zigbee2mqtt/office-plug/set", MQTTPayload: `{"state":"ON"}`, TimeoutSeconds: 10, SentAt: sentAt,
		})
		require.NoError(t, err)
		return id
	}
	person := int(userID)
	send(&person, nil, now.AddDate(0, 0, -100))
	send(nil, &runID, now.AddDate(0, 0, -100))
	recent := send(&person, nil, now.AddDate(0, 0, -80))

	deleted, err := repo.DeleteCommandsSentBefore(context.Background(), now.AddDate(0, 0, -90))

	require.NoError(t, err)
	assert.Equal(t, int64(2), deleted)
	assert.Equal(t, []int{recent}, remainingIDs(t, h, "SELECT id FROM sensor_command_history"))
}
