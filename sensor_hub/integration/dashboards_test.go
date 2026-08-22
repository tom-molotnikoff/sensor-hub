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

func TestDashboards_CreateAndList(t *testing.T) {
	resp, status := client.CreateDashboard("Integration Dashboard")
	require.Equal(t, http.StatusCreated, status)

	var created struct {
		ID int `json:"id"`
	}
	require.NoError(t, json.Unmarshal(resp, &created))
	require.True(t, created.ID > 0)

	list, status := client.ListDashboards()
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(list), "Integration Dashboard")
}

func TestDashboards_GetByID(t *testing.T) {
	resp, status := client.CreateDashboard("Get Test Dashboard")
	require.Equal(t, http.StatusCreated, status)

	var created struct {
		ID int `json:"id"`
	}
	json.Unmarshal(resp, &created)

	detail, status := client.GetDashboard(created.ID)
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(detail), "Get Test Dashboard")
}

func TestDashboards_GetByID_NotFound(t *testing.T) {
	_, status := client.GetDashboard(99999)
	assert.Equal(t, http.StatusNotFound, status)
}

func TestDashboards_Update(t *testing.T) {
	resp, status := client.CreateDashboard("Before Update")
	require.Equal(t, http.StatusCreated, status)

	var created struct {
		ID int `json:"id"`
	}
	json.Unmarshal(resp, &created)

	_, status = client.UpdateDashboard(created.ID, gen.UpdateDashboardRequest{
		Name: ptrStr("After Update"),
	})
	require.Equal(t, http.StatusOK, status)

	detail, status := client.GetDashboard(created.ID)
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(detail), "After Update")
}

func TestDashboards_Delete(t *testing.T) {
	resp, status := client.CreateDashboard("To Delete")
	require.Equal(t, http.StatusCreated, status)

	var created struct {
		ID int `json:"id"`
	}
	json.Unmarshal(resp, &created)

	status = client.DeleteDashboard(created.ID)
	assert.Equal(t, http.StatusOK, status)

	_, status = client.GetDashboard(created.ID)
	assert.Equal(t, http.StatusNotFound, status)
}

func TestDashboards_SetDefault(t *testing.T) {
	resp, status := client.CreateDashboard("Default Dashboard")
	require.Equal(t, http.StatusCreated, status)

	var created struct {
		ID int `json:"id"`
	}
	json.Unmarshal(resp, &created)

	_, status = client.SetDefaultDashboard(created.ID)
	assert.Equal(t, http.StatusOK, status)
}

func TestDashboards_CreateWithoutName(t *testing.T) {
	_, status := client.CreateDashboard("")
	assert.Equal(t, http.StatusBadRequest, status)
}

func TestDashboards_ViewerCannotManage(t *testing.T) {
	// Create a user that has no manage_dashboards permission
	user := gen.CreateUserRequest{
		Username: "dashboard-viewer",
		Password: "viewerpass789",
		Email:    ptrStr("dashviewer@test.com"),
	}
	client.CreateUser(user)

	viewer := testharness.NewClient(t, env.ServerURL)
	status := viewer.Login("dashboard-viewer", "viewerpass789")
	require.Equal(t, http.StatusOK, status)

	// Non-admin user should not be able to create dashboards
	_, status = viewer.CreateDashboard("Should Fail")
	assert.Equal(t, http.StatusForbidden, status)
}

func TestDashboards_UnauthenticatedAccessDenied(t *testing.T) {
	unauthed := testharness.NewClient(t, env.ServerURL)
	_, status := unauthed.ListDashboards()
	assert.Equal(t, http.StatusUnauthorized, status)
}
