//go:build unix

package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSystemdCreds puts a systemd-creds on PATH that "seals" by prefixing
// SEALED: and records its arguments, and pretends the process is root.
func fakeSystemdCreds(t *testing.T) (argsLog string) {
	t.Helper()
	dir := t.TempDir()
	argsLog = filepath.Join(dir, "args")
	script := `#!/bin/sh
echo "$@" >> "` + argsLog + `"
case "$1" in
encrypt) printf 'SEALED:'; cat ;;
decrypt) sed 's/^SEALED://' "$3" ;;
esac
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "systemd-creds"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	asRoot(t, true)
	return argsLog
}

func asRoot(t *testing.T, root bool) {
	t.Helper()
	previous := isRoot
	isRoot = func() bool { return root }
	t.Cleanup(func() { isRoot = previous })
}

func TestInitKey_WritesTheKeyFileWithMode0600(t *testing.T) {
	f := newKeyFixture(t)
	key, err := GenerateKey()
	require.NoError(t, err)

	path, err := InitKey(f.locations, key, false)
	require.NoError(t, err)

	assert.Equal(t, f.locations.ConfigKeyFile(), path)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	shown, err := ShowKey(f.locations)
	require.NoError(t, err)
	assert.Equal(t, key.Encode(), shown.Encode())
}

func TestInitKey_RefusesWhenAKeyExistsInAnyForm(t *testing.T) {
	key, err := GenerateKey()
	require.NoError(t, err)
	for name, place := range map[string]func(f keyFixture) (Locations, string){
		"key file":          func(f keyFixture) (Locations, string) { return f.locations, f.locations.ConfigKeyFile() },
		"sealed credential": func(f keyFixture) (Locations, string) { return f.locations, f.locations.SealedKeyFile() },
		"Compose secret":    func(f keyFixture) (Locations, string) { return f.locations, f.locations.ComposeSecret },
		"systemd credential": func(f keyFixture) (Locations, string) {
			l := f.locations
			l.CredentialsDir = t.TempDir()
			return l, filepath.Join(l.CredentialsDir, "secrets.key")
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newKeyFixture(t)
			l, existing := place(f)
			require.NoError(t, os.MkdirAll(filepath.Dir(existing), 0o755))
			require.NoError(t, os.WriteFile(existing, []byte("existing"), 0o600))

			_, err := InitKey(l, key, false)

			require.ErrorIs(t, err, ErrKeyExists)
			assert.Contains(t, err.Error(), existing)
			content, _ := os.ReadFile(existing)
			assert.Equal(t, "existing", string(content), "the existing key is left alone")
		})
	}
}

func TestInitKey_RefusesWhenADropInPassesTheHubAKey(t *testing.T) {
	key, err := GenerateKey()
	require.NoError(t, err)
	for name, dropIn := range map[string]string{
		"sealed credential":      "[Service]\nLoadCredentialEncrypted=secrets.key:/elsewhere/secrets.key.cred\n",
		"plain credential":       "[Service]\nLoadCredential=secrets.key:/root/secrets.key\n",
		"credential given whole": "[Service]\nSetCredentialEncrypted=secrets.key: \\\n  k6iUCUh0RJCQyvL8k8q1UyAAAAABAAAADAAAABAAAAC1lFmbWAqWZ8dCCQkAAAAAgAAAA\n",
		"imported credential":    "[Service]\nImportCredential=secrets.*\n",
		"key file flag": "[Service]\nExecStart=\nExecStart=/usr/bin/sensor-hub local serve --config-dir=/etc/sensor-hub \\\n" +
			"  --secrets-key-file=/root/secrets.key\n",
	} {
		t.Run(name, func(t *testing.T) {
			f := newKeyFixture(t)
			path := filepath.Join(filepath.Dir(f.locations.SystemdDropIn), "override.conf")
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
			require.NoError(t, os.WriteFile(path, []byte(dropIn), 0o644))

			_, err := InitKey(f.locations, key, false)

			require.ErrorIs(t, err, ErrKeyExists)
			assert.Contains(t, err.Error(), path)
			assert.NoFileExists(t, f.locations.ConfigKeyFile())
		})
	}
}

func TestInitKey_IgnoresDropInsThatPassNoKey(t *testing.T) {
	f := newKeyFixture(t)
	dir := filepath.Dir(f.locations.SystemdDropIn)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	for name, content := range map[string]string{
		"environment.conf":  "[Service]\nEnvironment=OTEL_SERVICE_NAME=hub\nLoadCredential=tls.key:/etc/ssl/hub.key\n",
		"commented.conf":    "[Service]\n# LoadCredential=secrets.key:/root/secrets.key\n; ExecStart=--secrets-key-file=/x\n",
		"unit.conf":         "[Unit]\nDescription=LoadCredential=secrets.key\n",
		"override.conf.bak": "[Service]\nLoadCredential=secrets.key:/root/secrets.key\n",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	key, err := GenerateKey()
	require.NoError(t, err)

	path, err := InitKey(f.locations, key, false)

	require.NoError(t, err)
	assert.Equal(t, f.locations.ConfigKeyFile(), path)
}

func TestInitKey_RemovesTheSealedKeyWhenTheDropInCannotBeWritten(t *testing.T) {
	f := newKeyFixture(t)
	fakeSystemdCreds(t)
	// A directory where the drop-in goes cannot be written over, even by root.
	require.NoError(t, os.MkdirAll(f.locations.SystemdDropIn, 0o755))
	key, err := GenerateKey()
	require.NoError(t, err)

	_, err = InitKey(f.locations, key, true)

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrKeyExists))
	assert.NoFileExists(t, f.locations.SealedKeyFile(), "a sealed key systemd is never told about would stop the hub starting")
	_, err = InitKey(f.locations, key, false)
	require.NoError(t, err, "a key file can still be written after the failed seal")
}

func TestInitKey_SealsWithTheTPMAndWritesTheDropIn(t *testing.T) {
	f := newKeyFixture(t)
	argsLog := fakeSystemdCreds(t)
	key, err := GenerateKey()
	require.NoError(t, err)

	path, err := InitKey(f.locations, key, true)
	require.NoError(t, err)

	assert.Equal(t, f.locations.SealedKeyFile(), path)
	sealed, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "SEALED:"+key.Encode()+"\n", string(sealed))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	assert.NoFileExists(t, f.locations.ConfigKeyFile(), "the plaintext key is never written")

	args, err := os.ReadFile(argsLog)
	require.NoError(t, err)
	assert.Equal(t, "encrypt --with-key=tpm2 --name=secrets.key - -\n", string(args))

	dropIn, err := os.ReadFile(f.locations.SystemdDropIn)
	require.NoError(t, err)
	assert.Equal(t, "[Service]\nLoadCredentialEncrypted=secrets.key:"+path+"\n", string(dropIn))

	shown, err := ShowKey(f.locations)
	require.NoError(t, err)
	assert.Equal(t, key.Encode(), shown.Encode())
}

func TestInitKey_SealingNeedsRoot(t *testing.T) {
	f := newKeyFixture(t)
	asRoot(t, false)
	key, err := GenerateKey()
	require.NoError(t, err)

	_, err = InitKey(f.locations, key, true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "root")
	assert.NoFileExists(t, f.locations.SealedKeyFile())
}

func TestShowKey_ASealedKeyNeedsRoot(t *testing.T) {
	f := newKeyFixture(t)
	asRoot(t, false)
	require.NoError(t, os.WriteFile(f.locations.SealedKeyFile(), []byte("SEALED:x"), 0o600))

	_, err := ShowKey(f.locations)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "needs root")
}

func TestShowKey_ErrorsWhenThereIsNoKey(t *testing.T) {
	f := newKeyFixture(t)

	_, err := ShowKey(f.locations)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no secret-store key")
}
