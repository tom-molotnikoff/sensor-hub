package database

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertCommandHistorySensor(t *testing.T, db *sql.DB) int {
	t.Helper()
	_, err := db.Exec("INSERT INTO sensors (name, sensor_driver, config) VALUES ('office-plug', 'mqtt-zigbee2mqtt', '{}')")
	require.NoError(t, err)
	var sensorID int
	require.NoError(t, db.QueryRow("SELECT id FROM sensors WHERE name = 'office-plug'").Scan(&sensorID))
	_, err = db.Exec(`INSERT INTO sensor_command_history (sensor_id, property, value, mqtt_topic, mqtt_payload)
		VALUES (?, 'state', 'ON', 'zigbee2mqtt/office-plug/set', '{"state":"ON"}')`, sensorID)
	require.NoError(t, err)
	return sensorID
}

func rolesWithPermission(t *testing.T, db *sql.DB, permission string) []string {
	t.Helper()
	rows, err := db.Query(`SELECT r.name FROM roles r
		JOIN role_permissions rp ON rp.role_id = r.id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE p.name = ? ORDER BY r.name`, permission)
	require.NoError(t, err)
	defer rows.Close()
	var roles []string
	for rows.Next() {
		var role string
		require.NoError(t, rows.Scan(&role))
		roles = append(roles, role)
	}
	require.NoError(t, rows.Err())
	return roles
}

func TestMigration25_ExistingCommandHistoryHasNoAutomationRun(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(24))
	insertCommandHistorySensor(t, db)

	require.NoError(t, m.Migrate(25))

	var runID sql.NullInt64
	require.NoError(t, db.QueryRow("SELECT automation_run_id FROM sensor_command_history").Scan(&runID))
	assert.False(t, runID.Valid)
}

func TestMigration25_GrantsAutomationPermissions(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(25))

	assert.Equal(t, []string{"admin", "user", "viewer"}, rolesWithPermission(t, db, "view_automations"))
	assert.Equal(t, []string{"admin", "user"}, rolesWithPermission(t, db, "manage_automations"))

	var email, inApp bool
	require.NoError(t, db.QueryRow(
		"SELECT email_enabled, inapp_enabled FROM notification_channel_defaults WHERE category = 'automation_failure'",
	).Scan(&email, &inApp))
	assert.True(t, email)
	assert.True(t, inApp)
}

func TestMigration25_ScheduleTriggerColumnsAreChecked(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(25))
	_, err := db.Exec("INSERT INTO automations (name) VALUES ('Evening lights')")
	require.NoError(t, err)

	_, err = db.Exec("INSERT INTO automation_triggers (automation_id, position, kind, at_minute_of_day, weekdays) VALUES (1, 1, 'schedule', 1440, 31)")
	assert.Error(t, err, "a time past 23:59 is refused")
	_, err = db.Exec("INSERT INTO automation_triggers (automation_id, position, kind, at_minute_of_day, weekdays) VALUES (1, 1, 'schedule', 1140, 0)")
	assert.Error(t, err, "a schedule with no weekdays is refused")
	_, err = db.Exec("INSERT INTO automation_triggers (automation_id, position, kind, at_minute_of_day, weekdays) VALUES (1, 1, 'schedule', 1140, 62)")
	assert.NoError(t, err)
}

func TestMigration25_DownRemovesAutomations(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(25))
	insertCommandHistorySensor(t, db)

	require.NoError(t, m.Migrate(24))

	var tables int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name LIKE 'automation%'").Scan(&tables))
	assert.Zero(t, tables)

	var columns int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('sensor_command_history') WHERE name = 'automation_run_id'").Scan(&columns))
	assert.Zero(t, columns)

	var commands int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM sensor_command_history").Scan(&commands))
	assert.Equal(t, 1, commands)

	assert.Empty(t, rolesWithPermission(t, db, "view_automations"))
}

func TestMigration26_AWaitStepWaitsAtLeastASecond(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(26))
	_, err := db.Exec("INSERT INTO automations (name) VALUES ('Evening lights')")
	require.NoError(t, err)

	_, err = db.Exec("INSERT INTO automation_steps (automation_id, position, kind, wait_seconds) VALUES (1, 1, 'wait', 0)")
	assert.Error(t, err, "a wait under a second is refused")
	_, err = db.Exec("INSERT INTO automation_steps (automation_id, position, kind) VALUES (1, 1, 'wait')")
	assert.Error(t, err, "a wait with no duration is refused")
	_, err = db.Exec("INSERT INTO automation_steps (automation_id, position, kind, wait_seconds) VALUES (1, 1, 'wait', 14400)")
	assert.NoError(t, err)
}

func TestMigration26_DownRemovesWhatThePreviousSchemaCannotHold(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(26))
	sensorID := insertCommandHistorySensor(t, db)
	_, err := db.Exec(`INSERT INTO automations (id, name) VALUES (1, 'Lamp timer'), (2, 'Lights off')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO automation_steps (automation_id, position, kind, sensor_id, property, value, wait_seconds) VALUES
		(1, 1, 'wait', NULL, NULL, NULL, 60),
		(2, 1, 'set', ?, 'state', 'OFF', NULL)`, sensorID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO automation_runs (automation_id, trigger_kind, status, steps_snapshot, started_at) VALUES
		(2, 'schedule', 'missed', '[]', CURRENT_TIMESTAMP),
		(2, 'schedule', 'waiting', '[]', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)

	require.NoError(t, m.Migrate(25))

	var names []string
	rows, err := db.Query("SELECT name FROM automations")
	require.NoError(t, err)
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		names = append(names, name)
	}
	require.NoError(t, rows.Close())
	assert.Equal(t, []string{"Lights off"}, names)

	var statuses []string
	rows, err = db.Query("SELECT status FROM automation_runs")
	require.NoError(t, err)
	for rows.Next() {
		var status string
		require.NoError(t, rows.Scan(&status))
		statuses = append(statuses, status)
	}
	require.NoError(t, rows.Close())
	assert.Equal(t, []string{"failed"}, statuses)
}

func TestMigration27_AnIntervalIsAtLeastAMinute(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(27))
	_, err := db.Exec("INSERT INTO automations (name) VALUES ('Pond pump')")
	require.NoError(t, err)

	_, err = db.Exec("INSERT INTO automation_triggers (automation_id, position, kind, interval_seconds) VALUES (1, 1, 'interval', 59)")
	assert.Error(t, err, "an interval under a minute is refused")
	_, err = db.Exec("INSERT INTO automation_triggers (automation_id, position, kind) VALUES (1, 1, 'interval')")
	assert.Error(t, err, "an interval with no length is refused")
	_, err = db.Exec("INSERT INTO automation_triggers (automation_id, position, kind, interval_seconds) VALUES (1, 1, 'interval', 1800)")
	assert.NoError(t, err)
}

func TestMigration27_DownRemovesAutomationsWithIntervalTriggers(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(27))
	_, err := db.Exec(`INSERT INTO automations (id, name) VALUES (1, 'Pond pump'), (2, 'Lights off')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO automation_triggers (automation_id, position, kind, at_minute_of_day, weekdays, interval_seconds) VALUES
		(1, 1, 'interval', NULL, NULL, 1800),
		(2, 1, 'schedule', 1410, 127, NULL)`)
	require.NoError(t, err)

	require.NoError(t, m.Migrate(26))

	var names []string
	rows, err := db.Query("SELECT name FROM automations")
	require.NoError(t, err)
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		names = append(names, name)
	}
	require.NoError(t, rows.Close())
	assert.Equal(t, []string{"Lights off"}, names)

	var columns int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('automation_triggers') WHERE name = 'interval_seconds'").Scan(&columns))
	assert.Zero(t, columns)
}

func TestMigration28_ExistingAutomationsRunInSingleMode(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(27))
	_, err := db.Exec("INSERT INTO automations (name) VALUES ('Lights off')")
	require.NoError(t, err)

	require.NoError(t, m.Migrate(28))

	var mode string
	require.NoError(t, db.QueryRow("SELECT mode FROM automations").Scan(&mode))
	assert.Equal(t, "single", mode)
	_, err = db.Exec("UPDATE automations SET mode = 'queued'")
	assert.Error(t, err, "a mode other than single or restart is refused")
}

func TestMigration28_DownRemovesRunsThePreviousSchemaCannotHold(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(28))
	_, err := db.Exec("INSERT INTO automations (id, name, mode) VALUES (1, 'Hall light', 'restart')")
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO automation_runs (automation_id, trigger_kind, status, steps_snapshot, started_at) VALUES
		(1, 'schedule', 'succeeded', '[]', CURRENT_TIMESTAMP),
		(1, 'schedule', 'cancelled', '[]', CURRENT_TIMESTAMP),
		(1, 'schedule', 'skipped', '[]', CURRENT_TIMESTAMP),
		(1, 'manual', 'succeeded', '[]', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)

	require.NoError(t, m.Migrate(27))

	var statuses []string
	rows, err := db.Query("SELECT trigger_kind || ' ' || status FROM automation_runs")
	require.NoError(t, err)
	for rows.Next() {
		var status string
		require.NoError(t, rows.Scan(&status))
		statuses = append(statuses, status)
	}
	require.NoError(t, rows.Close())
	assert.Equal(t, []string{"schedule succeeded"}, statuses)
	var columns int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('automations') WHERE name = 'mode'").Scan(&columns))
	assert.Zero(t, columns)
}

func TestMigration29_ReadingTriggerColumnsAreChecked(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(29))
	sensorID := insertCommandHistorySensor(t, db)
	_, err := db.Exec("INSERT INTO automations (name) VALUES ('Lounge heat on')")
	require.NoError(t, err)
	insert := func(operator string, threshold, value, margin any) error {
		_, err := db.Exec(`INSERT INTO automation_triggers
			(automation_id, position, kind, sensor_id, measurement_type_id, operator, threshold, binary_value, rearm_margin, hold_seconds)
			VALUES (1, 1, 'reading', ?, (SELECT id FROM measurement_types WHERE name = 'temperature'), ?, ?, ?, ?, 0)`,
			sensorID, operator, threshold, value, margin)
		return err
	}

	assert.Error(t, insert("falls_below", 16.0, nil, nil), "a numeric trigger without a margin is refused")
	assert.Error(t, insert("falls_below", 16.0, nil, -0.1), "a negative margin is refused")
	assert.Error(t, insert("becomes", nil, "true", 0.2), "a binary trigger with a margin is refused")
	assert.Error(t, insert("crosses", 16.0, nil, 0.2), "an unknown operator is refused")
	assert.NoError(t, insert("falls_below", 16.0, nil, 0.2))
	assert.NoError(t, insert("becomes", nil, "true", nil))
}

func TestMigration29_DeletingTheSensorDeletesItsReadingTriggers(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(29))
	sensorID := insertCommandHistorySensor(t, db)
	_, err := db.Exec("INSERT INTO automations (name) VALUES ('Lounge heat on')")
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO automation_triggers
		(automation_id, position, kind, sensor_id, measurement_type_id, operator, threshold, rearm_margin, hold_seconds)
		VALUES (1, 1, 'reading', ?, (SELECT id FROM measurement_types WHERE name = 'temperature'), 'falls_below', 16, 0.2, 0)`, sensorID)
	require.NoError(t, err)
	_, err = db.Exec("DELETE FROM sensor_command_history")
	require.NoError(t, err)

	_, err = db.Exec("DELETE FROM sensors WHERE id = ?", sensorID)
	require.NoError(t, err)

	var triggers int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM automation_triggers").Scan(&triggers))
	assert.Zero(t, triggers)
}

func TestMigration29_DownRemovesAutomationsWithReadingTriggers(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(29))
	sensorID := insertCommandHistorySensor(t, db)
	_, err := db.Exec(`INSERT INTO automations (id, name) VALUES (1, 'Lounge heat on'), (2, 'Lights off')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO automation_triggers
		(automation_id, position, kind, at_minute_of_day, weekdays, sensor_id, measurement_type_id, operator, threshold, rearm_margin, hold_seconds) VALUES
		(1, 1, 'reading', NULL, NULL, ?, (SELECT id FROM measurement_types WHERE name = 'temperature'), 'falls_below', 16, 0.2, 0),
		(2, 1, 'schedule', 1410, 127, NULL, NULL, NULL, NULL, NULL, NULL)`, sensorID)
	require.NoError(t, err)

	require.NoError(t, m.Migrate(28))

	var names []string
	rows, err := db.Query("SELECT name FROM automations")
	require.NoError(t, err)
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		names = append(names, name)
	}
	require.NoError(t, rows.Close())
	assert.Equal(t, []string{"Lights off"}, names)

	var columns int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('automation_triggers') WHERE name = 'sensor_id'").Scan(&columns))
	assert.Zero(t, columns)
}

func TestMigration31_DeletingTheCauseRunLeavesTheRunItCausedWithoutACause(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(31))
	_, err := db.Exec("INSERT INTO automations (id, name) VALUES (1, 'Heating on'), (2, 'Fan on')")
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO automation_runs (id, automation_id, trigger_kind, status, steps_snapshot, started_at, cause_run_id) VALUES
		(1, 1, 'schedule', 'succeeded', '[]', CURRENT_TIMESTAMP, NULL),
		(2, 2, 'reading', 'succeeded', '[]', CURRENT_TIMESTAMP, 1)`)
	require.NoError(t, err)

	_, err = db.Exec("DELETE FROM automations WHERE id = 1")
	require.NoError(t, err)

	var cause sql.NullInt64
	require.NoError(t, db.QueryRow("SELECT cause_run_id FROM automation_runs WHERE id = 2").Scan(&cause))
	assert.False(t, cause.Valid)

	require.NoError(t, m.Migrate(30))
	var columns int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('automation_runs') WHERE name = 'cause_run_id'").Scan(&columns))
	assert.Zero(t, columns)
}

func TestMigration34_ExistingCommandsTakeTheAutomationOfTheirRun(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(33))
	sensorID := insertCommandHistorySensor(t, db)
	_, err := db.Exec("INSERT INTO automations (id, name) VALUES (1, 'Evening lights')")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO automation_runs (id, automation_id, trigger_kind, status, steps_snapshot, started_at) VALUES (1, 1, 'schedule', 'succeeded', '[]', CURRENT_TIMESTAMP)")
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO sensor_command_history (sensor_id, automation_run_id, property, value, mqtt_topic, mqtt_payload)
		VALUES (?, 1, 'state', 'OFF', 'zigbee2mqtt/office-plug/set', '{"state":"OFF"}')`, sensorID)
	require.NoError(t, err)

	require.NoError(t, m.Migrate(34))

	rows, err := db.Query("SELECT automation_id FROM sensor_command_history ORDER BY id")
	require.NoError(t, err)
	defer rows.Close()
	var automationIDs []sql.NullInt64
	for rows.Next() {
		var id sql.NullInt64
		require.NoError(t, rows.Scan(&id))
		automationIDs = append(automationIDs, id)
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, []sql.NullInt64{{}, {Int64: 1, Valid: true}}, automationIDs, "a command no run sent has no automation, and the run's command takes the run's automation")

	require.NoError(t, m.Migrate(33))
	var columns int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('sensor_command_history') WHERE name = 'automation_id'").Scan(&columns))
	assert.Zero(t, columns)
}
