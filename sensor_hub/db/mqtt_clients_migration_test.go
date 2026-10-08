package database

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigration35_NullsTheEmbeddedBrokerAddressAndKeepsSubscriptions(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(34))

	_, err := db.Exec("INSERT INTO mqtt_brokers (name, type, host, port) VALUES ('Garage', 'external', 'mqtt.garage.lan', 1884)")
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO mqtt_subscriptions (broker_id, topic_pattern, driver_type)
		SELECT id, 'zigbee2mqtt/#', 'mqtt-zigbee2mqtt' FROM mqtt_brokers`)
	require.NoError(t, err)

	require.NoError(t, m.Migrate(35))

	var host sql.NullString
	var port sql.NullInt64
	require.NoError(t, db.QueryRow("SELECT host, port FROM mqtt_brokers WHERE type = 'embedded'").Scan(&host, &port))
	assert.False(t, host.Valid, "the embedded broker has no host")
	assert.False(t, port.Valid, "the embedded broker has no port")

	require.NoError(t, db.QueryRow("SELECT host, port FROM mqtt_brokers WHERE name = 'Garage'").Scan(&host, &port))
	assert.Equal(t, "mqtt.garage.lan", host.String)
	assert.Equal(t, int64(1884), port.Int64)

	var subscriptions int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM mqtt_subscriptions").Scan(&subscriptions))
	assert.Equal(t, 2, subscriptions, "rebuilding the brokers table does not cascade to its subscriptions")

	_, err = db.Exec("INSERT INTO mqtt_brokers (name, type) VALUES ('No address', 'external')")
	assert.Error(t, err, "an external broker still needs a host and port")
}
