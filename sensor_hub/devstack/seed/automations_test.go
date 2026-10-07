package main

import (
	"context"
	"testing"

	"example/sensorHub/automation"
	database "example/sensorHub/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seededAutomationsIn(t *testing.T, db *database.Handles) []automation.Automation {
	t.Helper()
	automations, err := database.NewAutomationRepository(db, discardLogger()).ListAutomations(context.Background())
	require.NoError(t, err)
	return automations
}

func TestSeed_CreatesTheAutomations(t *testing.T) {
	db := openTempDatabase(t)

	runSeed(t, db)

	enabled := map[string]bool{}
	for _, a := range seededAutomationsIn(t, db) {
		enabled[a.Name] = a.Enabled
	}
	for _, seeded := range seededAutomations {
		require.Contains(t, enabled, seeded.name)
		assert.Equal(t, seeded.enabled, enabled[seeded.name], seeded.name)
	}
}

func TestSeed_EverySetStepSwitchesAWritableProperty(t *testing.T) {
	db := openTempDatabase(t)
	runSeed(t, db)
	sensors := newSensorService(db)

	for _, a := range seededAutomationsIn(t, db) {
		for _, step := range a.Steps {
			if step.Kind != automation.StepSet {
				continue
			}
			capabilities, err := sensors.ServiceGetSensorCapabilities(context.Background(), step.SensorID)
			require.NoError(t, err)
			var properties []string
			for _, capability := range capabilities {
				properties = append(properties, capability.Property)
			}
			assert.Contains(t, properties, step.Property, "%s would be Broken when the hub starts", a.Name)
		}
	}
}

func TestSeed_RunHistoryShowsEveryFinishedStatus(t *testing.T) {
	db := openTempDatabase(t)
	runSeed(t, db)
	repository := database.NewAutomationRepository(db, discardLogger())
	ctx := context.Background()

	statuses := map[automation.RunStatus]bool{}
	for _, a := range seededAutomationsIn(t, db) {
		runs, err := repository.ListRuns(ctx, a.ID)
		require.NoError(t, err)
		for _, run := range runs {
			statuses[run.Status] = true
		}
	}
	assert.Equal(t, map[automation.RunStatus]bool{
		automation.RunSucceeded: true, automation.RunFailed: true, automation.RunCancelled: true,
		automation.RunMissed: true, automation.RunSkipped: true,
	}, statuses)

	states, err := repository.RunStates(ctx)
	require.NoError(t, err)
	failing := 0
	for _, state := range states {
		if state.LastRunFailed {
			failing++
		}
	}
	assert.Equal(t, 1, failing, "one automation shows the last run failed flag")

	var unlinked int
	require.NoError(t, db.Reader.QueryRow(`SELECT COUNT(*) FROM automation_run_steps s
		LEFT JOIN sensor_command_history c ON c.id = s.command_id
		WHERE s.kind = 'set' AND c.automation_id IS NULL`).Scan(&unlinked))
	assert.Zero(t, unlinked, "every set step links a command that shows its automation")
}
