//go:build integration

package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/secrets"
	"example/sensorHub/testharness"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func freeLocalAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	return listener.Addr().String()
}

// decryptFailuresGauge reads sensor_hub_secret_decrypt_failures from the
// hub's metrics listener.
func decryptFailuresGauge(t *testing.T, metricsAddress string) int {
	t.Helper()
	resp, err := http.Get("http://" + metricsAddress + "/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "sensor_hub_secret_decrypt_failures") {
			continue
		}
		fields := strings.Fields(line)
		value, err := strconv.Atoi(fields[len(fields)-1])
		require.NoError(t, err, line)
		return value
	}
	t.Fatalf("no sensor_hub_secret_decrypt_failures in the metrics:\n%s", body)
	return 0
}

// servedConfigDir is a fresh configuration directory for a hub run with
// startHub, with an admin to log in as.
func servedConfigDir(t *testing.T, adminPassword string) (configDir, dbPath, httpAddress, metricsAddress string) {
	t.Helper()
	configDir, dbPath = freshConfigDir(t)
	httpAddress, metricsAddress = freeLocalAddress(t), freeLocalAddress(t)
	properties, err := os.OpenFile(filepath.Join(configDir, "application.properties"), os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = fmt.Fprintf(properties, "http.listen.address=%s\nmetrics.listen.address=%s\nmqtt.broker.enabled=false\n", httpAddress, metricsAddress)
	require.NoError(t, err)
	require.NoError(t, properties.Close())
	_, stderr, err := runSensorHub(t, adminPassword+"\n", "local", "admin", "create", "admin", "--config-dir", configDir)
	require.NoError(t, err, stderr)
	return configDir, dbPath, httpAddress, metricsAddress
}

// replaceSecretsKey replaces the hub's key, as on a new host or after a lost
// key.
func replaceSecretsKey(t *testing.T, configDir string) secrets.Key {
	t.Helper()
	newKey, err := secrets.GenerateKey()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "secrets.key"), []byte(newKey.Encode()+"\n"), 0o600))
	return newKey
}

// storedSecret reads a secret from the database as the hub would under key.
func storedSecret(t *testing.T, dbPath string, key secrets.Key, owner, name string) (string, secrets.Status) {
	t.Helper()
	readOnly := openSQLite(t, dbPath)
	store, err := secrets.NewStore(database.NewSecretRepository(&database.Handles{Reader: readOnly, Writer: readOnly}), key, slog.Default())
	require.NoError(t, err)
	value, status, err := store.Get(context.Background(), owner, name)
	require.NoError(t, err)
	return value, status
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

func smtpSettingsWithPassword(password string) gen.EmailSettings {
	return gen.EmailSettings{
		Host: "smtp.example.com", Port: 587, Security: gen.EmailSettingsSecurityStarttls,
		Username: "alerts@example.com", FromAddress: "alerts@example.com", Password: &password,
	}
}

func emailPasswordStatus(t *testing.T, client *testharness.Client) gen.EmailSettingsPasswordStatus {
	t.Helper()
	resp, status := client.GetEmailSettings()
	require.Equal(t, http.StatusOK, status, "body: %s", resp)
	var settings gen.EmailSettings
	require.NoError(t, json.Unmarshal(resp, &settings))
	require.NotNil(t, settings.PasswordStatus)
	return *settings.PasswordStatus
}

func secretFailureNotifications(t *testing.T, client *testharness.Client) []gen.Notification {
	t.Helper()
	resp, status := client.GetNotifications(100, 0)
	require.Equal(t, http.StatusOK, status, "body: %s", resp)
	var listed []gen.UserNotification
	require.NoError(t, json.Unmarshal(resp, &listed))
	var found []gen.Notification
	for _, n := range listed {
		if n.Notification != nil && n.Notification.Category == gen.NotificationCategorySecretFailure {
			found = append(found, *n.Notification)
		}
	}
	return found
}

func TestSecretFailure_AHubStartedWithAnotherKeyRunsWithoutTheSecretUntilItIsEnteredAgain(t *testing.T) {
	const adminPassword = "secret-failure-admin"
	const brokerPassword = "broker-password-under-the-first-key"
	external := startLoginBroker(t, brokerPassword)
	configDir, dbPath, httpAddress, metricsAddress := servedConfigDir(t, adminPassword)
	baseURL := "http://" + httpAddress

	// The broker's password is stored under the key the hub generates.
	hub := startHub(t, configDir, httpAddress)
	admin := testharness.NewClient(t, baseURL)
	require.Equal(t, http.StatusOK, admin.Login("admin", adminPassword))
	resp, status := admin.CreateMQTTBroker(gen.MQTTBroker{
		Name: "Home Mosquitto", Type: "external", Host: ptrStr("127.0.0.1"), Port: ptrInt(external.Port),
		Username: ptrStr(reconnectUsername), Password: ptrStr(brokerPassword), Enabled: true,
	})
	require.Equal(t, http.StatusCreated, status, "body: %s", resp)
	var created struct {
		ID int `json:"id"`
	}
	require.NoError(t, json.Unmarshal(resp, &created))
	clientID := fmt.Sprintf("sensor-hub-%d", created.ID)
	require.Eventually(t, func() bool { return external.Connected(clientID) }, reconnectWithin, 100*time.Millisecond)
	assert.Empty(t, secretFailureNotifications(t, admin), "every secret decrypted at the first start")
	hub.stop()

	newKey := replaceSecretsKey(t, configDir)
	hub = startHub(t, configDir, httpAddress)

	admin = testharness.NewClient(t, baseURL)
	require.Equal(t, http.StatusOK, admin.Login("admin", adminPassword), "logins work")
	_, status = admin.GetAllSensors()
	assert.Equal(t, http.StatusOK, status, "the API works")

	brokerStatus := func() gen.MQTTBrokerPasswordStatus {
		resp, status := admin.GetMQTTBroker(created.ID)
		require.Equal(t, http.StatusOK, status, "body: %s", resp)
		var broker gen.MQTTBroker
		require.NoError(t, json.Unmarshal(resp, &broker))
		require.NotNil(t, broker.PasswordStatus)
		return *broker.PasswordStatus
	}
	assert.Equal(t, gen.MQTTBrokerPasswordStatusNeedsReentry, brokerStatus())
	assert.Never(t, func() bool { return external.Connected(clientID) }, 2*time.Second, 100*time.Millisecond,
		"a broker whose password does not decrypt stays disconnected")

	notices := secretFailureNotifications(t, admin)
	require.Len(t, notices, 1)
	assert.Equal(t, gen.Error, notices[0].Severity)
	assert.Equal(t, "Stored secrets need re-entry", notices[0].Title)
	assert.Contains(t, notices[0].Message, `MQTT broker "Home Mosquitto"`)
	assert.Equal(t, 1, decryptFailuresGauge(t, metricsAddress))

	// Entering the password again stores it under the new key and reconnects.
	resp, status = admin.UpdateMQTTBroker(created.ID, gen.MQTTBroker{
		Name: "Home Mosquitto", Type: "external", Host: ptrStr("127.0.0.1"), Port: ptrInt(external.Port),
		Username: ptrStr(reconnectUsername), Password: ptrStr(brokerPassword), Enabled: true,
	})
	require.Equal(t, http.StatusOK, status, "body: %s", resp)
	assert.Equal(t, gen.MQTTBrokerPasswordStatusSet, brokerStatus())
	assert.Eventually(t, func() bool { return external.Connected(clientID) }, reconnectWithin, 100*time.Millisecond,
		"the broker reconnects once its password is entered again")
	assert.Equal(t, 0, decryptFailuresGauge(t, metricsAddress))
	assert.Len(t, secretFailureNotifications(t, admin), 1, "re-entry raises no more notifications")

	value, secretStatus := storedSecret(t, dbPath, newKey, database.BrokerSecretOwner(created.ID), database.BrokerPasswordSecret)
	assert.Equal(t, secrets.StatusSet, secretStatus)
	assert.Equal(t, brokerPassword, value, "the row is overwritten under the current key")
	hub.stop()
}

func TestSecretFailure_AnSMTPPasswordUnderAnotherKeyNeedsReentryUntilItIsEnteredAgain(t *testing.T) {
	const adminPassword = "smtp-reentry-admin"
	const smtpPassword = "smtp-password-under-the-first-key"
	configDir, dbPath, httpAddress, _ := servedConfigDir(t, adminPassword)
	baseURL := "http://" + httpAddress

	hub := startHub(t, configDir, httpAddress)
	admin := testharness.NewClient(t, baseURL)
	require.Equal(t, http.StatusOK, admin.Login("admin", adminPassword))
	resp, status := admin.UpdateEmailSettings(smtpSettingsWithPassword(smtpPassword))
	require.Equal(t, http.StatusOK, status, "body: %s", resp)
	require.Equal(t, gen.EmailSettingsPasswordStatusSet, emailPasswordStatus(t, admin))
	hub.stop()

	newKey := replaceSecretsKey(t, configDir)
	hub = startHub(t, configDir, httpAddress)
	admin = testharness.NewClient(t, baseURL)
	require.Equal(t, http.StatusOK, admin.Login("admin", adminPassword))
	assert.Equal(t, gen.EmailSettingsPasswordStatusNeedsReentry, emailPasswordStatus(t, admin))

	resp, status = admin.UpdateEmailSettings(smtpSettingsWithPassword(smtpPassword))
	require.Equal(t, http.StatusOK, status, "body: %s", resp)
	assert.Equal(t, gen.EmailSettingsPasswordStatusSet, emailPasswordStatus(t, admin))
	hub.stop()

	value, secretStatus := storedSecret(t, dbPath, newKey, database.SMTPSecretOwner, database.SMTPPasswordSecret)
	assert.Equal(t, secrets.StatusSet, secretStatus)
	assert.Equal(t, smtpPassword, value, "the row is overwritten under the current key")
}

// The password is checked for in the WAL while the hub runs, before a
// checkpoint at shutdown can fold the WAL into the database file.
func TestSMTPPassword_NeverReachesTheDatabaseTheWALOrTheLogs(t *testing.T) {
	const adminPassword = "smtp-secrecy-admin"
	const smtpPassword = "plaintext-smtp-password-5c1d"
	configDir, dbPath, httpAddress, _ := servedConfigDir(t, adminPassword)

	hub := startHub(t, configDir, httpAddress)
	admin := testharness.NewClient(t, "http://"+httpAddress)
	require.Equal(t, http.StatusOK, admin.Login("admin", adminPassword))
	resp, status := admin.UpdateEmailSettings(smtpSettingsWithPassword(smtpPassword))
	require.Equal(t, http.StatusOK, status, "body: %s", resp)
	assert.NotContains(t, string(resp), smtpPassword)
	assert.False(t, fileHolds(t, dbPath, smtpPassword), "the SMTP password is not in the database file")
	assert.False(t, fileHolds(t, dbPath+"-wal", smtpPassword), "the SMTP password is not in the WAL")
	hub.stop()

	assert.NotContains(t, hub.output.String(), smtpPassword, "the hub never logs the SMTP password")
	assert.False(t, fileHolds(t, dbPath, smtpPassword), "the SMTP password is not in the database file")
	keyText, err := os.ReadFile(filepath.Join(configDir, "secrets.key"))
	require.NoError(t, err)
	key, err := secrets.ParseKey(keyText)
	require.NoError(t, err)
	value, secretStatus := storedSecret(t, dbPath, key, database.SMTPSecretOwner, database.SMTPPasswordSecret)
	assert.Equal(t, secrets.StatusSet, secretStatus)
	assert.Equal(t, smtpPassword, value, "the password is stored, encrypted")
}
