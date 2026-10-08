package database

import (
	"bytes"
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	appProps "example/sensorHub/application_properties"

	"github.com/golang-migrate/migrate/v4"
	sqlite_migrate "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// schemaVersionOf15 is the last migration a 1.5.x release shipped.
const schemaVersionOf15 = 24

// seed15Database writes a database at the 1.5.x schema, as a 1.5.x hub left
// it, and runs fn against it before closing it.
func seed15Database(t *testing.T, fn func(db *sql.DB)) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sensor_hub.db")
	db, err := sql.Open("sqlite", "file:"+path+"?"+writerDSNParams)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)

	source, err := iofs.New(migrationsFS, "migrations")
	require.NoError(t, err)
	driver, err := sqlite_migrate.WithInstance(db, &sqlite_migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	m, err := migrate.NewWithInstance("iofs", source, "sqlite", driver)
	require.NoError(t, err)
	require.NoError(t, m.Migrate(schemaVersionOf15))

	fn(db)

	sourceErr, dbErr := m.Close()
	require.NoError(t, sourceErr)
	require.NoError(t, dbErr)
	return path
}

func openUpgraded(t *testing.T, path string) *Handles {
	t.Helper()
	h, err := Open(&appProps.ApplicationConfiguration{DatabasePath: path, DatabaseReaderConnections: 2}, slog.Default())
	require.NoError(t, err)
	t.Cleanup(func() { _ = h.Close() })
	return h
}

// reverseSeal stands in for AES-GCM: the step under test only needs a seal
// whose output differs from its input.
func reverseSeal(owner, name string, value []byte) (SealedSecret, error) {
	out := make([]byte, len(value))
	for i, b := range value {
		out[len(value)-1-i] = b
	}
	return SealedSecret{Owner: owner, Name: name, KeyID: 1, Nonce: []byte("nonce"), Ciphertext: out}, nil
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

func passwordColumnPresent(t *testing.T, h *Handles) bool {
	t.Helper()
	var n int
	require.NoError(t, h.Reader.QueryRow(
		"SELECT COUNT(*) FROM pragma_table_info('mqtt_brokers') WHERE name = 'password'").Scan(&n))
	return n > 0
}

func TestMoveBrokerPasswordsToSecrets_ScrubsEveryPlaintextCopyFromTheFileAndWAL(t *testing.T) {
	const earlier = "earlier-broker-password-7f3a"
	const current = "current-broker-password-9c1e"
	path := seed15Database(t, func(db *sql.DB) {
		_, err := db.Exec(`INSERT INTO mqtt_brokers (name, type, host, port, username, password)
			VALUES ('Garage', 'external', 'mqtt.garage.lan', 1883, 'hub', ?)`, earlier)
		require.NoError(t, err)
		// Each rewrite of the row leaves the old cell, password and all, in the
		// page's free space, as edits on a 1.5.x hub did.
		_, err = db.Exec("UPDATE mqtt_brokers SET password = ?, username = 'hub-renamed' WHERE name = 'Garage'", current)
		require.NoError(t, err)
		_, err = db.Exec("UPDATE mqtt_brokers SET username = 'hub' WHERE name = 'Garage'")
		require.NoError(t, err)
		_, err = db.Exec(`INSERT INTO mqtt_brokers (name, type, host, port, password)
			VALUES ('No password', 'external', 'other.lan', 1883, NULL), ('Empty password', 'external', 'third.lan', 1883, '')`)
		require.NoError(t, err)
		// Enough brokers that the table spans several pages, so dropping it
		// frees leaf pages rather than only relocating its root page.
		_, err = db.Exec(`WITH RECURSIVE seq(n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 200)
			INSERT INTO mqtt_brokers (name, type, host, port, password)
			SELECT 'Filler ' || n, 'external', 'filler-' || n || '.lan', 1883, ? || '-' || n FROM seq`, current)
		require.NoError(t, err)
	})
	require.True(t, fileContains(t, path, []byte(current)), "the seed holds the password in plaintext")

	h := openUpgraded(t, path)
	moved, err := MoveBrokerPasswordsToSecrets(context.Background(), h, reverseSeal, slog.Default())
	require.NoError(t, err)

	assert.Equal(t, 201, moved)
	assert.False(t, passwordColumnPresent(t, h))
	for _, password := range []string{earlier, current} {
		assert.False(t, fileContains(t, path, []byte(password)), "%s is gone from the database file", password)
		assert.False(t, fileContains(t, path+"-wal", []byte(password)), "%s is gone from the WAL", password)
	}

	var garageID int
	require.NoError(t, h.Reader.QueryRow("SELECT id FROM mqtt_brokers WHERE name = 'Garage'").Scan(&garageID))
	stored, err := NewSecretRepository(h).All(context.Background())
	require.NoError(t, err)
	assert.Len(t, stored, 201, "a NULL or empty password stores nothing")
	garage, err := NewSecretRepository(h).Get(context.Background(), BrokerSecretOwner(garageID), BrokerPasswordSecret)
	require.NoError(t, err)
	want, _ := reverseSeal(BrokerSecretOwner(garageID), BrokerPasswordSecret, []byte(current))
	assert.Equal(t, &want, garage)
}

func TestMoveBrokerPasswordsToSecrets_DoesNothingOnceTheColumnIsGone(t *testing.T) {
	path := seed15Database(t, func(db *sql.DB) {
		_, err := db.Exec(`INSERT INTO mqtt_brokers (name, type, host, port, password)
			VALUES ('Garage', 'external', 'mqtt.garage.lan', 1883, 'pw')`)
		require.NoError(t, err)
	})
	h := openUpgraded(t, path)
	_, err := MoveBrokerPasswordsToSecrets(context.Background(), h, reverseSeal, slog.Default())
	require.NoError(t, err)

	moved, err := MoveBrokerPasswordsToSecrets(context.Background(), h, func(string, string, []byte) (SealedSecret, error) {
		t.Fatal("nothing is sealed on a second run")
		return SealedSecret{}, nil
	}, slog.Default())

	require.NoError(t, err)
	assert.Equal(t, 0, moved)
}

func TestMoveBrokerPasswordsToSecrets_FinishesARunThatStoppedAfterCommitting(t *testing.T) {
	path := seed15Database(t, func(db *sql.DB) {
		_, err := db.Exec(`INSERT INTO mqtt_brokers (name, type, host, port, password)
			VALUES ('Garage', 'external', 'mqtt.garage.lan', 1883, NULL)`)
		require.NoError(t, err)
	})
	h := openUpgraded(t, path)
	require.True(t, passwordColumnPresent(t, h), "the hub stopped before dropping the column")

	moved, err := MoveBrokerPasswordsToSecrets(context.Background(), h, reverseSeal, slog.Default())

	require.NoError(t, err)
	assert.Equal(t, 0, moved)
	assert.False(t, passwordColumnPresent(t, h))
	var secureDelete int
	require.NoError(t, h.Writer.QueryRow("PRAGMA secure_delete").Scan(&secureDelete))
	assert.Equal(t, 0, secureDelete, "the writer goes back to its usual setting")
}

func TestMigration36_DownGivesBackAnEmptyPasswordColumn(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Up())
	_, err := db.Exec("ALTER TABLE mqtt_brokers DROP COLUMN password")
	require.NoError(t, err, "as the startup step leaves it")

	require.NoError(t, m.Migrate(35))

	var n int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('mqtt_brokers') WHERE name = 'password'").Scan(&n))
	assert.Equal(t, 1, n)
	require.NoError(t, m.Migrate(34), "000035's down finds the column it copies")
}
