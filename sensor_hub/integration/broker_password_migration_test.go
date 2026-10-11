//go:build integration

package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/secrets"
	"example/sensorHub/testharness"

	"github.com/golang-migrate/migrate/v4"
	sqlite_migrate "github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// schemaVersionOf20 is the last migration a 2.0.x release shipped.
const schemaVersionOf20 = 40

func TestBrokerPasswordMigration_AnEncryptedPasswordFrom20IsStillSetAfterTheUpgrade(t *testing.T) {
	const adminPassword = "broker-migration-admin"
	configDir, dbPath := freshConfigDir(t)
	httpAddress := freeLocalAddress(t)
	properties, err := os.OpenFile(filepath.Join(configDir, "application.properties"), os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = fmt.Fprintf(properties, "http.listen.address=%s\nmetrics.listen.address=\nmqtt.broker.enabled=false\n", httpAddress)
	require.NoError(t, err)
	require.NoError(t, properties.Close())

	// A 2.0.x database: the startup step has dropped the password column and
	// the broker's password is held encrypted under the hub's key.
	db, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=journal_mode(WAL)")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	driver, err := sqlite_migrate.WithInstance(db, &sqlite_migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	m, err := migrate.NewWithDatabaseInstance("file://../db/migrations", "sqlite", driver)
	require.NoError(t, err)
	require.NoError(t, m.Migrate(schemaVersionOf20))
	_, err = db.Exec("ALTER TABLE mqtt_brokers DROP COLUMN password")
	require.NoError(t, err)
	result, err := db.Exec(`INSERT INTO mqtt_brokers (name, type, host, port, username, enabled)
		VALUES ('Home Mosquitto', 'external', 'mosquitto.home.lan', 1883, 'hub', 0)`)
	require.NoError(t, err)
	brokerID, err := result.LastInsertId()
	require.NoError(t, err)
	key, err := secrets.GenerateKey()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "secrets.key"), []byte(key.Encode()+"\n"), 0o600))
	store, err := secrets.NewStore(database.NewSecretRepository(&database.Handles{Reader: db, Writer: db}), key, slog.Default())
	require.NoError(t, err)
	require.NoError(t, store.Set(context.Background(), database.BrokerSecretOwner(int(brokerID)), database.BrokerPasswordSecret, "broker-password-from-2.0"))
	sourceErr, dbErr := m.Close()
	require.NoError(t, sourceErr)
	require.NoError(t, dbErr)

	startHub(t, configDir, httpAddress)

	latest, err := database.LatestMigrationVersion()
	require.NoError(t, err)
	var version uint
	require.NoError(t, openSQLite(t, dbPath).QueryRow("SELECT version FROM schema_migrations").Scan(&version))
	require.Equal(t, latest, version, "the hub ran the migrations")
	_, stderr, err := runSensorHub(t, adminPassword+"\n", "local", "admin", "create", "admin", "--config-dir", configDir)
	require.NoError(t, err, stderr)
	admin := testharness.NewClient(t, "http://"+httpAddress)
	require.Equal(t, http.StatusOK, admin.Login("admin", adminPassword))
	resp, status := admin.GetMQTTBroker(int(brokerID))
	require.Equal(t, http.StatusOK, status, "body: %s", resp)
	var broker gen.MQTTBroker
	require.NoError(t, json.Unmarshal(resp, &broker))
	require.NotNil(t, broker.PasswordStatus)
	assert.Equal(t, gen.MQTTBrokerPasswordStatusSet, *broker.PasswordStatus)
}
