package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthLogin_RefusesAnEmptyPasswordWithoutARequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer server.Close()
	rootCmd.SetIn(strings.NewReader("\n"))
	t.Cleanup(func() { rootCmd.SetIn(nil) })

	_, _, err := executeRootCommand(t, "--server", server.URL, "auth", "login", "--username", "alice", "--password-stdin")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "password must not be empty")
	assert.Zero(t, requests, "no request reaches the hub")
}
