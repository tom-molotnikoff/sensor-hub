//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"slices"
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

func TestUsers_DisabledUsersSessionAndApiKeyAreRejected(t *testing.T) {
	user, userID := signedInUser(t, "user-to-disable", "user")
	key, _ := createApiKey(t, user, "disabled-user-key")
	_, status := user.GetMe()
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, http.StatusOK, apiKeyWorks(t, key))

	_, status = client.SetUserDisabled(userID, true)
	require.Equal(t, http.StatusOK, status)

	_, status = user.GetMe()
	assert.Equal(t, http.StatusUnauthorized, status, "the session cookie")
	assert.Equal(t, http.StatusUnauthorized, apiKeyWorks(t, key), "the API key")
	var sessions int
	require.NoError(t, env.DB.Reader.QueryRow(`SELECT COUNT(*) FROM sessions WHERE user_id = ?`, userID).Scan(&sessions))
	assert.Zero(t, sessions, "disabling deletes the sessions")

	body, status := testharness.NewClient(t, env.ServerURL).LoginBody("user-to-disable", "integration-pass-1")
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.JSONEq(t, `{"message":"account disabled"}`, string(body))

	_, status = client.SetUserDisabled(userID, false)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, http.StatusOK, apiKeyWorks(t, key), "enabling restores the API key")
	assert.Equal(t, http.StatusOK, testharness.NewClient(t, env.ServerURL).Login("user-to-disable", "integration-pass-1"))
}

func TestUsers_CannotDisableThemselves(t *testing.T) {
	_, status := client.SetUserDisabled(currentUserID(t, client), true)
	assert.Equal(t, http.StatusBadRequest, status)
}

func TestUsers_ViewerCannotDisableAUser(t *testing.T) {
	viewer, _ := signedInUser(t, "viewer-disabling", "viewer")
	_, target := signedInUser(t, "viewer-disable-target", "viewer")

	_, status := viewer.SetUserDisabled(target, true)
	assert.Equal(t, http.StatusForbidden, status)
}

// The last enabled admin can only be targeted by a caller who holds
// manage_users without being an admin, since an admin cannot disable
// themselves. The test grants manage_users to the user role for its duration
// and disables every other admin first.
func TestUsers_LastEnabledAdminCannotBeDisabled(t *testing.T) {
	adminID := currentUserID(t, client)
	userRoleID, manageUsersID := roleID(t, "user"), permissionID(t, "manage_users")
	require.Equal(t, http.StatusOK, client.AssignPermission(userRoleID, manageUsersID))
	t.Cleanup(func() { client.RemovePermission(userRoleID, manageUsersID) })

	resp, status := client.ListUsers()
	require.Equal(t, http.StatusOK, status)
	var users []gen.User
	require.NoError(t, json.Unmarshal(resp, &users))
	for _, u := range users {
		if u.Id == adminID || u.Disabled || !slices.Contains(u.Roles, "admin") {
			continue
		}
		_, status := client.SetUserDisabled(u.Id, true)
		require.Equal(t, http.StatusOK, status)
		t.Cleanup(func() { client.SetUserDisabled(u.Id, false) })
	}

	manager, _ := signedInUser(t, "user-manager", "user")
	body, status := manager.SetUserDisabled(adminID, true)
	assert.Equal(t, http.StatusConflict, status, string(body))

	_, status = client.GetMe()
	assert.Equal(t, http.StatusOK, status, "the admin is still enabled and signed in")
}

func currentUserID(t *testing.T, c *testharness.Client) int {
	t.Helper()
	resp, status := c.GetMe()
	require.Equal(t, http.StatusOK, status)
	var me struct {
		User gen.User `json:"user"`
	}
	require.NoError(t, json.Unmarshal(resp, &me))
	require.NotZero(t, me.User.Id)
	return me.User.Id
}

func roleID(t *testing.T, name string) int {
	t.Helper()
	resp, status := client.ListRoles()
	require.Equal(t, http.StatusOK, status)
	var roles []gen.RoleInfo
	require.NoError(t, json.Unmarshal(resp, &roles))
	for _, r := range roles {
		if r.Name == name {
			return r.Id
		}
	}
	t.Fatalf("no role %q", name)
	return 0
}

func permissionID(t *testing.T, name string) int {
	t.Helper()
	perms, status := client.ListPermissions()
	require.Equal(t, http.StatusOK, status)
	for _, p := range perms {
		if p.Name == name {
			return p.Id
		}
	}
	t.Fatalf("no permission %q", name)
	return 0
}
