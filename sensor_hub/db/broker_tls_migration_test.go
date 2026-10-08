package database

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func brokerColumnNames(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query("SELECT name FROM pragma_table_info('mqtt_brokers') ORDER BY cid")
	require.NoError(t, err)
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		names = append(names, name)
	}
	require.NoError(t, rows.Err())
	return names
}

func TestMigration37_ReplacesTheCertificatePathsWithATLSSwitchAndACA(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(36))
	_, err := db.Exec(`INSERT INTO mqtt_brokers (name, type, host, port, ca_cert_path)
		VALUES ('Garage', 'external', 'mqtt.garage.lan', 1883, '/etc/ssl/garage-ca.pem')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO mqtt_subscriptions (broker_id, topic_pattern, driver_type)
		SELECT id, 'zigbee2mqtt/#', 'mqtt-zigbee2mqtt' FROM mqtt_brokers WHERE name = 'Garage'`)
	require.NoError(t, err)

	require.NoError(t, m.Migrate(37))

	columns := brokerColumnNames(t, db)
	assert.Subset(t, columns, []string{"tls", "ca_cert_pem"})
	assert.NotContains(t, columns, "ca_cert_path")
	assert.NotContains(t, columns, "client_cert_path")
	assert.NotContains(t, columns, "client_key_path")

	var tls bool
	var ca sql.NullString
	require.NoError(t, db.QueryRow("SELECT tls, ca_cert_pem FROM mqtt_brokers WHERE name = 'Garage'").Scan(&tls, &ca))
	assert.False(t, tls, "an existing broker stays on plain TCP, as it connected before")
	assert.False(t, ca.Valid)

	var subscriptions int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM mqtt_subscriptions WHERE topic_pattern = 'zigbee2mqtt/#'").Scan(&subscriptions))
	assert.Equal(t, 1, subscriptions)

	require.NoError(t, m.Migrate(36))
	columns = brokerColumnNames(t, db)
	assert.Subset(t, columns, []string{"ca_cert_path", "client_cert_path", "client_key_path"})
	assert.NotContains(t, columns, "tls")
	assert.NotContains(t, columns, "ca_cert_pem")
}
