//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	gen "example/sensorHub/gen"
	"example/sensorHub/testharness"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsers_CreateAndList(t *testing.T) {
	user := gen.CreateUserRequest{
		Username: "integration-viewer",
		Password: "viewerpass123",
		Email:    ptrStr("viewer@test.com"),
	}
	_, status := client.CreateUser(user)
	require.Equal(t, http.StatusCreated, status)

	resp, status := client.ListUsers()
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(resp), "integration-viewer")
}

func TestUsers_ViewerCannotAccessAdminEndpoints(t *testing.T) {
	// Create a viewer user and log in as them
	user := gen.CreateUserRequest{
		Username: "viewer-restricted",
		Password: "viewerpass456",
		Email:    ptrStr("restricted@test.com"),
	}
	client.CreateUser(user)

	viewer := testharness.NewClient(t, env.ServerURL)
	status := viewer.Login("viewer-restricted", "viewerpass456")
	require.Equal(t, http.StatusOK, status)

	// Viewer should not be able to create sensors (requires manage_sensors)
	_, status = viewer.ListUsers()
	assert.Equal(t, http.StatusForbidden, status)
}

func TestUsers_ListRoles(t *testing.T) {
	resp, status := client.ListRoles()
	require.Equal(t, http.StatusOK, status)

	var roles []json.RawMessage
	require.NoError(t, json.Unmarshal(resp, &roles))
	assert.NotEmpty(t, roles, "should have default roles")
}

func TestUsers_Delete(t *testing.T) {
	user := gen.CreateUserRequest{
		Username: "user-to-delete",
		Password: "deletepass123",
		Email:    ptrStr("delete@test.com"),
	}
	resp, status := client.CreateUser(user)
	require.Equal(t, http.StatusCreated, status)

	var created struct {
		ID int `json:"id"`
	}
	json.Unmarshal(resp, &created)
	require.True(t, created.ID > 0)

	status = client.DeleteUser(created.ID)
	assert.Equal(t, http.StatusOK, status)
}

func TestUsers_DeleteRemovesOwnedApiKeys(t *testing.T) {
	user := gen.CreateUserRequest{
		Username: "user-with-api-key",
		Password: "deletepass123",
		Email:    ptrStr("user-with-api-key@test.com"),
		Roles:    &[]string{"user"},
	}
	resp, status := client.CreateUser(user)
	require.Equal(t, http.StatusCreated, status)

	var created struct {
		ID int `json:"id"`
	}
	require.NoError(t, json.Unmarshal(resp, &created))
	require.True(t, created.ID > 0)

	userClient := testharness.NewClient(t, env.ServerURL)
	require.Equal(t, http.StatusOK, userClient.Login(user.Username, user.Password))
	require.Equal(t, http.StatusOK, userClient.ChangePassword("deletepass456"))

	_, status = userClient.CreateApiKey("delete-test-key")
	require.Equal(t, http.StatusCreated, status)

	status = client.DeleteUser(created.ID)
	assert.Equal(t, http.StatusOK, status)

	var apiKeyCount int
	require.NoError(t, env.DB.Reader.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE user_id = ?`, created.ID).Scan(&apiKeyCount))
	assert.Zero(t, apiKeyCount)
}

func TestUsers_DeleteRemovesSensorCommandHistory(t *testing.T) {
	fixture := setupCommandFixture(t, "delete-history-plug")
	defer fixture.stop()

	user := gen.CreateUserRequest{
		Username: "user-with-command-history",
		Password: "deletepass123",
		Email:    ptrStr("user-with-command-history@test.com"),
		Roles:    &[]string{"user"},
	}
	resp, status := client.CreateUser(user)
	require.Equal(t, http.StatusCreated, status)

	var created struct {
		ID int `json:"id"`
	}
	require.NoError(t, json.Unmarshal(resp, &created))
	require.True(t, created.ID > 0)

	userClient := testharness.NewClient(t, env.ServerURL)
	require.Equal(t, http.StatusOK, userClient.Login(user.Username, user.Password))
	require.Equal(t, http.StatusOK, userClient.ChangePassword("deletepass456"))

	command, status := userClient.SendSensorCommand(fixture.sensor.Id, "state", "ON")
	require.Equal(t, http.StatusAccepted, status)

	var historyCount int
	require.NoError(t, env.DB.Reader.QueryRow(`SELECT COUNT(*) FROM sensor_command_history WHERE id = ?`, command.Id).Scan(&historyCount))
	require.Equal(t, 1, historyCount)

	status = client.DeleteUser(created.ID)
	assert.Equal(t, http.StatusOK, status)

	require.NoError(t, env.DB.Reader.QueryRow(`SELECT COUNT(*) FROM sensor_command_history WHERE id = ?`, command.Id).Scan(&historyCount))
	assert.Zero(t, historyCount)
}
