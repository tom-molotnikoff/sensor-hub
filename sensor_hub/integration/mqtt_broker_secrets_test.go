//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

// createBrokerWithPassword creates a disabled external broker through the API
// and returns its id. Disabled, so nothing tries to reach the made-up host.
func createBrokerWithPassword(t *testing.T, name string, password *string) int {
	t.Helper()
	resp, status := client.CreateMQTTBroker(gen.MQTTBroker{
		Name: name, Type: "external", Host: ptrStr(name + ".example.com"), Port: ptrInt(1883),
		Username: ptrStr("hub"), Password: password,
	})
	require.Equal(t, http.StatusCreated, status, "body: %s", resp)
	var created struct {
		ID int `json:"id"`
	}
	require.NoError(t, json.Unmarshal(resp, &created))
	t.Cleanup(func() { client.DeleteMQTTBroker(created.ID) })
	return created.ID
}

// storedBrokerPassword reads the broker's password the way the connection
// manager does.
func storedBrokerPassword(t *testing.T, id int) (string, secrets.Status) {
	t.Helper()
	value, status, err := env.Secrets.Get(context.Background(), database.BrokerSecretOwner(id), database.BrokerPasswordSecret)
	require.NoError(t, err)
	return value, status
}

func passwordStatusOf(t *testing.T, id int) string {
	t.Helper()
	resp, status := client.GetMQTTBroker(id)
	require.Equal(t, http.StatusOK, status)
	var broker map[string]any
	require.NoError(t, json.Unmarshal(resp, &broker))
	return fmt.Sprint(broker["password_status"])
}

func clientWithRole(t *testing.T, role string) *testharness.Client {
	t.Helper()
	username := fmt.Sprintf("broker-secrets-%s-%d", role, time.Now().UnixNano())
	_, status := client.CreateUser(gen.CreateUserRequest{
		Username: username, Password: "rolepass123", Email: ptrStr(username + "@test.com"), Roles: &[]string{role},
	})
	require.Equal(t, http.StatusCreated, status)
	c := testharness.NewClient(t, env.ServerURL)
	require.Equal(t, http.StatusOK, c.Login(username, "rolepass123"))
	require.Equal(t, http.StatusOK, c.ChangePassword("rolepass123"))
	return c
}

func TestMQTTBrokerSecrets_NoResponseAtAnyRoleCarriesThePassword(t *testing.T) {
	const password = "write-only-broker-password-5d2e"
	id := createBrokerWithPassword(t, "write-only-broker", ptrStr(password))

	callers := map[string]*testharness.Client{
		"admin":  client,
		"user":   clientWithRole(t, "user"),
		"viewer": clientWithRole(t, "viewer"),
	}
	for role, caller := range callers {
		list, status := caller.ListMQTTBrokers()
		require.Equal(t, http.StatusOK, status, role)
		one, status := caller.GetMQTTBroker(id)
		require.Equal(t, http.StatusOK, status, role)

		for what, body := range map[string]json.RawMessage{"list": list, "get": one} {
			assert.NotContains(t, string(body), password, "%s %s", role, what)
		}
		var brokers []map[string]any
		require.NoError(t, json.Unmarshal(list, &brokers))
		for _, broker := range brokers {
			assert.NotContains(t, broker, "password", "%s list", role)
		}
		var broker map[string]any
		require.NoError(t, json.Unmarshal(one, &broker))
		assert.NotContains(t, broker, "password", "%s get", role)
		assert.Equal(t, "set", broker["password_status"], role)
	}
}

func TestMQTTBrokerSecrets_SetKeepAndClear(t *testing.T) {
	id := createBrokerWithPassword(t, "set-keep-clear-broker", ptrStr("first-password"))
	value, status := storedBrokerPassword(t, id)
	require.Equal(t, secrets.StatusSet, status)
	require.Equal(t, "first-password", value)

	update := func(password *string) {
		t.Helper()
		resp, code := client.UpdateMQTTBroker(id, gen.MQTTBroker{
			Name: "set-keep-clear-broker", Type: "external", Host: ptrStr("set-keep-clear-broker.example.com"),
			Port: ptrInt(1883), Username: ptrStr("hub"), Password: password,
		})
		require.Equal(t, http.StatusOK, code, "body: %s", resp)
	}

	update(nil)
	value, _ = storedBrokerPassword(t, id)
	assert.Equal(t, "first-password", value, "an omitted password keeps the stored one")

	update(ptrStr("****"))
	value, _ = storedBrokerPassword(t, id)
	assert.Equal(t, "first-password", value, "the placeholder keeps the stored one")

	update(ptrStr("second-password"))
	value, _ = storedBrokerPassword(t, id)
	assert.Equal(t, "second-password", value, "a new password replaces it")
	assert.Equal(t, "set", passwordStatusOf(t, id))

	update(ptrStr(""))
	_, status = storedBrokerPassword(t, id)
	assert.Equal(t, secrets.StatusUnset, status, "an empty password clears it")
	assert.Equal(t, "unset", passwordStatusOf(t, id))
}

func TestMQTTBrokerSecrets_DeletingTheBrokerDeletesItsSecret(t *testing.T) {
	id := createBrokerWithPassword(t, "deleted-with-secret-broker", ptrStr("doomed-password"))

	require.Equal(t, http.StatusNoContent, client.DeleteMQTTBroker(id))

	var rows int
	require.NoError(t, env.DB.Reader.QueryRow("SELECT COUNT(*) FROM secrets WHERE owner = ?",
		database.BrokerSecretOwner(id)).Scan(&rows))
	assert.Zero(t, rows)
}

func TestMQTTBrokerSecrets_TheConnectionUsesTheStoredPassword(t *testing.T) {
	external, err := testharness.StartExternalBrokerWithLogin("hub", "the-right-password")
	require.NoError(t, err)
	t.Cleanup(func() { _ = external.Stop() })

	resp, status := client.CreateMQTTBroker(gen.MQTTBroker{
		Name: "login-broker", Type: "external", Host: ptrStr("127.0.0.1"), Port: ptrInt(external.Port),
		Username: ptrStr("hub"), Password: ptrStr("the-right-password"), Enabled: false,
	})
	require.Equal(t, http.StatusCreated, status, "body: %s", resp)
	var created struct {
		ID int `json:"id"`
	}
	require.NoError(t, json.Unmarshal(resp, &created))
	t.Cleanup(func() {
		env.ConnectionManager.DisconnectBroker(created.ID)
		client.DeleteMQTTBroker(created.ID)
	})

	// The broker as the API returns it has no password: the connection manager
	// has to fetch it from the secret store.
	brokerJSON, status := client.GetMQTTBroker(created.ID)
	require.Equal(t, http.StatusOK, status)
	var broker gen.MQTTBroker
	require.NoError(t, json.Unmarshal(brokerJSON, &broker))
	require.Nil(t, broker.Password)

	require.NoError(t, env.ConnectionManager.ConnectBroker(context.Background(), broker))
	assert.Eventually(t, func() bool { return env.ConnectionManager.IsConnected(created.ID) },
		5*time.Second, 100*time.Millisecond)
}

func TestMQTTBrokerSecrets_CLIPromptLeftEmptyCreatesABrokerWithoutAPassword(t *testing.T) {
	term := startOnTerminal(t, withFlags(hubFlags(t), "mqtt", "brokers", "create",
		"--name", "cli-no-password-broker", "--host", "cli-no-password.example.com", "--username", "hub", "--enabled=false")...)
	term.answer("Password (leave empty for none): ", "")
	output, err := term.wait()
	require.NoError(t, err, output)
	start := strings.Index(output, "{")
	require.GreaterOrEqual(t, start, 0, "the created broker is printed as JSON; output: %q", output)
	var created struct {
		ID int `json:"id"`
	}
	require.NoError(t, json.Unmarshal([]byte(output[start:]), &created), output)
	t.Cleanup(func() { client.DeleteMQTTBroker(created.ID) })

	resp, status := client.GetMQTTBroker(created.ID)
	require.Equal(t, http.StatusOK, status)
	var broker map[string]any
	require.NoError(t, json.Unmarshal(resp, &broker))
	assert.Equal(t, "hub", broker["username"])
	assert.Equal(t, "unset", broker["password_status"])
	assert.NotContains(t, output, "Confirm password", "an empty password is not asked for twice")
}
