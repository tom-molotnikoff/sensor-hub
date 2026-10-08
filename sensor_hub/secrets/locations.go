package secrets

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const (
	// CredentialName is the name of the systemd credential holding the key,
	// loaded into $CREDENTIALS_DIRECTORY.
	CredentialName = "secrets.key"
	// ComposeSecretPath is where Compose mounts a secret named
	// sensor-hub-secrets-key.
	ComposeSecretPath = "/run/secrets/sensor-hub-secrets-key"
	// SystemdDropInPath is the drop-in that has systemd decrypt the sealed key
	// for the packaged unit.
	SystemdDropInPath = "/etc/systemd/system/sensor-hub.service.d/secrets-key.conf"

	keyFileName    = "secrets.key"
	sealedFileName = "secrets.key.cred"
)

// Locations are the places a key may be. The hub reads the first that holds
// one, in the order of the fields.
type Locations struct {
	// CredentialsDir is $CREDENTIALS_DIRECTORY, where systemd puts decrypted
	// credentials for the unit. Empty when not run by systemd with one.
	CredentialsDir string
	// ComposeSecret is the path a Compose secret is mounted at.
	ComposeSecret string
	// KeyFile is the --secrets-key-file flag. Empty when not given.
	KeyFile string
	// ConfigDir holds secrets.key, and secrets.key.cred when the key is
	// sealed with the TPM.
	ConfigDir string
	// SystemdDropIn is where sealing writes the unit drop-in.
	SystemdDropIn string
}

// LocationsFor gives the locations for a hub run with the given configuration
// directory and --secrets-key-file flag.
func LocationsFor(configDir, keyFile string) Locations {
	return Locations{
		CredentialsDir: os.Getenv("CREDENTIALS_DIRECTORY"),
		ComposeSecret:  ComposeSecretPath,
		KeyFile:        keyFile,
		ConfigDir:      configDir,
		SystemdDropIn:  SystemdDropInPath,
	}
}

// ConfigKeyFile is where the hub writes a key it generates.
func (l Locations) ConfigKeyFile() string { return filepath.Join(l.ConfigDir, keyFileName) }

// SealedKeyFile is where a TPM-sealed key is kept.
func (l Locations) SealedKeyFile() string { return filepath.Join(l.ConfigDir, sealedFileName) }

// keySource is one place a key may be.
type keySource struct {
	path        string
	description string
	// guarded sources must pass the file rules: not readable by others and
	// not inside the database's directory. A systemd credential and a Compose
	// secret are placed by their manager and exempt.
	guarded bool
	// required sources were named by the operator, so their absence is an
	// error rather than a reason to look further.
	required bool
}

func (l Locations) hubSources() []keySource {
	var sources []keySource
	if l.CredentialsDir != "" {
		sources = append(sources, keySource{path: filepath.Join(l.CredentialsDir, CredentialName), description: "systemd credential"})
	}
	if l.ComposeSecret != "" {
		sources = append(sources, keySource{path: l.ComposeSecret, description: "Compose secret"})
	}
	if l.KeyFile != "" {
		sources = append(sources, keySource{path: l.KeyFile, description: "--secrets-key-file", guarded: true, required: true})
	}
	return append(sources, keySource{path: l.ConfigKeyFile(), description: "configuration directory", guarded: true})
}

// LoadKey finds the hub's key and checks the file holding it. When there is
// none anywhere it generates one into the configuration directory. A key file
// readable by others or inside the directory holding the database is refused
// with an error naming the rule.
func LoadKey(l Locations, databasePath string, logger *slog.Logger) (Key, error) {
	for _, source := range l.hubSources() {
		info, err := os.Stat(source.path)
		if errors.Is(err, fs.ErrNotExist) && !source.required {
			continue
		}
		if err != nil {
			return Key{}, fmt.Errorf("cannot read the secret-store key from the %s %s: %w", source.description, source.path, err)
		}
		if source.guarded {
			if err := checkKeyFile(source.path, info, databasePath); err != nil {
				return Key{}, err
			}
		}
		key, err := readKeyFile(source.path)
		if err != nil {
			return Key{}, err
		}
		logger.Info("loaded the secret-store key", "from", source.description, "path", source.path)
		return key, nil
	}
	return generateKeyFile(l, databasePath, logger)
}

func generateKeyFile(l Locations, databasePath string, logger *slog.Logger) (Key, error) {
	// A sealed key only reaches the hub through systemd. Generating a second
	// key beside it would leave secrets written now unreadable under the unit.
	if _, err := os.Stat(l.SealedKeyFile()); err == nil {
		return Key{}, fmt.Errorf("the secret-store key is sealed in %s but systemd did not pass it to this process; start the hub through its systemd unit, or pass the key with --secrets-key-file", l.SealedKeyFile())
	}
	path := l.ConfigKeyFile()
	if err := checkOutsideDatabaseDir(path, databasePath); err != nil {
		return Key{}, err
	}
	key, err := GenerateKey()
	if err != nil {
		return Key{}, err
	}
	if err := writeKeyFile(path, key, ""); err != nil {
		return Key{}, err
	}
	logger.Warn("no secret-store key was found, so a new one was generated; keep a copy with 'sensor-hub local secrets show-key'", "path", path)
	return key, nil
}

func checkKeyFile(path string, info fs.FileInfo, databasePath string) error {
	if info.Mode().Perm()&0o004 != 0 {
		return fmt.Errorf("refusing the secret-store key %s: it is readable by others (mode %04o); chmod 600 it", path, info.Mode().Perm())
	}
	return checkOutsideDatabaseDir(path, databasePath)
}

// checkOutsideDatabaseDir refuses a key path inside the directory holding
// the database, where a copy of the database directory would carry the key
// along with the secrets it decrypts.
func checkOutsideDatabaseDir(keyPath, databasePath string) error {
	databaseDir := resolvePath(filepath.Dir(databasePath))
	keyDir := resolvePath(filepath.Dir(keyPath))
	rel, err := filepath.Rel(databaseDir, keyDir)
	if err != nil {
		return fmt.Errorf("cannot compare the secret-store key %s with the database directory %s: %w", keyPath, databaseDir, err)
	}
	if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return fmt.Errorf("refusing the secret-store key %s: it is inside %s, the directory holding the database; keep it elsewhere so a copy of the database never carries its key", keyPath, databaseDir)
	}
	return nil
}

// resolvePath makes a path absolute and follows symlinks where it exists, so
// two spellings of one directory compare equal.
func resolvePath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}

func readKeyFile(path string) (Key, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Key{}, fmt.Errorf("cannot read the secret-store key %s: %w", path, err)
	}
	key, err := ParseKey(content)
	if err != nil {
		return Key{}, fmt.Errorf("the secret-store key %s is not usable: %w", path, err)
	}
	return key, nil
}

// writeKeyFile writes a new key file with mode 0600, never replacing one. Run
// as root, it gives the file the owner of ownerReference, so the service can
// read a key written for it by root.
func writeKeyFile(path string, key Key, ownerReference string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("cannot write the secret-store key %s: %w", path, err)
	}
	if err := fillKeyFile(f, key, ownerReference); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return fmt.Errorf("cannot write the secret-store key %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("cannot write the secret-store key %s: %w", path, err)
	}
	return nil
}

func fillKeyFile(f *os.File, key Key, ownerReference string) error {
	if err := chownLikeIfRoot(f, ownerReference); err != nil {
		return err
	}
	if _, err := f.WriteString(key.Encode() + "\n"); err != nil {
		return err
	}
	return f.Sync()
}
