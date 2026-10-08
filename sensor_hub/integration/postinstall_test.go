//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
)

// The package's postinstall runs here in a plain Debian container, against
// the real sensor-hub binary, with stand-ins for systemd. Installing the
// built packages under a real systemd is scripts/test-packages.sh.

const (
	keyFile      = "/etc/sensor-hub/secrets.key"
	sealedKey    = "/etc/sensor-hub/secrets.key.cred"
	keyDropIn    = "/etc/systemd/system/sensor-hub.service.d/secrets-key.conf"
	dropInDir    = "/etc/systemd/system/sensor-hub.service.d"
	packagedUnit = "/usr/lib/systemd/system/sensor-hub.service"
	defaultsDir  = "/usr/share/sensor-hub/defaults"
	sealedFake   = "SEALED:"
)

// fakeSystemctl records each call and which of the key files exist at that
// moment, so a test can see that the key came first.
const fakeSystemctl = `#!/bin/sh
present=""
for f in ` + keyFile + " " + sealedKey + " " + keyDropIn + `; do
  [ -e "$f" ] && present="$present $(basename "$f")"
done
echo "$*:$present" >> /var/log/systemctl.calls
`

// systemdCreds stands in for systemd-creds on a host with or without a TPM.
// Sealing prefixes SEALED: so the key can be checked through the seal.
func systemdCreds(hasTPM, sealFails bool) string {
	tpmStatus := "exit 1"
	if hasTPM {
		tpmStatus = "exit 0"
	}
	encrypt := `printf '` + sealedFake + `'; cat`
	if sealFails {
		encrypt = `echo "Failed to seal to the TPM" >&2; exit 1`
	}
	return `#!/bin/sh
echo "$*" >> /var/log/systemd-creds.calls
case "$1" in
has-tpm2) ` + tpmStatus + ` ;;
encrypt) ` + encrypt + ` ;;
decrypt) sed 's/^` + sealedFake + `//' "$3" ;;
esac
`
}

var (
	linuxBinaryOnce sync.Once
	linuxBinary     string
	linuxBinaryErr  error
)

// buildLinuxSensorHub builds sensor-hub for the architecture Docker runs.
func buildLinuxSensorHub(t *testing.T) string {
	t.Helper()
	linuxBinaryOnce.Do(func() {
		arch, err := exec.Command("docker", "info", "--format", "{{.Architecture}}").Output()
		if err != nil {
			linuxBinaryErr = fmt.Errorf("docker info: %w", err)
			return
		}
		goarch := map[string]string{"x86_64": "amd64", "aarch64": "arm64"}[strings.TrimSpace(string(arch))]
		if goarch == "" {
			linuxBinaryErr = fmt.Errorf("unsupported Docker architecture %q", arch)
			return
		}
		dir, err := os.MkdirTemp("", "sensor-hub-linux-*")
		if err != nil {
			linuxBinaryErr = err
			return
		}
		linuxBinary = filepath.Join(dir, "sensor-hub")
		build := exec.Command("go", "build", "-o", linuxBinary, "example/sensorHub")
		build.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+goarch, "CGO_ENABLED=0")
		if out, err := build.CombinedOutput(); err != nil {
			linuxBinaryErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	require.NoError(t, linuxBinaryErr)
	return linuxBinary
}

func cleanupLinuxSensorHub() {
	if linuxBinary != "" {
		os.RemoveAll(filepath.Dir(linuxBinary))
	}
}

func packagingDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "packaging")
}

type packageHost struct {
	t         *testing.T
	container testcontainers.Container
	upgrade   bool
}

// configFiles are the configuration files the package ships as templates.
var configFiles = map[string]string{
	"environment":            "environment",
	"application.properties": filepath.Join("defaults", "application.properties"),
	"database.properties":    filepath.Join("defaults", "database.properties"),
}

// operatorChange is a line an operator or the hub's properties saver might
// have added, which an upgrade must keep.
const operatorChange = "sensor.collection.interval=60\n"

// startPackageHost lays out a host as the package leaves it just before
// postinstall: the service account, the binary and the configuration
// templates. An upgrade also has the configuration a previous install left,
// with a change of the operator's in application.properties.
// systemdCredsScript is the systemd-creds on the path, or "" for none.
func startPackageHost(t *testing.T, systemdCredsScript string, upgrade bool) *packageHost {
	t.Helper()
	ctx := context.Background()
	packaging := packagingDir()
	files := []testcontainers.ContainerFile{
		{HostFilePath: buildLinuxSensorHub(t), ContainerFilePath: "/usr/bin/sensor-hub", FileMode: 0o755},
		{HostFilePath: filepath.Join(packaging, "scripts", "preinstall.sh"), ContainerFilePath: "/tmp/preinstall.sh", FileMode: 0o755},
		{HostFilePath: filepath.Join(packaging, "scripts", "postinstall.sh"), ContainerFilePath: "/tmp/postinstall.sh", FileMode: 0o755},
		{HostFilePath: filepath.Join(packaging, "sensor-hub.service"), ContainerFilePath: packagedUnit, FileMode: 0o644},
		{Reader: strings.NewReader(fakeSystemctl), ContainerFilePath: "/usr/local/bin/systemctl", FileMode: 0o755},
	}
	for name, source := range configFiles {
		files = append(files, testcontainers.ContainerFile{
			HostFilePath:      filepath.Join(packaging, source),
			ContainerFilePath: defaultsDir + "/" + name,
			FileMode:          0o644,
		})
	}
	if systemdCredsScript != "" {
		files = append(files, testcontainers.ContainerFile{
			Reader: strings.NewReader(systemdCredsScript), ContainerFilePath: "/usr/local/bin/systemd-creds", FileMode: 0o755,
		})
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "debian:bookworm-slim",
			Cmd:   []string{"sleep", "infinity"},
			Files: files,
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	h := &packageHost{t: t, container: container, upgrade: upgrade}
	h.mustRun("/tmp/preinstall.sh")
	if upgrade {
		h.mustRun("install -d -m 0755 /etc/sensor-hub && cp " + defaultsDir + "/* /etc/sensor-hub/" +
			" && chown sensor-hub:sensor-hub /etc/sensor-hub/* && chmod 0640 /etc/sensor-hub/*" +
			" && printf '" + operatorChange + "' >> /etc/sensor-hub/application.properties")
	}
	return h
}

func (h *packageHost) run(script string) (int, string) {
	h.t.Helper()
	code, reader, err := h.container.Exec(context.Background(), []string{"bash", "-c", script}, tcexec.Multiplexed())
	require.NoError(h.t, err)
	out, err := io.ReadAll(reader)
	require.NoError(h.t, err)
	return code, string(out)
}

func (h *packageHost) mustRun(script string) string {
	h.t.Helper()
	code, out := h.run(script)
	require.Equal(h.t, 0, code, "%s\n%s", script, out)
	return out
}

// postinstall runs the script with the arguments dpkg passes it: an upgrade
// names the version it replaces.
func (h *packageHost) postinstall() string {
	h.t.Helper()
	args := "configure"
	if h.upgrade {
		args = "configure 1.5.2"
	}
	return h.mustRun("/tmp/postinstall.sh " + args)
}

func (h *packageHost) exists(path string) bool {
	h.t.Helper()
	code, _ := h.run("test -e " + path)
	return code == 0
}

// modeAndOwner gives a file's mode and owner as "600 user:group".
func (h *packageHost) modeAndOwner(path string) string {
	h.t.Helper()
	return strings.TrimSpace(h.mustRun("stat -c '%a %U:%G' " + path))
}

func (h *packageHost) read(path string) string {
	h.t.Helper()
	return h.mustRun("cat " + path)
}

func (h *packageHost) systemctlCalls() string {
	h.t.Helper()
	return h.read("/var/log/systemctl.calls")
}

func TestPostinstall_WritesAKeyFileTheServiceCanReadWhereThereIsNoTPM(t *testing.T) {
	for name, tc := range map[string]struct {
		systemdCreds string
		upgrade      bool
		calls        string
	}{
		"upgrade, no systemd-creds": {
			upgrade: true,
			calls:   "daemon-reload: secrets.key\nrestart sensor-hub: secrets.key\n",
		},
		"install, systemd-creds without a TPM": {
			systemdCreds: systemdCreds(false, false),
			calls:        "daemon-reload: secrets.key\nenable sensor-hub: secrets.key\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := startPackageHost(t, tc.systemdCreds, tc.upgrade)

			out := h.postinstall()

			assert.Contains(t, out, "Wrote a new secret-store key to "+keyFile)
			assert.Contains(t, out, "sensor-hub local secrets show-key")
			assert.Equal(t, "600 sensor-hub:sensor-hub", h.modeAndOwner(keyFile))
			assert.False(t, h.exists(sealedKey))
			assert.False(t, h.exists(keyDropIn))
			assert.Equal(t, tc.calls, h.systemctlCalls(), "the key is written before the reload and the restart")

			key := h.mustRun("runuser -u sensor-hub -- sensor-hub local secrets show-key")
			assert.Equal(t, h.read(keyFile), key, "the service account reads the key")
			assert.Len(t, strings.TrimSpace(key), 44, "32 bytes of standard base64")
		})
	}
}

func TestPostinstall_SealsTheKeyWithTheTPM(t *testing.T) {
	t.Parallel()
	h := startPackageHost(t, systemdCreds(true, false), true)

	out := h.postinstall()

	assert.Contains(t, out, "Sealed a new secret-store key with the TPM in "+sealedKey)
	assert.Equal(t, "600 root:root", h.modeAndOwner(sealedKey))
	assert.Equal(t, "[Service]\nLoadCredentialEncrypted=secrets.key:"+sealedKey+"\n", h.read(keyDropIn))
	assert.False(t, h.exists(keyFile), "the plaintext key is never written")
	assert.Equal(t, "has-tpm2\nencrypt --with-key=tpm2 --name=secrets.key - -\n", h.read("/var/log/systemd-creds.calls"))
	assert.Equal(t,
		"daemon-reload: secrets.key.cred secrets-key.conf\nrestart sensor-hub: secrets.key.cred secrets-key.conf\n",
		h.systemctlCalls(), "the key and drop-in are written before the reload and the restart")
	packaged, err := os.ReadFile(filepath.Join(packagingDir(), "sensor-hub.service"))
	require.NoError(t, err)
	assert.Equal(t, string(packaged), h.read(packagedUnit), "the packaged unit is not edited")

	key := h.mustRun("sensor-hub local secrets show-key")
	assert.Equal(t, sealedFake+key, h.read(sealedKey), "show-key as root opens the sealed key")
}

func TestPostinstall_FallsBackToAKeyFileWhenSealingFails(t *testing.T) {
	t.Parallel()
	h := startPackageHost(t, systemdCreds(true, true), true)

	out := h.postinstall()

	assert.Contains(t, out, "Could not seal the secret-store key with the TPM")
	assert.Contains(t, out, "Failed to seal to the TPM")
	assert.Equal(t, "600 sensor-hub:sensor-hub", h.modeAndOwner(keyFile))
	assert.False(t, h.exists(sealedKey))
	assert.False(t, h.exists(keyDropIn))
	assert.Equal(t, "daemon-reload: secrets.key\nrestart sensor-hub: secrets.key\n", h.systemctlCalls())
}

func TestPostinstall_LeavesAnExistingKeyAlone(t *testing.T) {
	for name, existing := range map[string]string{
		"key file": "printf 'existing\\n' > " + keyFile + " && chmod 0600 " + keyFile +
			" && chown sensor-hub:sensor-hub " + keyFile,
		"sealed key": "printf '" + sealedFake + "existing\\n' > " + sealedKey + " && chmod 0600 " + sealedKey +
			" && mkdir -p " + dropInDir + " && printf '[Service]\\nLoadCredentialEncrypted=secrets.key:" + sealedKey + "\\n' > " + keyDropIn,
		"drop-in passing a key kept elsewhere": "mkdir -p " + dropInDir +
			" && printf '[Service]\\nLoadCredential=secrets.key:/root/secrets.key\\n' > " + dropInDir + "/override.conf",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := startPackageHost(t, systemdCreds(true, false), true)
			h.mustRun(existing)
			snapshot := "find /etc/sensor-hub " + filepath.Dir(dropInDir) + " -type f | sort | xargs md5sum"
			before := h.mustRun(snapshot)

			out := h.postinstall()
			again := h.postinstall()

			assert.Equal(t, before, h.mustRun(snapshot), "no key file, sealed key or drop-in is written or changed")
			assert.NotContains(t, out+again, "secret-store key", "nothing is said about a key that is already there")
			assert.NotContains(t, h.read("/var/log/systemd-creds.calls"), "encrypt")
			assert.Equal(t, strings.Repeat("daemon-reload:"+presentKeyFiles(h)+"\nrestart sensor-hub:"+presentKeyFiles(h)+"\n", 2),
				h.systemctlCalls(), "postinstall still reloads and restarts")
		})
	}
}

// presentKeyFiles gives the key files present, as fakeSystemctl lists them.
func presentKeyFiles(h *packageHost) string {
	present := ""
	for _, path := range []string{keyFile, sealedKey, keyDropIn} {
		if h.exists(path) {
			present += " " + filepath.Base(path)
		}
	}
	return present
}

func TestPostinstall_CreatesTheConfigurationFilesOnAFreshInstall(t *testing.T) {
	t.Parallel()
	h := startPackageHost(t, "", false)

	h.postinstall()

	for name, source := range configFiles {
		path := "/etc/sensor-hub/" + name
		assert.Equal(t, "640 sensor-hub:sensor-hub", h.modeAndOwner(path), path)
		shipped, err := os.ReadFile(filepath.Join(packagingDir(), source))
		require.NoError(t, err)
		assert.Equal(t, string(shipped), h.read(path), path)
	}
	assert.Equal(t, "no", strings.TrimSpace(h.mustRun("test -e /etc/sensor-hub/smtp.properties && echo yes || echo no")),
		"2.0 has no smtp.properties")
}

func TestPostinstall_LeavesTheConfigurationFilesOfAnUpgradeAlone(t *testing.T) {
	t.Parallel()
	h := startPackageHost(t, "", true)
	h.mustRun("rm /etc/sensor-hub/environment")
	// 1.5.x shipped smtp.properties too, and an upgrade leaves it in place.
	h.mustRun("printf 'smtp.user=alerts@example.com\n' > /etc/sensor-hub/smtp.properties" +
		" && chown sensor-hub:sensor-hub /etc/sensor-hub/smtp.properties && chmod 0640 /etc/sensor-hub/smtp.properties")
	kept := "cd /etc/sensor-hub && stat -c '%n %a %U:%G %Y' application.properties database.properties smtp.properties" +
		" && md5sum application.properties database.properties smtp.properties"
	// And the Gmail OAuth files, which the hub cannot delete from a directory
	// root owns.
	h.mustRun("cd /etc/sensor-hub && echo '{}' > credentials.json && echo '{}' > token.json")
	before := h.mustRun(kept)

	out := h.postinstall()

	assert.Equal(t, before, h.mustRun(kept), "files that exist are not changed")
	assert.Equal(t, "", strings.TrimSpace(h.mustRun("cd /etc/sensor-hub && ls credentials.json token.json 2>/dev/null || true")),
		"the Gmail OAuth files are deleted")
	assert.Contains(t, out, "Deleted /etc/sensor-hub/credentials.json")
	assert.Contains(t, out, "Deleted /etc/sensor-hub/token.json")
	assert.Contains(t, h.read("/etc/sensor-hub/application.properties"), operatorChange)
	assert.Equal(t, "640 sensor-hub:sensor-hub", h.modeAndOwner("/etc/sensor-hub/environment"), "a missing file is created")
}
