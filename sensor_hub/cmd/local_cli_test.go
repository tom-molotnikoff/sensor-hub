package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootHelp_GroupsLocalApartFromHubCommands(t *testing.T) {
	help, _, err := executeRootCommand(t, "--help")
	require.NoError(t, err)

	groups := map[string][]string{}
	var current string
	for _, line := range nonEmptyLines(help) {
		if !strings.HasPrefix(line, " ") {
			current = line
			continue
		}
		groups[current] = append(groups[current], strings.Fields(line)[0])
	}

	assert.Equal(t, []string{"local"}, groups["Commands that act on this machine's install:"])
	assert.Contains(t, groups["Commands that talk to a hub:"], "sensors")
	assert.NotContains(t, groups["Commands that talk to a hub:"], "local")
	assert.NotContains(t, help, "Additional Commands:")
}

func TestServe_IsUnknownOutsideLocal(t *testing.T) {
	_, _, err := executeRootCommand(t, "serve")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown command "serve"`)
}

func TestLocalCommands_NameTheMissingConfigFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "application.properties"), nil, 0o600))

	for _, args := range [][]string{
		{"local", "serve"},
		{"local", "admin", "create", "someone"},
		{"local", "db", "backup", filepath.Join(dir, "backup.db")},
		{"local", "secrets", "init-key"},
		{"local", "secrets", "show-key"},
		{"local", "secrets", "check-seal"},
	} {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			_, _, err := executeRootCommand(t, append(args, "--config-dir", dir)...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), dir)
			assert.Contains(t, err.Error(), "database.properties")
		})
	}
}
