//go:build unix

package secrets

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tpmStandIn puts a systemd-creds and a systemctl on PATH. The systemd-creds
// seals by prefixing the key it used, such as "tpm2:", and unseals only what
// the stand-in TPM still accepts: a credential sealed under a TPM state it has
// since lost does not decrypt. The systemctl reports the hub's unit state.
type tpmStandIn struct {
	dir string
}

func newTPMStandIn(t *testing.T) tpmStandIn {
	t.Helper()
	dir := t.TempDir()
	systemdCreds := `#!/bin/sh
case "$1" in
has-tpm2) [ -e "` + dir + `/no-tpm" ] && exit 1; exit 0 ;;
encrypt)
  case "$2" in --with-key=tpm2) [ -e "` + dir + `/no-tpm" ] && { echo "No TPM2 device" >&2; exit 1; } ;; esac
  printf '%s:' "${2#--with-key=}"; cat ;;
decrypt)
  failures=$(cat "` + dir + `/failing-unseals" 2>/dev/null || echo 0)
  if [ "$failures" -gt 0 ]; then
    echo $((failures - 1)) > "` + dir + `/failing-unseals"
    echo "TPM2 resource manager busy" >&2; exit 1
  fi
  case "$(cat "$3")" in
  tpm2:*|host:*) sed 's/^[a-z0-9]*://' "$3" ;;
  *) echo "Failed to unseal secret using TPM2: State not recoverable" >&2; exit 1 ;;
  esac ;;
esac
`
	systemctl := `#!/bin/sh
state=$(cat "` + dir + `/hub-state" 2>/dev/null || echo inactive)
echo "$state"
[ "$state" = active ]
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "systemd-creds"), []byte(systemdCreds), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "systemctl"), []byte(systemctl), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	asRoot(t, true)
	previous := unsealRetryDelay
	unsealRetryDelay = 0
	t.Cleanup(func() { unsealRetryDelay = previous })
	return tpmStandIn{dir: dir}
}

func (s tpmStandIn) set(t *testing.T, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(s.dir, name), []byte(content), 0o600))
}

// removeTPM makes the TPM unusable: has-tpm2 fails and nothing seals to it.
func (s tpmStandIn) removeTPM(t *testing.T) { s.set(t, "no-tpm", "") }

var checkedAt = time.Date(2026, 10, 8, 21, 55, 0, 0, time.UTC)

const grace = 5 * time.Minute

func writeSealedKey(t *testing.T, l Locations, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(l.SealedKeyFile(), []byte(content), 0o600))
}

func sealedTestKey(t *testing.T, l Locations) Key {
	t.Helper()
	key, err := GenerateKey()
	require.NoError(t, err)
	writeSealedKey(t, l, "tpm2:"+key.Encode()+"\n")
	return key
}

func assertKeyUnchanged(t *testing.T, l Locations, content string) {
	t.Helper()
	got, err := os.ReadFile(l.SealedKeyFile())
	require.NoError(t, err)
	assert.Equal(t, content, string(got))
	setAside, err := filepath.Glob(l.SealedKeyFile() + ".unsealable-*")
	require.NoError(t, err)
	assert.Empty(t, setAside)
}

func TestCheckSealedKey_LeavesAKeyThatUnsealsAlone(t *testing.T) {
	newTPMStandIn(t)
	f := newKeyFixture(t)
	key := sealedTestKey(t, f.locations)

	check, err := CheckSealedKey(f.locations, checkedAt, grace)

	require.NoError(t, err)
	assert.Equal(t, SealKept, check.Outcome)
	assertKeyUnchanged(t, f.locations, "tpm2:"+key.Encode()+"\n")
}

func TestCheckSealedKey_KeepsAKeyThatUnsealsOnALaterTry(t *testing.T) {
	tpm := newTPMStandIn(t)
	tpm.set(t, "failing-unseals", "1")
	f := newKeyFixture(t)
	key := sealedTestKey(t, f.locations)

	check, err := CheckSealedKey(f.locations, checkedAt, grace)

	require.NoError(t, err)
	assert.Equal(t, SealKept, check.Outcome)
	assertKeyUnchanged(t, f.locations, "tpm2:"+key.Encode()+"\n")
}

func TestCheckSealedKey_SealsANewKeyWithTheTPMWhenAWorkingTPMRefusesTheOldOne(t *testing.T) {
	newTPMStandIn(t)
	f := newKeyFixture(t)
	writeSealedKey(t, f.locations, "sealed under a TPM state that is gone")

	check, err := CheckSealedKey(f.locations, checkedAt, grace)

	require.NoError(t, err)
	require.Equal(t, SealReplaced, check.Outcome)
	assert.ErrorContains(t, check.Reason, "State not recoverable")
	assert.Equal(t, "tpm2", check.Replacement.SealedWith)
	assert.Equal(t, f.locations.SealedKeyFile()+".unsealable-20261008T215500Z", check.Replacement.SetAside)
	old, err := os.ReadFile(check.Replacement.SetAside)
	require.NoError(t, err)
	assert.Equal(t, "sealed under a TPM state that is gone", string(old), "the old key is kept")
	info, err := os.Stat(f.locations.SealedKeyFile())
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	_, err = ShowKey(f.locations)
	assert.NoError(t, err, "the new sealed key unseals")
	_, err = os.Stat(f.locations.SealedKeyFile() + ".new")
	assert.ErrorIs(t, err, os.ErrNotExist)

	recorded, err := ReadKeyReplacement(f.locations)
	require.NoError(t, err)
	require.NotNil(t, recorded)
	assert.Equal(t, KeyReplacement{At: checkedAt, SealedWith: "tpm2", SetAside: check.Replacement.SetAside,
		Reason: check.Reason.Error()}, *recorded)
}

func TestCheckSealedKey_WaitsForAnUnusableTPMWithinTheGracePeriod(t *testing.T) {
	tpm := newTPMStandIn(t)
	tpm.removeTPM(t)
	f := newKeyFixture(t)
	writeSealedKey(t, f.locations, "sealed to a TPM that has not appeared yet")

	first, err := CheckSealedKey(f.locations, checkedAt, grace)
	require.NoError(t, err)
	later, err := CheckSealedKey(f.locations, checkedAt.Add(grace-time.Second), grace)
	require.NoError(t, err)

	assert.Equal(t, SealWaitingForTPM, first.Outcome)
	assert.Equal(t, SealWaitingForTPM, later.Outcome)
	assert.Equal(t, checkedAt, later.TPMUnusableSince, "the grace period runs from the first failure")
	assertKeyUnchanged(t, f.locations, "sealed to a TPM that has not appeared yet")
	recorded, err := ReadKeyReplacement(f.locations)
	require.NoError(t, err)
	assert.Nil(t, recorded)
}

func TestCheckSealedKey_SealsWithTheHostKeyOnceTheTPMStaysUnusablePastTheGracePeriod(t *testing.T) {
	tpm := newTPMStandIn(t)
	tpm.removeTPM(t)
	f := newKeyFixture(t)
	writeSealedKey(t, f.locations, "sealed to a TPM this host no longer has")
	_, err := CheckSealedKey(f.locations, checkedAt, grace)
	require.NoError(t, err)

	check, err := CheckSealedKey(f.locations, checkedAt.Add(grace), grace)

	require.NoError(t, err)
	require.Equal(t, SealReplaced, check.Outcome)
	assert.Equal(t, "host", check.Replacement.SealedWith)
	_, err = ShowKey(f.locations)
	assert.NoError(t, err)
	recorded, err := ReadKeyReplacement(f.locations)
	require.NoError(t, err)
	require.NotNil(t, recorded)
	assert.Equal(t, "host", recorded.SealedWith)
	_, err = os.Stat(filepath.Join(f.locations.KeyCheckState, tpmUnusableSinceFile))
	assert.ErrorIs(t, err, os.ErrNotExist, "the wait is over")
}

func TestCheckSealedKey_ForgetsTheWaitOnceTheTPMUnsealsAgain(t *testing.T) {
	tpm := newTPMStandIn(t)
	tpm.removeTPM(t)
	f := newKeyFixture(t)
	key, err := GenerateKey()
	require.NoError(t, err)
	tpm.set(t, "failing-unseals", "3")
	writeSealedKey(t, f.locations, "tpm2:"+key.Encode()+"\n")
	_, err = CheckSealedKey(f.locations, checkedAt, grace)
	require.NoError(t, err)
	require.NoError(t, os.Remove(filepath.Join(tpm.dir, "no-tpm")))

	check, err := CheckSealedKey(f.locations, checkedAt.Add(grace), grace)

	require.NoError(t, err)
	assert.Equal(t, SealKept, check.Outcome)
	_, err = os.Stat(filepath.Join(f.locations.KeyCheckState, tpmUnusableSinceFile))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestCheckSealedKey_LeavesTheKeyOfARunningHubAlone(t *testing.T) {
	tpm := newTPMStandIn(t)
	tpm.set(t, "hub-state", "active")
	f := newKeyFixture(t)
	writeSealedKey(t, f.locations, "sealed under a TPM state that is gone")

	check, err := CheckSealedKey(f.locations, checkedAt, grace)

	require.NoError(t, err)
	assert.Equal(t, SealSkipped, check.Outcome)
	assertKeyUnchanged(t, f.locations, "sealed under a TPM state that is gone")
}

func TestCheckSealedKey_ReplacesASealedKeyThatHoldsNoKey(t *testing.T) {
	newTPMStandIn(t)
	f := newKeyFixture(t)
	writeSealedKey(t, f.locations, "tpm2:not a key\n")

	check, err := CheckSealedKey(f.locations, checkedAt, grace)

	require.NoError(t, err)
	require.Equal(t, SealReplaced, check.Outcome)
	assert.ErrorContains(t, check.Reason, "does not hold a secret-store key")
}

func TestCheckSealedKey_ChangesNothingWhenSystemdCredsCannotRun(t *testing.T) {
	tpm := newTPMStandIn(t)
	require.NoError(t, os.Remove(filepath.Join(tpm.dir, "systemd-creds")))
	t.Setenv("PATH", tpm.dir)
	f := newKeyFixture(t)
	writeSealedKey(t, f.locations, "sealed")

	_, err := CheckSealedKey(f.locations, checkedAt, grace)

	require.Error(t, err)
	assertKeyUnchanged(t, f.locations, "sealed")
}

func TestCheckSealedKey_DoesNothingWithoutASealedKey(t *testing.T) {
	newTPMStandIn(t)
	f := newKeyFixture(t)

	check, err := CheckSealedKey(f.locations, checkedAt, grace)

	require.NoError(t, err)
	assert.Equal(t, SealKept, check.Outcome)
	_, err = os.Stat(f.locations.SealedKeyFile())
	assert.ErrorIs(t, err, os.ErrNotExist, "no key is created")
}
