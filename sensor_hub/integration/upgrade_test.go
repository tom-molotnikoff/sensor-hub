//go:build integration

package integration

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	database "example/sensorHub/db"
	"example/sensorHub/secrets"

	"github.com/golang-migrate/migrate/v4"
	sqlite_migrate "github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// schemaVersionOf15 is the last migration a 1.5.x release shipped.
const schemaVersionOf15 = 24

// install15 is a 1.5.x install about to be upgraded.
type install15 struct {
	configDir   string
	dbPath      string
	httpAddress string
	brokerID    int
}

// seed15Install lays out a 1.5.x install: its configuration directory and a
// database at the 1.5.x schema holding an outbound broker with a password.
func seed15Install(t *testing.T, brokerPassword string) install15 {
	t.Helper()
	root := t.TempDir()
	configDir := filepath.Join(root, "etc")
	dbPath := filepath.Join(root, "var", "sensor_hub.db")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), 0o755))

	db, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=journal_mode(WAL)")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	driver, err := sqlite_migrate.WithInstance(db, &sqlite_migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	m, err := migrate.NewWithDatabaseInstance("file://../db/migrations", "sqlite", driver)
	require.NoError(t, err)
	require.NoError(t, m.Migrate(schemaVersionOf15))

	result, err := db.Exec(`INSERT INTO mqtt_brokers (name, type, host, port, username, password, enabled)
		VALUES ('Home Mosquitto', 'external', 'mosquitto.home.lan', 1883, 'hub', ?, 0)`, brokerPassword)
	require.NoError(t, err)
	id, err := result.LastInsertId()
	require.NoError(t, err)
	// A 1.5.x hub rewrote the row on every edit, leaving copies in free space.
	_, err = db.Exec("UPDATE mqtt_brokers SET username = 'hub-account' WHERE id = ?", id)
	require.NoError(t, err)
	sourceErr, dbErr := m.Close()
	require.NoError(t, sourceErr)
	require.NoError(t, dbErr)

	listen, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	httpAddress := listen.Addr().String()
	require.NoError(t, listen.Close())

	files := map[string]string{
		"application.properties": fmt.Sprintf("http.listen.address=%s\nmetrics.listen.address=\nmqtt.broker.enabled=false\n", httpAddress),
		"database.properties":    fmt.Sprintf("database.path=%s\n", dbPath),
		"smtp.properties":        "smtp.user=\n",
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(configDir, name), []byte(content), 0o640))
	}
	return install15{configDir: configDir, dbPath: dbPath, httpAddress: httpAddress, brokerID: int(id)}
}

// startHub runs 'local serve' as the packaged unit does and waits until it
// answers. The hub is stopped at the end of the test.
// startHub runs the hub until the test ends. Its output is written by the
// process's copying goroutines while the test reads it, hence the lock.
func startHub(t *testing.T, install install15) *lockedBuffer {
	t.Helper()
	output := &lockedBuffer{}
	cmd := exec.Command(buildSensorHub(t), "local", "serve", "--config-dir", install.configDir)
	cmd.Stdout, cmd.Stderr = output, output
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		_ = cmd.Wait()
	})

	healthURL := fmt.Sprintf("http://%s/api/health", install.httpAddress)
	require.Eventually(t, func() bool {
		resp, err := http.Get(healthURL)
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 60*time.Second, 200*time.Millisecond, "the hub did not come up:\n%s", output)
	return output
}

func fileHolds(t *testing.T, path string, needle string) bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false
	}
	require.NoError(t, err)
	return bytes.Contains(data, []byte(needle))
}

func TestUpgradeFrom15_EncryptsBrokerPasswordsAndScrubsThePlaintext(t *testing.T) {
	const brokerPassword = "plaintext-1.5-broker-password-e81b"
	install := seed15Install(t, brokerPassword)
	dbPath := install.dbPath
	require.True(t, fileHolds(t, dbPath, brokerPassword), "the 1.5.x database holds the password in plaintext")

	output := startHub(t, install)

	assert.Contains(t, output.String(), "generated")
	assert.False(t, fileHolds(t, dbPath, brokerPassword), "the password is gone from the database file")
	assert.False(t, fileHolds(t, dbPath+"-wal", brokerPassword), "the password is gone from the WAL")

	keyText, err := os.ReadFile(filepath.Join(install.configDir, "secrets.key"))
	require.NoError(t, err)
	key, err := secrets.ParseKey(keyText)
	require.NoError(t, err)
	readOnly, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	require.NoError(t, err)
	t.Cleanup(func() { readOnly.Close() })
	store, err := secrets.NewStore(database.NewSecretRepository(&database.Handles{Reader: readOnly, Writer: readOnly}), key, slog.Default())
	require.NoError(t, err)

	value, status, err := store.Get(context.Background(), database.BrokerSecretOwner(install.brokerID), database.BrokerPasswordSecret)
	require.NoError(t, err)
	assert.Equal(t, secrets.StatusSet, status)
	assert.Equal(t, brokerPassword, value, "the secret decrypts to the original password")

	var passwordColumns int
	require.NoError(t, readOnly.QueryRow(
		"SELECT COUNT(*) FROM pragma_table_info('mqtt_brokers') WHERE name = 'password'").Scan(&passwordColumns))
	assert.Zero(t, passwordColumns)
}
