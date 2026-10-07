package service

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type historyFixture struct {
	service *cleanupService
	handles *database.Handles
	runs    struct{ oldFinished, recentFinished, oldWaiting int }
	cmds    struct{ old, recent int }
}

func newHistoryFixture(t *testing.T) historyFixture {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handles, err := database.Open(&appProps.ApplicationConfiguration{
		DatabasePath:              filepath.Join(t.TempDir(), "cleanup.db"),
		DatabaseReaderConnections: 1,
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { handles.Close() })

	commandRepo := database.NewSensorCommandHistoryRepository(handles, logger)
	f := historyFixture{
		service: NewCleanupService(nil, nil, nil, nil, nil,
			database.NewAutomationRepository(handles, logger), commandRepo, nil, nil, logger).(*cleanupService),
		handles: handles,
	}

	_, err = handles.Writer.Exec("INSERT INTO sensors (id, name, sensor_driver, config) VALUES (1, 'office-plug', 'mqtt-zigbee2mqtt', '{}')")
	require.NoError(t, err)
	_, err = handles.Writer.Exec("INSERT INTO automations (id, name) VALUES (1, 'Evening lights')")
	require.NoError(t, err)

	now := time.Now().UTC()
	insertRun := func(status string, startedAt time.Time, finishedAt *time.Time) int {
		result, err := handles.Writer.Exec(`INSERT INTO automation_runs (automation_id, trigger_kind, status, steps_snapshot, started_at, finished_at)
			VALUES (1, 'schedule', ?, '[]', ?, ?)`, status, startedAt, finishedAt)
		require.NoError(t, err)
		id, err := result.LastInsertId()
		require.NoError(t, err)
		_, err = handles.Writer.Exec(`INSERT INTO automation_run_steps (run_id, position, kind, outcome, started_at)
			VALUES (?, 0, 'wait', 'succeeded', ?)`, id, startedAt)
		require.NoError(t, err)
		return int(id)
	}
	fortyDaysAgo := now.AddDate(0, 0, -40)
	tenDaysAgo := now.AddDate(0, 0, -10)
	f.runs.oldFinished = insertRun("succeeded", fortyDaysAgo, &fortyDaysAgo)
	f.runs.recentFinished = insertRun("failed", tenDaysAgo, &tenDaysAgo)
	f.runs.oldWaiting = insertRun("waiting", fortyDaysAgo, nil)

	send := func(sentAt time.Time) int {
		id, err := commandRepo.AddSentCommand(context.Background(), database.NewCommand{
			SensorID: 1, Property: "state", Value: "ON",
			MQTTTopic: "zigbee2mqtt/office-plug/set", MQTTPayload: `{"state":"ON"}`, TimeoutSeconds: 10, SentAt: sentAt,
		})
		require.NoError(t, err)
		return id
	}
	f.cmds.old = send(now.AddDate(0, 0, -100))
	f.cmds.recent = send(now.AddDate(0, 0, -80))
	return f
}

func (f historyFixture) ids(t *testing.T, query string) []int {
	t.Helper()
	rows, err := f.handles.Reader.Query(query)
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

func TestCleanupService_PerformCleanup_DeletesRunsAndCommandsPastTheirRetention(t *testing.T) {
	f := newHistoryFixture(t)

	err := f.service.performCleanup(context.Background(), &appProps.ApplicationConfiguration{AutomationHistoryRetentionDays: 30, CommandHistoryRetentionDays: 90})

	require.NoError(t, err)
	assert.Equal(t, []int{f.runs.recentFinished, f.runs.oldWaiting}, f.ids(t, "SELECT id FROM automation_runs ORDER BY id"))
	assert.Equal(t, []int{f.runs.recentFinished, f.runs.oldWaiting}, f.ids(t, "SELECT run_id FROM automation_run_steps ORDER BY run_id"))
	assert.Equal(t, []int{f.cmds.recent}, f.ids(t, "SELECT id FROM sensor_command_history ORDER BY id"))
}

func TestCleanupService_PerformCleanup_ZeroRetentionKeepsRunsAndCommands(t *testing.T) {
	f := newHistoryFixture(t)

	err := f.service.performCleanup(context.Background(), &appProps.ApplicationConfiguration{})

	require.NoError(t, err)
	allRuns := []int{f.runs.oldFinished, f.runs.recentFinished, f.runs.oldWaiting}
	assert.Equal(t, allRuns, f.ids(t, "SELECT id FROM automation_runs ORDER BY id"))
	assert.Equal(t, allRuns, f.ids(t, "SELECT run_id FROM automation_run_steps ORDER BY run_id"))
	assert.Equal(t, []int{f.cmds.old, f.cmds.recent}, f.ids(t, "SELECT id FROM sensor_command_history ORDER BY id"))
}
