//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	gen "example/sensorHub/gen"
	"example/sensorHub/testharness"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// signedInUser creates a user with one role, signs them in and clears the
// password change every new user starts with.
func signedInUser(t *testing.T, username, role string) (*testharness.Client, int) {
	t.Helper()
	const password = "integration-pass-1"
	resp, status := client.CreateUser(gen.CreateUserRequest{
		Username: username,
		Password: password,
		Email:    ptrStr(username + "@test.com"),
		Roles:    &[]string{role},
	})
	require.Equal(t, http.StatusCreated, status, string(resp))
	var created struct {
		ID int `json:"id"`
	}
	require.NoError(t, json.Unmarshal(resp, &created))

	c := testharness.NewClient(t, env.ServerURL)
	require.Equal(t, http.StatusOK, c.Login(username, password))
	require.Equal(t, http.StatusOK, c.ChangePassword(password))
	return c, created.ID
}

// createApiKey creates a key as the given client and returns the key and its id.
func createApiKey(t *testing.T, c *testharness.Client, name string) (string, int) {
	t.Helper()
	resp, status := c.CreateApiKey(name)
	require.Equal(t, http.StatusCreated, status, string(resp))
	var created struct {
		Key string `json:"key"`
	}
	require.NoError(t, json.Unmarshal(resp, &created))

	keys, status := c.ListApiKeys()
	require.Equal(t, http.StatusOK, status)
	for _, k := range keys {
		if k.Name != nil && *k.Name == name {
			return created.Key, *k.Id
		}
	}
	t.Fatalf("api key %q not in the owner's list", name)
	return "", 0
}

func apiKeyWorks(t *testing.T, key string) int {
	t.Helper()
	_, status := testharness.NewApiKeyClient(t, env.ServerURL, key).GetMe()
	return status
}

func TestApiKeys_ViewerCannotTouchAnAdminsKey(t *testing.T) {
	viewer, _ := signedInUser(t, "key-owner-viewer", "viewer")
	adminKey, adminKeyID := createApiKey(t, client, "admin-key-viewer-targets")
	t.Cleanup(func() { client.DeleteApiKey(adminKeyID) })

	expiry := time.Now().Add(time.Hour)
	assert.Equal(t, http.StatusNotFound, viewer.RevokeApiKey(adminKeyID))
	assert.Equal(t, http.StatusNotFound, viewer.UpdateApiKeyExpiry(adminKeyID, &expiry))
	assert.Equal(t, http.StatusNotFound, viewer.DeleteApiKey(adminKeyID))

	assert.Equal(t, http.StatusOK, apiKeyWorks(t, adminKey), "the admin's key still works")
	keys, _ := client.ListApiKeys()
	for _, k := range keys {
		if *k.Id == adminKeyID {
			assert.False(t, *k.Revoked)
			assert.Nil(t, k.ExpiresAt)
		}
	}
}

func TestApiKeys_ViewerManagesTheirOwnKey(t *testing.T) {
	viewer, _ := signedInUser(t, "own-key-viewer", "viewer")
	key, keyID := createApiKey(t, viewer, "viewer-own-key")

	assert.Equal(t, http.StatusOK, viewer.RevokeApiKey(keyID))
	assert.Equal(t, http.StatusUnauthorized, apiKeyWorks(t, key))
	assert.Equal(t, http.StatusOK, viewer.DeleteApiKey(keyID))
}

func TestApiKeys_AdminRevokesAViewersKey(t *testing.T) {
	viewer, _ := signedInUser(t, "revoked-key-viewer", "viewer")
	key, keyID := createApiKey(t, viewer, "viewer-key-admin-revokes")
	require.Equal(t, http.StatusOK, apiKeyWorks(t, key))

	expiry := time.Now().Add(time.Hour)
	assert.Equal(t, http.StatusOK, client.UpdateApiKeyExpiry(keyID, &expiry))
	assert.Equal(t, http.StatusOK, client.RevokeApiKey(keyID))
	assert.Equal(t, http.StatusUnauthorized, apiKeyWorks(t, key))
	assert.Equal(t, http.StatusOK, client.DeleteApiKey(keyID))

	keys, status := viewer.ListApiKeys()
	require.Equal(t, http.StatusOK, status)
	assert.Empty(t, keys)
}

func TestApiKeys_MustChangeUserIsHeldToTheAllowList(t *testing.T) {
	user, userID := signedInUser(t, "must-change-key-user", "user")
	key, _ := createApiKey(t, user, "must-change-key")
	keyClient := testharness.NewApiKeyClient(t, env.ServerURL, key)
	_, status := keyClient.ListApiKeys()
	require.Equal(t, http.StatusOK, status)

	require.Equal(t, http.StatusOK, client.SetMustChangePassword(userID, true))

	_, status = keyClient.ListApiKeys()
	assert.Equal(t, http.StatusForbidden, status, "a route outside the allow-list")
	_, status = keyClient.GetMe()
	assert.Equal(t, http.StatusOK, status, "a route on the allow-list")
}
