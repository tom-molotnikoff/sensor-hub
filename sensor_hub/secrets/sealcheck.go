package secrets

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DefaultTPMGracePeriod is how long the key check waits, from the first
// failure this boot, for a TPM that cannot be used before it gives up on the
// sealed key. A TPM that is late at boot gets the time to appear, and the hub
// is down for at most about this long.
const DefaultTPMGracePeriod = 5 * time.Minute

// HubUnit is the systemd unit the key check runs before.
const HubUnit = "sensor-hub.service"

// unsealAttempts and unsealRetryDelay give a TPM that is briefly busy a few
// chances before its answer is taken. Variables so tests run quickly.
var (
	unsealAttempts   = 3
	unsealRetryDelay = time.Second
)

const (
	tpmUnusableSinceFile = "tpm-unusable-since"
	keyReplacedFile      = "key-replaced"
)

// SealOutcome is what CheckSealedKey did.
type SealOutcome int

const (
	// SealKept means the key unsealed, or there is no sealed key.
	SealKept SealOutcome = iota
	// SealSkipped means the hub is running, so its key was left alone.
	SealSkipped
	// SealWaitingForTPM means the TPM cannot be used and the grace period has
	// not run out, so the key was left alone for now.
	SealWaitingForTPM
	// SealReplaced means a new key was sealed in place of one that could not
	// be used.
	SealReplaced
)

// SealCheck is what CheckSealedKey found and did.
type SealCheck struct {
	Outcome SealOutcome
	// Reason is why the sealed key could not be unsealed.
	Reason error
	// TPMUnusableSince is when this boot first found the TPM unusable, while
	// waiting for it.
	TPMUnusableSince time.Time
	// Replacement describes the new key, when one was sealed.
	Replacement *KeyReplacement
}

// KeyReplacement records a sealed key the check replaced, for the hub to
// mention when it reports the secrets that need re-entry.
type KeyReplacement struct {
	At time.Time
	// SealedWith is the systemd-creds key the new key is sealed with: tpm2,
	// or host when the TPM could not be used.
	SealedWith string
	// SetAside is where the old sealed key is kept, so it can be put back
	// should the TPM unseal it again.
	SetAside string
	Reason   string
}

// CheckSealedKey makes sure the TPM-sealed key unseals before systemd starts
// the hub. systemd fails a unit whose credential will not decrypt before the
// hub ever runs, so a sealed key that cannot be used would stop the hub
// altogether. When the TPM works but refuses the key, after a firmware or boot
// change, the key is set aside and a new one sealed with the TPM in its place:
// the hub starts, and every secret stored under the old key reports that it
// needs re-entry. When the TPM cannot be used at all, it may only be late or
// briefly unreachable, so the key is left alone until the TPM has stayed
// unusable for the grace period this boot; the hub's unit fails and restarts
// in the meantime, running this check again. After that the new key is sealed
// with the host's own credential secret.
//
// It does nothing when there is no sealed key, and leaves the key of a running
// hub alone. It needs root. When systemd-creds or systemctl cannot be run at
// all it changes nothing and returns the error, since that says nothing about
// the key.
func CheckSealedKey(l Locations, now time.Time, grace time.Duration) (SealCheck, error) {
	sealed := l.SealedKeyFile()
	if _, err := os.Stat(sealed); errors.Is(err, os.ErrNotExist) {
		return SealCheck{}, nil
	} else if err != nil {
		return SealCheck{}, fmt.Errorf("cannot check the sealed key %s: %w", sealed, err)
	}
	if !isRoot() {
		return SealCheck{}, fmt.Errorf("checking the sealed key %s needs root", sealed)
	}
	running, err := hubRunning()
	if err != nil {
		return SealCheck{}, err
	}
	if running {
		return SealCheck{Outcome: SealSkipped}, nil
	}
	reason, err := unusableSealedKey(sealed)
	if err != nil {
		return SealCheck{}, err
	}
	if reason == nil {
		return SealCheck{}, l.clearKeyCheckState(tpmUnusableSinceFile)
	}

	sealWith := []string{"tpm2", "host"}
	if !errors.Is(reason, errKeyNotUsable) && !tpmUsable() {
		since, err := l.tpmUnusableSince(now)
		if err != nil {
			return SealCheck{}, err
		}
		if now.Sub(since) < grace {
			return SealCheck{Outcome: SealWaitingForTPM, Reason: reason, TPMUnusableSince: since}, nil
		}
		sealWith = []string{"host"}
	}

	replacement, err := replaceSealedKey(sealed, now, sealWith)
	if err != nil {
		return SealCheck{}, fmt.Errorf("the sealed key %s cannot be used (%v), and replacing it failed: %w", sealed, reason, err)
	}
	replacement.Reason = reason.Error()
	if err := l.recordKeyReplacement(replacement); err != nil {
		return SealCheck{}, err
	}
	return SealCheck{Outcome: SealReplaced, Reason: reason, Replacement: &replacement}, nil
}

// hubRunning reports whether the hub's unit is up, in which case it already
// holds its key and a check could only take that key from under it.
func hubRunning() (bool, error) {
	out, err := exec.Command("systemctl", "is-active", HubUnit).Output()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return false, fmt.Errorf("cannot tell whether %s is running: %w", HubUnit, err)
	}
	state := strings.TrimSpace(string(out))
	return state == "active" || state == "reloading", nil
}

// tpmUsable reports whether systemd-creds finds a TPM it can use.
func tpmUsable() bool {
	return exec.Command("systemd-creds", "has-tpm2").Run() == nil
}

// unusableSealedKey says why the sealed key cannot be used, or gives nil
// when it unseals to a key. The error is for a check that could not be made.
func unusableSealedKey(sealed string) (reason error, err error) {
	for attempt := 1; ; attempt++ {
		_, err := unsealKey(sealed)
		if err == nil {
			return nil, nil
		}
		if errors.Is(err, errKeyNotUsable) {
			return err, nil
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return nil, err
		}
		if attempt == unsealAttempts {
			return err, nil
		}
		time.Sleep(unsealRetryDelay)
	}
}

// replaceSealedKey seals a new key with the first of sealWith that works and
// swaps it in for the one at sealed, keeping the old one beside it. The
// sealed key exists at every moment, so the unit's credential always has a
// file to read.
func replaceSealedKey(sealed string, now time.Time, sealWith []string) (KeyReplacement, error) {
	key, err := GenerateKey()
	if err != nil {
		return KeyReplacement{}, err
	}
	var content []byte
	var used string
	for _, with := range sealWith {
		if content, err = sealKeyWith(key, with); err == nil {
			used = with
			break
		}
	}
	if err != nil {
		return KeyReplacement{}, err
	}
	replacement := sealed + ".new"
	_ = os.Remove(replacement) // left by an earlier attempt that stopped part way
	if err := writeNewFile(replacement, content, 0o600); err != nil {
		return KeyReplacement{}, fmt.Errorf("cannot write the new sealed key %s: %w", replacement, err)
	}
	setAside := sealed + ".unsealable-" + now.UTC().Format("20060102T150405Z")
	if err := os.Link(sealed, setAside); err != nil {
		_ = os.Remove(replacement)
		return KeyReplacement{}, fmt.Errorf("cannot keep the old sealed key as %s: %w", setAside, err)
	}
	if err := os.Rename(replacement, sealed); err != nil {
		_ = os.Remove(replacement)
		_ = os.Remove(setAside)
		return KeyReplacement{}, fmt.Errorf("cannot put the new sealed key in place at %s: %w", sealed, err)
	}
	return KeyReplacement{At: now, SealedWith: used, SetAside: setAside}, nil
}

// tpmUnusableSince gives when this boot first found the TPM unusable,
// recording now when this is the first time.
func (l Locations) tpmUnusableSince(now time.Time) (time.Time, error) {
	path := filepath.Join(l.KeyCheckState, tpmUnusableSinceFile)
	content, err := os.ReadFile(path)
	if err == nil {
		if since, err := time.Parse(time.RFC3339, strings.TrimSpace(string(content))); err == nil {
			return since, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return time.Time{}, fmt.Errorf("cannot read %s: %w", path, err)
	}
	if err := l.writeKeyCheckState(tpmUnusableSinceFile, now.UTC().Format(time.RFC3339)+"\n"); err != nil {
		return time.Time{}, err
	}
	return now, nil
}

func (l Locations) recordKeyReplacement(r KeyReplacement) error {
	if err := l.clearKeyCheckState(tpmUnusableSinceFile); err != nil {
		return err
	}
	oneLine := strings.NewReplacer("\n", " ", "\r", " ")
	record := fmt.Sprintf("at=%s\nsealed_with=%s\nset_aside=%s\nreason=%s\n",
		r.At.UTC().Format(time.RFC3339), r.SealedWith, oneLine.Replace(r.SetAside), oneLine.Replace(r.Reason))
	return l.writeKeyCheckState(keyReplacedFile, record)
}

// ReadKeyReplacement gives the key the check replaced this boot, or nil when
// it replaced none.
func ReadKeyReplacement(l Locations) (*KeyReplacement, error) {
	if l.KeyCheckState == "" {
		return nil, nil
	}
	path := filepath.Join(l.KeyCheckState, keyReplacedFile)
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	var r KeyReplacement
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		name, value, _ := strings.Cut(scanner.Text(), "=")
		switch name {
		case "at":
			r.At, _ = time.Parse(time.RFC3339, value)
		case "sealed_with":
			r.SealedWith = value
		case "set_aside":
			r.SetAside = value
		case "reason":
			r.Reason = value
		}
	}
	return &r, nil
}

// The state directory and its files say nothing secret, and the hub, which
// runs unprivileged, reads the replacement record.
func (l Locations) writeKeyCheckState(name, content string) error {
	if l.KeyCheckState == "" {
		return errors.New("there is nowhere to keep the key check's state")
	}
	if err := os.MkdirAll(l.KeyCheckState, 0o755); err != nil {
		return fmt.Errorf("cannot create %s: %w", l.KeyCheckState, err)
	}
	path := filepath.Join(l.KeyCheckState, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("cannot write %s: %w", path, err)
	}
	return nil
}

func (l Locations) clearKeyCheckState(name string) error {
	if l.KeyCheckState == "" {
		return nil
	}
	err := os.Remove(filepath.Join(l.KeyCheckState, name))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
