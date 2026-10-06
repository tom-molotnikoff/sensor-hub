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

func TestMigration25_DeletingARunKeepsItsCommands(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(25))
	insertCommandHistorySensor(t, db)

	_, err := db.Exec("INSERT INTO automations (name) VALUES ('Evening lights')")
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO automation_runs (automation_id, trigger_kind, status, steps_snapshot, started_at)
		VALUES (1, 'schedule', 'succeeded', '[]', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	_, err = db.Exec("UPDATE sensor_command_history SET automation_run_id = 1")
	require.NoError(t, err)

	_, err = db.Exec("DELETE FROM automations WHERE id = 1")
	require.NoError(t, err)

	var commands int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM sensor_command_history WHERE automation_run_id IS NULL").Scan(&commands))
	assert.Equal(t, 1, commands)
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
