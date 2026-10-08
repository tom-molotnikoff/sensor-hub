package secrets

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// credentialDirectives are the [Service] settings that hand a unit a
// credential, each naming the credential before any colon. ImportCredential
// names it with a glob.
var credentialDirectives = map[string]bool{
	"LoadCredential":          true,
	"LoadCredentialEncrypted": true,
	"SetCredential":           true,
	"SetCredentialEncrypted":  true,
	"ImportCredential":        true,
}

// dropInPassingAKey looks through the unit's drop-in directory for a drop-in
// that already hands the hub a key, either as the secrets.key credential or
// with --secrets-key-file on its command line. It describes the first found,
// or gives "" when there is none.
func (l Locations) dropInPassingAKey() (string, error) {
	if l.SystemdDropIn == "" {
		return "", nil
	}
	dir := filepath.Dir(l.SystemdDropIn)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("cannot check the systemd drop-ins in %s for a secret-store key: %w", dir, err)
	}
	var names []string // ReadDir sorts them, so the first found is stable
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".conf") {
			names = append(names, entry.Name())
		}
	}
	for _, name := range names {
		dropIn := filepath.Join(dir, name)
		content, err := os.ReadFile(dropIn)
		if err != nil {
			return "", fmt.Errorf("cannot check the systemd drop-in %s for a secret-store key: %w", dropIn, err)
		}
		if setting := keySetting(string(content)); setting != "" {
			return fmt.Sprintf("%s, a systemd drop-in that passes the hub a key with %s", dropIn, setting), nil
		}
	}
	return "", nil
}

// keySetting gives the first [Service] setting in a unit file that passes the
// hub a key, or "" when none does.
func keySetting(unit string) string {
	inService := false
	for _, line := range unitLines(unit) {
		if strings.HasPrefix(line, "[") {
			inService = line == "[Service]"
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		if !inService || !ok {
			continue
		}
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		if credentialDirectives[name] {
			id, _, _ := strings.Cut(value, ":")
			if matched, _ := path.Match(id, CredentialName); matched {
				return line
			}
		}
		if name == "ExecStart" && strings.Contains(value, "--secrets-key-file") {
			return line
		}
	}
	return ""
}

// unitLines gives a unit file's lines with comments and blank lines dropped
// and backslash continuations joined.
func unitLines(unit string) []string {
	var lines []string
	var pending strings.Builder
	for _, raw := range strings.Split(unit, "\n") {
		line := strings.TrimSpace(raw)
		if pending.Len() == 0 && (line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";")) {
			continue
		}
		if strings.HasSuffix(line, "\\") {
			pending.WriteString(strings.TrimSuffix(line, "\\") + " ")
			continue
		}
		pending.WriteString(line)
		lines = append(lines, pending.String())
		pending.Reset()
	}
	if pending.Len() > 0 {
		lines = append(lines, pending.String())
	}
	return lines
}
