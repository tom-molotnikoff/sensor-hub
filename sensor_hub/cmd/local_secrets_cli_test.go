//go:build unix

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// localConfig writes a configuration directory a local command accepts.
func localConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"application.properties", "database.properties"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), nil, 0o600))
	}
	return dir
}

func TestLocalSecretsInitKey_WritesAKeyThatShowKeyPrintsAndRefusesASecond(t *testing.T) {
	configDir := localConfig(t)

	_, stderr, err := executeRootCommand(t, "local", "secrets", "init-key", "--config-dir", configDir,
		"--from-stdin=false", "--seal=false")
	require.NoError(t, err)
	keyPath := filepath.Join(configDir, "secrets.key")
	assert.Contains(t, stderr, keyPath)
	info, err := os.Stat(keyPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	written, err := os.ReadFile(keyPath)
	require.NoError(t, err)

	stdout, _, err := executeRootCommand(t, "local", "secrets", "show-key", "--config-dir", configDir)
	require.NoError(t, err)
	assert.Equal(t, string(written), stdout, "the key on one line and nothing else")

	_, _, err = executeRootCommand(t, "local", "secrets", "init-key", "--config-dir", configDir,
		"--from-stdin=false", "--seal=false")
	require.Error(t, err)
	assert.Contains(t, err.Error(), keyPath)
	again, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	assert.Equal(t, written, again, "the existing key is left alone")
}

func TestLocalSecretsInitKey_ReadsTheKeyFromStdin(t *testing.T) {
	configDir := localConfig(t)
	key := strings.Repeat("A", 43) + "="
	withStdin(t, key+"\n")

	_, _, err := executeRootCommand(t, "local", "secrets", "init-key", "--config-dir", configDir,
		"--from-stdin", "--seal=false")
	require.NoError(t, err)

	stdout, _, err := executeRootCommand(t, "local", "secrets", "show-key", "--config-dir", configDir)
	require.NoError(t, err)
	assert.Equal(t, key+"\n", stdout)
}

func TestLocalSecretsInitKey_RefusesAMalformedKeyOnStdin(t *testing.T) {
	configDir := localConfig(t)
	withStdin(t, "not a key\n")

	_, _, err := executeRootCommand(t, "local", "secrets", "init-key", "--config-dir", configDir,
		"--from-stdin", "--seal=false")

	require.Error(t, err)
	assert.NoFileExists(t, filepath.Join(configDir, "secrets.key"))
}
