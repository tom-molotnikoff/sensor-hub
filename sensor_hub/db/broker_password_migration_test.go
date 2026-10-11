package database

import (
	"bytes"
	"database/sql"
	"log/slog"
	"os"
	"testing"

	appProps "example/sensorHub/application_properties"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func queryRows(t *testing.T, db *sql.DB, query string) [][]any {
	t.Helper()
	rows, err := db.Query(query)
	require.NoError(t, err)
	defer rows.Close()
	columns, err := rows.Columns()
	require.NoError(t, err)
	var all [][]any
	for rows.Next() {
		row := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range row {
			pointers[i] = &row[i]
		}
		require.NoError(t, rows.Scan(pointers...))
		all = append(all, row)
	}
	require.NoError(t, rows.Err())
	return all
}

const (
	brokerRowsQuery = `SELECT id, name, type, host, port, username, client_id, enabled,
		created_at, updated_at, tls, ca_cert_pem FROM mqtt_brokers ORDER BY id`
	subscriptionRowsQuery = "SELECT * FROM mqtt_subscriptions ORDER BY id"
)

// Garage sets every column it can away from its default, so a value the
// rebuild lost would not come back looking unchanged.
func seedBrokers(t *testing.T, db *sql.DB, withPasswords bool) {
	t.Helper()
	columns, values := "", ""
	if withPasswords {
		columns, values = ", password", ", 'garage-password'"
	}
	_, err := db.Exec(`INSERT INTO mqtt_brokers (name, type, host, port, username, client_id, enabled,
		created_at, updated_at, tls, ca_cert_pem` + columns + `)
		VALUES ('Garage', 'external', 'mqtt.garage.lan', 8883, 'hub', 'hub-garage', 0,
			'2025-01-02 03:04:05', '2025-06-07 08:09:10', 1, '-----BEGIN CERTIFICATE-----'` + values + `)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO mqtt_brokers (name, type, host, port, username)
		VALUES ('Loft', 'external', 'mqtt.loft.lan', 1884, 'loft')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO mqtt_subscriptions (broker_id, topic_pattern, driver_type)
		SELECT id, 'zigbee2mqtt/#', 'mqtt-zigbee2mqtt' FROM mqtt_brokers WHERE name IN ('Garage', 'Loft')
		UNION ALL
		SELECT id, 'shellies/#', 'mqtt-shelly' FROM mqtt_brokers WHERE name = 'Garage'`)
	require.NoError(t, err)
}

func assertMigration41KeepsBrokersAndSubscriptions(t *testing.T, passwordColumnPresent bool) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(40))
	if !passwordColumnPresent {
		_, err := db.Exec("ALTER TABLE mqtt_brokers DROP COLUMN password")
		require.NoError(t, err, "as a 2.0.x hub left it")
	}
	seedBrokers(t, db, passwordColumnPresent)
	brokers := queryRows(t, db, brokerRowsQuery)
	subscriptions := queryRows(t, db, subscriptionRowsQuery)
	require.Len(t, subscriptions, 3)

	require.NoError(t, m.Migrate(41))

	assert.NotContains(t, brokerColumnNames(t, db), "password")
	assert.Equal(t, brokers, queryRows(t, db, brokerRowsQuery))
	assert.Equal(t, subscriptions, queryRows(t, db, subscriptionRowsQuery))
	assert.Empty(t, queryRows(t, db, "PRAGMA foreign_key_check"))
}

func TestMigration41_DropsThePasswordColumnAndKeepsBrokersAndSubscriptions(t *testing.T) {
	assertMigration41KeepsBrokersAndSubscriptions(t, true)
}

func TestMigration41_KeepsBrokersAndSubscriptionsWhereA20HubDroppedTheColumn(t *testing.T) {
	assertMigration41KeepsBrokersAndSubscriptions(t, false)
}

func TestMigration41_KeepsEveryOtherColumnConstraintAndDefault(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(40))
	_, err := db.Exec("INSERT INTO mqtt_brokers (name, type, host, port) VALUES ('Deleted', 'external', 'gone.lan', 1883)")
	require.NoError(t, err)
	var deletedID int64
	require.NoError(t, db.QueryRow("SELECT id FROM mqtt_brokers WHERE name = 'Deleted'").Scan(&deletedID))
	_, err = db.Exec("DELETE FROM mqtt_brokers WHERE name = 'Deleted'")
	require.NoError(t, err)
	const columnsQuery = `SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info('mqtt_brokers')
		WHERE name != 'password' ORDER BY cid`
	const indexesQuery = `SELECT "unique", origin, partial FROM pragma_index_list('mqtt_brokers')`
	columns := queryRows(t, db, columnsQuery)
	indexes := queryRows(t, db, indexesQuery)

	require.NoError(t, m.Migrate(41))

	assert.Equal(t, columns, queryRows(t, db, columnsQuery))
	assert.Equal(t, indexes, queryRows(t, db, indexesQuery))

	result, err := db.Exec("INSERT INTO mqtt_brokers (name, host, port) VALUES ('Shed', 'mqtt.shed.lan', 1883)")
	require.NoError(t, err)
	id, err := result.LastInsertId()
	require.NoError(t, err)
	assert.Greater(t, id, deletedID, "a deleted broker's id is not handed out again")
	var brokerType string
	var enabled, tls bool
	var createdAt sql.NullString
	require.NoError(t, db.QueryRow("SELECT type, enabled, tls, created_at FROM mqtt_brokers WHERE id = ?", id).
		Scan(&brokerType, &enabled, &tls, &createdAt))
	assert.Equal(t, "external", brokerType)
	assert.True(t, enabled)
	assert.False(t, tls)
	assert.True(t, createdAt.Valid)

	for _, insert := range []string{
		"INSERT INTO mqtt_brokers (name, type) VALUES ('No address', 'external')",
		"INSERT INTO mqtt_brokers (name, type, host, port) VALUES ('Odd', 'bridged', 'odd.lan', 1883)",
		"INSERT INTO mqtt_brokers (name, host, port) VALUES ('Shed', 'other.lan', 1883)",
		"INSERT INTO mqtt_brokers (host, port) VALUES ('nameless.lan', 1883)",
	} {
		_, err := db.Exec(insert)
		assert.Error(t, err, insert)
	}
	_, err = db.Exec("INSERT INTO mqtt_brokers (name, type) VALUES ('In process', 'embedded')")
	assert.NoError(t, err, "an embedded broker still needs no address")
}

func TestMigration41_DeletingABrokerStillDeletesItsSubscriptions(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(40))
	seedBrokers(t, db, true)
	require.NoError(t, m.Migrate(41))

	_, err := db.Exec("DELETE FROM mqtt_brokers WHERE name = 'Garage'")
	require.NoError(t, err)

	var garage, loft int
	require.NoError(t, db.QueryRow(`SELECT
		COUNT(*) FILTER (WHERE broker_id NOT IN (SELECT id FROM mqtt_brokers)),
		COUNT(*) FILTER (WHERE broker_id IN (SELECT id FROM mqtt_brokers WHERE name = 'Loft'))
		FROM mqtt_subscriptions`).Scan(&garage, &loft))
	assert.Zero(t, garage)
	assert.Equal(t, 1, loft)
}

func fileContains(t *testing.T, path string, needle []byte) bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false
	}
	require.NoError(t, err)
	return bytes.Contains(data, needle)
}

func TestMigration41_LeavesNoPlaintextPasswordInTheFileOrWAL(t *testing.T) {
	const earlier = "earlier-broker-password-7f3a"
	const current = "current-broker-password-9c1e"
	db, path := newTempFileDBAt(t)
	require.NoError(t, newTestMigrator(t, db).Migrate(40))
	_, err := db.Exec(`INSERT INTO mqtt_brokers (name, type, host, port, username, password)
		VALUES ('Garage', 'external', 'mqtt.garage.lan', 1883, 'hub', ?)`, earlier)
	require.NoError(t, err)
	// Each rewrite of the row leaves the old cell, password and all, in the
	// page's free space.
	_, err = db.Exec("UPDATE mqtt_brokers SET password = ?, username = 'hub-renamed' WHERE name = 'Garage'", current)
	require.NoError(t, err)
	_, err = db.Exec("UPDATE mqtt_brokers SET username = 'hub' WHERE name = 'Garage'")
	require.NoError(t, err)
	// Enough brokers that the table spans several pages, so dropping it frees
	// leaf pages rather than only relocating its root page.
	_, err = db.Exec(`WITH RECURSIVE seq(n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 200)
		INSERT INTO mqtt_brokers (name, type, host, port, password)
		SELECT 'Filler ' || n, 'external', 'filler-' || n || '.lan', 1883, ? || '-' || n FROM seq`, current)
	require.NoError(t, err)
	require.NoError(t, db.Close())
	require.True(t, fileContains(t, path, []byte(current)), "the seed holds the password in plaintext")

	h, err := Open(&appProps.ApplicationConfiguration{DatabasePath: path, DatabaseReaderConnections: 2}, slog.Default())
	require.NoError(t, err)
	t.Cleanup(func() { _ = h.Close() })

	for _, password := range []string{earlier, current} {
		assert.False(t, fileContains(t, path, []byte(password)), "%s is gone from the database file", password)
		assert.False(t, fileContains(t, path+"-wal", []byte(password)), "%s is gone from the WAL", password)
	}
}

func TestMigration41_DownGivesBackAnEmptyPasswordColumn(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(40))
	seedBrokers(t, db, true)
	require.NoError(t, m.Migrate(41))

	require.NoError(t, m.Migrate(40))

	var nullable, stored int
	require.NoError(t, db.QueryRow(`SELECT NOT "notnull" FROM pragma_table_info('mqtt_brokers') WHERE name = 'password'`).Scan(&nullable))
	assert.Equal(t, 1, nullable)
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM mqtt_brokers WHERE password IS NOT NULL").Scan(&stored))
	assert.Zero(t, stored)
	require.NoError(t, m.Migrate(34), "the older down migrations find the column they copy")
}

func TestMigration36_DownGivesBackAnEmptyPasswordColumn(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(40))
	_, err := db.Exec("ALTER TABLE mqtt_brokers DROP COLUMN password")
	require.NoError(t, err, "as a 2.0.x hub left it")

	require.NoError(t, m.Migrate(35))

	var n int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('mqtt_brokers') WHERE name = 'password'").Scan(&n))
	assert.Equal(t, 1, n)
	require.NoError(t, m.Migrate(34), "000035's down finds the column it copies")
}
