//go:build unix

package secrets

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type keyFixture struct {
	locations    Locations
	databasePath string
	logs         *bytes.Buffer
	logger       *slog.Logger
}

// newKeyFixture lays out a configuration directory and a database directory
// apart from each other, with no key anywhere.
func newKeyFixture(t *testing.T) keyFixture {
	t.Helper()
	root := t.TempDir()
	configDir := filepath.Join(root, "etc")
	dataDir := filepath.Join(root, "data")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.MkdirAll(dataDir, 0o755))
	logs := &bytes.Buffer{}
	return keyFixture{
		locations: Locations{
			ComposeSecret: filepath.Join(root, "run", "secrets", "sensor-hub-secrets-key"),
			ConfigDir:     configDir,
			SystemdDropIn: filepath.Join(root, "systemd", "sensor-hub.service.d", "secrets-key.conf"),
		},
		databasePath: filepath.Join(dataDir, "sensor_hub.db"),
		logs:         logs,
		logger:       slog.New(slog.NewTextHandler(logs, nil)),
	}
}

func writeTestKey(t *testing.T, path string, mode os.FileMode) Key {
	t.Helper()
	key, err := GenerateKey()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(key.Encode()+"\n"), mode))
	require.NoError(t, os.Chmod(path, mode))
	return key
}

func TestLoadKey_GeneratesAKeyInTheConfigDirectoryWhenThereIsNone(t *testing.T) {
	f := newKeyFixture(t)

	key, err := LoadKey(f.locations, f.databasePath, f.logger)
	require.NoError(t, err)

	path := filepath.Join(f.locations.ConfigDir, "secrets.key")
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, key.Encode()+"\n", string(content))
	assert.Len(t, key.Encode(), 44, "32 bytes of standard base64")
	assert.Contains(t, f.logs.String(), "generated")
	assert.Contains(t, f.logs.String(), path)

	again, err := LoadKey(f.locations, f.databasePath, f.logger)
	require.NoError(t, err)
	assert.Equal(t, key.Encode(), again.Encode(), "the next start reads the generated key")
}

func TestLoadKey_TakesTheFirstLocationThatHoldsAKey(t *testing.T) {
	f := newKeyFixture(t)
	credentials := t.TempDir()
	flagPath := filepath.Join(t.TempDir(), "flag.key")
	configKey := writeTestKey(t, filepath.Join(f.locations.ConfigDir, "secrets.key"), 0o600)
	flagKey := writeTestKey(t, flagPath, 0o600)
	composeKey := writeTestKey(t, f.locations.ComposeSecret, 0o644)
	credentialKey := writeTestKey(t, filepath.Join(credentials, "secrets.key"), 0o644)

	l := f.locations
	l.CredentialsDir, l.KeyFile = credentials, flagPath
	steps := []struct {
		want   Key
		remove string
	}{
		{credentialKey, filepath.Join(credentials, "secrets.key")},
		{composeKey, f.locations.ComposeSecret},
		{flagKey, flagPath},
		{configKey, ""},
	}
	for i, step := range steps {
		key, err := LoadKey(l, f.databasePath, f.logger)
		require.NoError(t, err, "step %d", i)
		assert.Equal(t, step.want.Encode(), key.Encode(), "step %d", i)
		if step.remove == flagPath {
			l.KeyFile = ""
		} else if step.remove != "" {
			require.NoError(t, os.Remove(step.remove))
		}
	}
}

func TestLoadKey_RefusesAKeyFileReadableByOthers(t *testing.T) {
	f := newKeyFixture(t)
	flagPath := filepath.Join(t.TempDir(), "flag.key")
	writeTestKey(t, flagPath, 0o644)
	writeTestKey(t, filepath.Join(f.locations.ConfigDir, "secrets.key"), 0o604)

	l := f.locations
	l.KeyFile = flagPath
	_, err := LoadKey(l, f.databasePath, f.logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "readable by others")
	assert.Contains(t, err.Error(), flagPath)

	_, err = LoadKey(f.locations, f.databasePath, f.logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "readable by others")
}

func TestLoadKey_RefusesAKeyFileInsideTheDatabaseDirectory(t *testing.T) {
	f := newKeyFixture(t)
	dataDir := filepath.Dir(f.databasePath)
	for _, path := range []string{filepath.Join(dataDir, "secrets.key"), filepath.Join(dataDir, "keys", "secrets.key")} {
		writeTestKey(t, path, 0o600)
		l := f.locations
		l.KeyFile = path

		_, err := LoadKey(l, f.databasePath, f.logger)

		require.Error(t, err, path)
		assert.Contains(t, err.Error(), "directory holding the database", path)
	}
}

func TestLoadKey_RefusesToGenerateIntoTheDatabaseDirectory(t *testing.T) {
	f := newKeyFixture(t)
	l := f.locations
	l.ConfigDir = filepath.Dir(f.databasePath)

	_, err := LoadKey(l, f.databasePath, f.logger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory holding the database")
	assert.NoFileExists(t, filepath.Join(l.ConfigDir, "secrets.key"))
}

func TestLoadKey_AMissingFlagFileIsAnError(t *testing.T) {
	f := newKeyFixture(t)
	l := f.locations
	l.KeyFile = filepath.Join(t.TempDir(), "typo.key")

	_, err := LoadKey(l, f.databasePath, f.logger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--secrets-key-file")
	assert.NoFileExists(t, filepath.Join(l.ConfigDir, "secrets.key"), "no key is generated in its place")
}

func TestLoadKey_DoesNotGenerateBesideASealedKey(t *testing.T) {
	f := newKeyFixture(t)
	require.NoError(t, os.WriteFile(f.locations.SealedKeyFile(), []byte("sealed"), 0o600))

	_, err := LoadKey(f.locations, f.databasePath, f.logger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "sealed")
	assert.NoFileExists(t, f.locations.ConfigKeyFile())
}

func TestLoadKey_RefusesAMalformedKey(t *testing.T) {
	f := newKeyFixture(t)
	require.NoError(t, os.WriteFile(f.locations.ConfigKeyFile(), []byte("too short\n"), 0o600))

	_, err := LoadKey(f.locations, f.databasePath, f.logger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), f.locations.ConfigKeyFile())
}
