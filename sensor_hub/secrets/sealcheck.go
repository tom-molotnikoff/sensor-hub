package secrets

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// unsealAttempts and unsealRetryDelay give a TPM that is briefly busy a few
// chances before its key is treated as lost. Variables so tests run quickly.
var (
	unsealAttempts   = 3
	unsealRetryDelay = time.Second
)

// SealCheck is what CheckSealedKey found and did.
type SealCheck struct {
	// Replaced is true when the sealed key could not be used and a new one
	// was sealed in its place.
	Replaced bool
	// Reason is why the old key could not be used.
	Reason error
	// SetAside is where the old sealed key was kept, so it can still be
	// recovered should the TPM unseal it again.
	SetAside string
	// SealedWith is the systemd-creds key the new key was sealed with: tpm2,
	// or host when the TPM would not seal either.
	SealedWith string
}

// CheckSealedKey makes sure the TPM-sealed key still unseals before systemd
// starts the hub. systemd fails a unit whose credential will not decrypt
// before the hub ever runs, so a TPM that refuses the key, after a firmware or
// boot change, would stop the hub altogether. Instead the key that cannot be
// used is set aside and a new key sealed in its place: the hub starts, and
// every secret stored under the old key reports that it needs re-entry.
//
// It does nothing when there is no sealed key. It needs root. When
// systemd-creds cannot be run at all it changes nothing and returns the error,
// since that says nothing about the key.
func CheckSealedKey(l Locations, now time.Time) (SealCheck, error) {
	sealed := l.SealedKeyFile()
	if _, err := os.Stat(sealed); errors.Is(err, os.ErrNotExist) {
		return SealCheck{}, nil
	} else if err != nil {
		return SealCheck{}, fmt.Errorf("cannot check the sealed key %s: %w", sealed, err)
	}
	if !isRoot() {
		return SealCheck{}, fmt.Errorf("checking the sealed key %s needs root", sealed)
	}
	reason, err := unusableSealedKey(sealed)
	if err != nil || reason == nil {
		return SealCheck{}, err
	}
	setAside, sealedWith, err := replaceSealedKey(sealed, now)
	if err != nil {
		return SealCheck{}, fmt.Errorf("the sealed key %s cannot be used (%v), and replacing it failed: %w", sealed, reason, err)
	}
	return SealCheck{Replaced: true, Reason: reason, SetAside: setAside, SealedWith: sealedWith}, nil
}

// unusableSealedKey says why the sealed key cannot be used, or gives nil
// when it unseals to a key. The error is for a check that could not be made.
func unusableSealedKey(sealed string) (reason error, err error) {
	for attempt := 1; ; attempt++ {
		_, err := unsealKey(sealed)
		if err == nil {
			return nil, nil
		}
		var exitErr *exec.ExitError
		if errors.Is(err, errKeyNotUsable) {
			return err, nil
		}
		if !errors.As(err, &exitErr) {
			return nil, err
		}
		if attempt == unsealAttempts {
			return err, nil
		}
		time.Sleep(unsealRetryDelay)
	}
}

// replaceSealedKey seals a new key and swaps it in for the one at sealed,
// keeping the old one beside it. The sealed key exists at every moment, so
// the unit's credential always has a file to read.
func replaceSealedKey(sealed string, now time.Time) (setAside, sealedWith string, err error) {
	key, err := GenerateKey()
	if err != nil {
		return "", "", err
	}
	sealedWith = "tpm2"
	content, err := sealWith(key, sealedWith)
	if err != nil {
		// A TPM that has gone, or will not seal, still leaves the host's own
		// credential secret, which only root can read.
		sealedWith = "host"
		if content, err = sealWith(key, sealedWith); err != nil {
			return "", "", err
		}
	}
	replacement := sealed + ".new"
	_ = os.Remove(replacement) // left by an earlier attempt that stopped part way
	if err := writeNewFile(replacement, content, 0o600); err != nil {
		return "", "", fmt.Errorf("cannot write the new sealed key %s: %w", replacement, err)
	}
	setAside = sealed + ".unsealable-" + now.UTC().Format("20060102T150405Z")
	if err := os.Link(sealed, setAside); err != nil {
		_ = os.Remove(replacement)
		return "", "", fmt.Errorf("cannot keep the old sealed key as %s: %w", setAside, err)
	}
	if err := os.Rename(replacement, sealed); err != nil {
		_ = os.Remove(replacement)
		_ = os.Remove(setAside)
		return "", "", fmt.Errorf("cannot put the new sealed key in place at %s: %w", sealed, err)
	}
	return setAside, sealedWith, nil
}
