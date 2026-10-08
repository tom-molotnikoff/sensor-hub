//go:build integration

package integration

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

var (
	sensorHubBinaryOnce sync.Once
	sensorHubBinary     string
	sensorHubBinaryErr  error
)

func buildSensorHub(t *testing.T) string {
	t.Helper()
	sensorHubBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "sensor-hub-bin-*")
		if err != nil {
			sensorHubBinaryErr = err
			return
		}
		sensorHubBinary = filepath.Join(dir, "sensor-hub")
		out, err := exec.Command("go", "build", "-o", sensorHubBinary, "example/sensorHub").CombinedOutput()
		if err != nil {
			sensorHubBinaryErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	require.NoError(t, sensorHubBinaryErr)
	return sensorHubBinary
}

func cleanupSensorHubBinary() {
	if sensorHubBinary != "" {
		os.RemoveAll(filepath.Dir(sensorHubBinary))
	}
}

func runSensorHub(t *testing.T, stdin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := exec.Command(buildSensorHub(t), args...)
	cmd.Stdin = strings.NewReader(stdin)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err = cmd.Run()
	return out.String(), errOut.String(), err
}

func freshConfigDir(t *testing.T) (configDir, dbPath string) {
	t.Helper()
	configDir = t.TempDir()
	dbPath = filepath.Join(t.TempDir(), "sensor_hub.db")
	files := map[string]string{
		"application.properties": "auth.bcrypt.cost=4\n",
		"database.properties":    fmt.Sprintf("database.path=%s\n", dbPath),
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(configDir, name), []byte(content), 0o600))
	}
	return configDir, dbPath
}

func openSQLite(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestLocalAdminCreate_CreatesTheFirstAdminAndRefusesASecond(t *testing.T) {
	configDir, dbPath := freshConfigDir(t)

	stdout, stderr, err := runSensorHub(t, "first-admin-pass\n",
		"local", "admin", "create", "first-admin", "--email", "first@example.com", "--config-dir", configDir)
	require.NoError(t, err, stderr)
	assert.Equal(t, "first-admin\n", stdout)

	db := openSQLite(t, dbPath)
	var hash, role string
	var mustChange bool
	require.NoError(t, db.QueryRow(`
		SELECT u.password_hash, u.must_change_password, r.name
		FROM users u JOIN user_roles ur ON ur.user_id = u.id JOIN roles r ON r.id = ur.role_id
		WHERE u.username = 'first-admin'`).Scan(&hash, &mustChange, &role))
	assert.Equal(t, "admin", role)
	assert.False(t, mustChange)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte("first-admin-pass")))
	cost, err := bcrypt.Cost([]byte(hash))
	require.NoError(t, err)
	assert.Equal(t, 10, cost, "the configured cost of 4 is raised to the floor")

	_, stderr, err = runSensorHub(t, "second-admin-pass\n",
		"local", "admin", "create", "second-admin", "--config-dir", configDir)
	require.Error(t, err)
	assert.Contains(t, stderr, "an admin already exists")

	var users int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM users").Scan(&users))
	assert.Equal(t, 1, users)
}

func TestLocalAdminCreate_RejectsAShortPassword(t *testing.T) {
	configDir, dbPath := freshConfigDir(t)

	_, stderr, err := runSensorHub(t, "short\n", "local", "admin", "create", "someone", "--config-dir", configDir)
	require.Error(t, err)
	assert.Contains(t, stderr, "at least 8 characters")
	assert.NoFileExists(t, dbPath)
}

func TestLocalDbBackup_CopiesTheLiveDatabaseAndNeverOverwrites(t *testing.T) {
	backupPath := filepath.Join(t.TempDir(), "backup.db")

	_, stderr, err := runSensorHub(t, "", "local", "db", "backup", backupPath, "--config-dir", env.ConfigDir)
	require.NoError(t, err, stderr)

	info, err := os.Stat(backupPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	var admins int
	require.NoError(t, openSQLite(t, backupPath).
		QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", env.AdminUser).Scan(&admins))
	assert.Equal(t, 1, admins)

	before, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	_, stderr, err = runSensorHub(t, "", "local", "db", "backup", backupPath, "--config-dir", env.ConfigDir)
	require.Error(t, err)
	assert.Contains(t, stderr, "already exists")
	after, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	assert.Equal(t, before, after)
}
