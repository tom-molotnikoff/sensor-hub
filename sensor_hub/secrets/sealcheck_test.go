//go:build unix

package secrets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tpmStandIn puts a systemd-creds on PATH that seals by prefixing the key it
// used, such as "tpm2:", and unseals only what the stand-in TPM still accepts:
// a credential sealed under a TPM state it has since lost does not decrypt.
type tpmStandIn struct {
	dir string
}

func newTPMStandIn(t *testing.T) tpmStandIn {
	t.Helper()
	dir := t.TempDir()
	script := `#!/bin/sh
echo "$1" >> "` + dir + `/calls"
case "$1" in
encrypt)
  case "$2" in --with-key=tpm2) [ -e "` + dir + `/no-tpm" ] && { echo "No TPM2 device" >&2; exit 1; } ;; esac
  printf '%s:' "${2#--with-key=}"; cat ;;
decrypt)
  case "$(cat "$3")" in
  tpm2:*|host:*) sed 's/^[a-z0-9]*://' "$3" ;;
  *) echo "Failed to unseal secret using TPM2: State not recoverable" >&2; exit 1 ;;
  esac ;;
esac
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "systemd-creds"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	asRoot(t, true)
	previous := unsealRetryDelay
	unsealRetryDelay = 0
	t.Cleanup(func() { unsealRetryDelay = previous })
	return tpmStandIn{dir: dir}
}

func (s tpmStandIn) calls(t *testing.T, command string) int {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(s.dir, "calls"))
	require.NoError(t, err)
	return strings.Count(string(content), command+"\n")
}

var checkedAt = time.Date(2026, 10, 8, 21, 55, 0, 0, time.UTC)

func writeSealedKey(t *testing.T, l Locations, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(l.SealedKeyFile(), []byte(content), 0o600))
}

func TestCheckSealedKey_LeavesAKeyThatUnsealsAlone(t *testing.T) {
	newTPMStandIn(t)
	f := newKeyFixture(t)
	key, err := GenerateKey()
	require.NoError(t, err)
	writeSealedKey(t, f.locations, "tpm2:"+key.Encode()+"\n")

	check, err := CheckSealedKey(f.locations, checkedAt)

	require.NoError(t, err)
	assert.False(t, check.Replaced)
	shown, err := ShowKey(f.locations)
	require.NoError(t, err)
	assert.Equal(t, key.Encode(), shown.Encode())
	entries, err := os.ReadDir(f.locations.ConfigDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "nothing is set aside or left behind")
}

func TestCheckSealedKey_SealsANewKeyWhenTheTPMRefusesTheOldOne(t *testing.T) {
	tpm := newTPMStandIn(t)
	f := newKeyFixture(t)
	writeSealedKey(t, f.locations, "sealed under a TPM state that is gone")

	check, err := CheckSealedKey(f.locations, checkedAt)

	require.NoError(t, err)
	assert.True(t, check.Replaced)
	assert.ErrorContains(t, check.Reason, "State not recoverable")
	assert.Equal(t, "tpm2", check.SealedWith)
	assert.Equal(t, unsealAttempts, tpm.calls(t, "decrypt"), "the TPM is given a few chances first")
	assert.Equal(t, f.locations.SealedKeyFile()+".unsealable-20261008T215500Z", check.SetAside)
	old, err := os.ReadFile(check.SetAside)
	require.NoError(t, err)
	assert.Equal(t, "sealed under a TPM state that is gone", string(old), "the old key is kept")
	info, err := os.Stat(f.locations.SealedKeyFile())
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	_, err = ShowKey(f.locations)
	assert.NoError(t, err, "the new sealed key unseals")
	_, err = os.Stat(f.locations.SealedKeyFile() + ".new")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestCheckSealedKey_SealsWithTheHostKeyWhenTheTPMWillNotSealEither(t *testing.T) {
	tpm := newTPMStandIn(t)
	require.NoError(t, os.WriteFile(filepath.Join(tpm.dir, "no-tpm"), nil, 0o600))
	f := newKeyFixture(t)
	writeSealedKey(t, f.locations, "sealed to a TPM this host no longer has")

	check, err := CheckSealedKey(f.locations, checkedAt)

	require.NoError(t, err)
	assert.True(t, check.Replaced)
	assert.Equal(t, "host", check.SealedWith)
	_, err = ShowKey(f.locations)
	assert.NoError(t, err)
}

func TestCheckSealedKey_ReplacesASealedKeyThatHoldsNoKey(t *testing.T) {
	newTPMStandIn(t)
	f := newKeyFixture(t)
	writeSealedKey(t, f.locations, "tpm2:not a key\n")

	check, err := CheckSealedKey(f.locations, checkedAt)

	require.NoError(t, err)
	assert.True(t, check.Replaced)
	assert.ErrorContains(t, check.Reason, "does not hold a secret-store key")
}

func TestCheckSealedKey_ChangesNothingWhenSystemdCredsCannotRun(t *testing.T) {
	asRoot(t, true)
	t.Setenv("PATH", t.TempDir())
	f := newKeyFixture(t)
	writeSealedKey(t, f.locations, "sealed")

	check, err := CheckSealedKey(f.locations, checkedAt)

	require.Error(t, err)
	assert.False(t, check.Replaced)
	content, readErr := os.ReadFile(f.locations.SealedKeyFile())
	require.NoError(t, readErr)
	assert.Equal(t, "sealed", string(content))
}

func TestCheckSealedKey_DoesNothingWithoutASealedKey(t *testing.T) {
	newTPMStandIn(t)
	f := newKeyFixture(t)

	check, err := CheckSealedKey(f.locations, checkedAt)

	require.NoError(t, err)
	assert.False(t, check.Replaced)
	_, err = os.Stat(f.locations.SealedKeyFile())
	assert.ErrorIs(t, err, os.ErrNotExist, "no key is created")
}
