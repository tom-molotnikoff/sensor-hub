package secrets

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// isRoot reports whether the process runs as root. A variable so tests can
// exercise the root-only paths.
var isRoot = func() bool { return os.Geteuid() == 0 }

// existingKeySources are every form a key may already take, including a
// sealed credential the hub only sees through systemd.
func (l Locations) existingKeySources() []string {
	var paths []string
	if l.CredentialsDir != "" {
		paths = append(paths, filepath.Join(l.CredentialsDir, CredentialName))
	}
	paths = append(paths, l.SealedKeyFile())
	if l.ComposeSecret != "" {
		paths = append(paths, l.ComposeSecret)
	}
	if l.KeyFile != "" {
		paths = append(paths, l.KeyFile)
	}
	return append(paths, l.ConfigKeyFile())
}

// ErrKeyExists is what InitKey's refusal wraps when a key already exists.
var ErrKeyExists = errors.New("a secret-store key already exists")

// existingKey describes a key that already exists in any form, including a
// unit drop-in that passes the hub one, or gives "" when there is none.
func (l Locations) existingKey() (string, error) {
	for _, path := range l.existingKeySources() {
		_, err := os.Stat(path)
		if err == nil {
			return path, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("cannot check for a secret-store key at %s: %w", path, err)
		}
	}
	return l.dropInPassingAKey()
}

// InitKey makes key the hub's key and returns where it went. Sealed, it is
// encrypted with the TPM through systemd-creds and a unit drop-in has systemd
// hand it to the hub; this needs root. Otherwise it is written to the
// configuration directory with mode 0600, owned like application.properties.
// It refuses with ErrKeyExists when a key already exists in any form.
func InitKey(l Locations, key Key, seal bool) (string, error) {
	existing, err := l.existingKey()
	if err != nil {
		return "", err
	}
	if existing != "" {
		return "", fmt.Errorf("%w at %s; refusing to replace it, since secrets stored under it could no longer be decrypted", ErrKeyExists, existing)
	}
	if seal {
		return l.SealedKeyFile(), sealKey(l, key)
	}
	path := l.ConfigKeyFile()
	return path, writeKeyFile(path, key, filepath.Join(l.ConfigDir, "application.properties"))
}

func sealKey(l Locations, key Key) error {
	if !isRoot() {
		return errors.New("sealing the key with the TPM needs root")
	}
	sealed, err := sealKeyWith(key, "tpm2")
	if err != nil {
		return err
	}
	credPath, err := filepath.Abs(l.SealedKeyFile())
	if err != nil {
		return err
	}
	if err := writeNewFile(credPath, sealed, 0o600); err != nil {
		return fmt.Errorf("cannot write the sealed key %s: %w", credPath, err)
	}
	if err := writeDropIn(l.SystemdDropIn, credPath); err != nil {
		// A sealed key that systemd is never told about would stop the hub
		// starting, so it goes too.
		_ = os.Remove(credPath)
		return fmt.Errorf("cannot write the systemd drop-in %s: %w", l.SystemdDropIn, err)
	}
	return nil
}

// sealKeyWith encrypts key as the secrets.key credential with the given
// systemd-creds key: tpm2, or host for the host's own credential secret.
func sealKeyWith(key Key, withKey string) ([]byte, error) {
	return runSystemdCreds([]byte(key.Encode()+"\n"), "encrypt", "--with-key="+withKey, "--name="+CredentialName, "-", "-")
}

func writeDropIn(path, credPath string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	dropIn := fmt.Sprintf("[Service]\nLoadCredentialEncrypted=%s:%s\n", CredentialName, credPath)
	return os.WriteFile(path, []byte(dropIn), 0o644)
}

// ShowKey finds the key the hub would use, as LoadKey does, and also opens a
// TPM-sealed key, which needs root. It never generates one.
func ShowKey(l Locations) (Key, error) {
	if l.CredentialsDir != "" {
		if key, found, err := readKeyIfPresent(filepath.Join(l.CredentialsDir, CredentialName)); found {
			return key, err
		}
	}
	sealed := l.SealedKeyFile()
	if _, err := os.Stat(sealed); err == nil {
		return unsealKey(sealed)
	}
	for _, path := range []string{l.ComposeSecret, l.KeyFile, l.ConfigKeyFile()} {
		if path == "" {
			continue
		}
		if key, found, err := readKeyIfPresent(path); found {
			return key, err
		}
	}
	return Key{}, fmt.Errorf("no secret-store key found; the hub generates one at %s when it first starts, or create one with 'sensor-hub local secrets init-key'", l.ConfigKeyFile())
}

func readKeyIfPresent(path string) (Key, bool, error) {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return Key{}, false, nil
	}
	key, err := readKeyFile(path)
	return key, true, err
}

func unsealKey(path string) (Key, error) {
	if !isRoot() {
		return Key{}, fmt.Errorf("the secret-store key is sealed with the TPM in %s; reading it needs root", path)
	}
	plain, err := runSystemdCreds(nil, "decrypt", "--name="+CredentialName, path, "-")
	if err != nil {
		return Key{}, err
	}
	key, err := ParseKey(plain)
	if err != nil {
		return Key{}, fmt.Errorf("the sealed key %s is not usable: %w: %w", path, errKeyNotUsable, err)
	}
	return key, nil
}

// errKeyNotUsable marks a sealed key that unsealed to something other than
// a key.
var errKeyNotUsable = errors.New("it does not hold a secret-store key")

func runSystemdCreds(stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.Command("systemd-creds", args...)
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("systemd-creds %s failed: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func writeNewFile(path string, content []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}
