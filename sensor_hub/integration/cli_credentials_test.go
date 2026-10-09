//go:build integration

package integration

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loginUser creates a user to log in as, deleted when the test ends.
func loginUser(t *testing.T, username, password string) {
	t.Helper()
	resp, status := client.CreateUser(gen.CreateUserRequest{Username: username, Password: password})
	require.Equal(t, http.StatusCreated, status, string(resp))
	id := createdUserID(t, string(resp))
	t.Cleanup(func() { client.DeleteUser(id) })
}

func cliConfigPath(t *testing.T) string {
	return filepath.Join(cliHome(t), ".sensor-hub.yaml")
}

func TestAuthLoginCLI_ReadsThePasswordFromStdin(t *testing.T) {
	loginUser(t, "cli-login-stdin", "login-stdin-pass")
	hub := hubFlags(t)

	stdout, stderr, err := runSensorHub(t, "login-stdin-pass\n",
		withFlags(hub, "auth", "login", "--username", "cli-login-stdin", "--password-stdin")...)
	require.NoError(t, err, stderr)
	assert.Contains(t, stdout, "csrf_token", "the login response is printed")

	_, stderr, err = runSensorHub(t, "not-the-password\n",
		withFlags(hub, "auth", "login", "--username", "cli-login-stdin", "--password-stdin")...)
	require.Error(t, err)
	assert.Contains(t, stderr, "HTTP 401")
}

func TestAuthLoginCLI_PromptsOnceOnATerminal(t *testing.T) {
	loginUser(t, "cli-login-prompt", "login-prompt-pass")

	term := startOnTerminal(t, withFlags(hubFlags(t), "auth", "login", "--username", "cli-login-prompt")...)
	term.answer("Password: ", "login-prompt-pass")
	output, err := term.wait()

	require.NoError(t, err, output)
	assert.Contains(t, output, "csrf_token", "the login response is printed")
	assert.NotContains(t, output, "login-prompt-pass", "the prompt does not echo the password")
	assert.NotContains(t, output, "Confirm password", "a login asks for the password once")
}

func TestAuthLoginCLI_RefusesToPromptWithoutATerminal(t *testing.T) {
	_, stderr, err := runSensorHub(t, "piped-but-not-asked-for\n",
		withFlags(hubFlags(t), "auth", "login", "--username", "cli-login-no-terminal")...)

	require.Error(t, err)
	assert.Contains(t, stderr, "--password-stdin")
}

// A secret never travels as a flag value, where the process list and the
// shell history would show it.
func TestCLI_TakesNoSecretAsAFlag(t *testing.T) {
	for name, run := range map[string]struct {
		args []string
		flag string
	}{
		"login password":       {[]string{"auth", "login", "--username", "someone", "--password", "x"}, "--password"},
		"api key on a command": {[]string{"sensors", "list", "--api-key", "x"}, "--api-key"},
		"api key on health":    {[]string{"health", "--api-key", "x"}, "--api-key"},
	} {
		t.Run(name, func(t *testing.T) {
			_, stderr, err := runSensorHub(t, "", append(run.args, "--server", env.ServerURL)...)

			require.Error(t, err)
			assert.Contains(t, stderr, "unknown flag: "+run.flag)
		})
	}
}

func TestCLI_TakesTheAPIKeyFromTheEnvironment(t *testing.T) {
	key := adminAPIKey(t)

	t.Run("with --server no config file is needed", func(t *testing.T) {
		t.Setenv("SENSOR_HUB_API_KEY", key)
		require.NoFileExists(t, cliConfigPath(t))

		stdout, stderr, err := runSensorHub(t, "", "auth", "me", "--server", env.ServerURL)

		require.NoError(t, err, stderr)
		assert.Contains(t, stdout, env.AdminUser)
	})

	t.Run("it wins over the config file", func(t *testing.T) {
		config := "server: " + env.ServerURL + "\napi_key: shk_not_a_real_key\n"
		require.NoError(t, os.WriteFile(cliConfigPath(t), []byte(config), 0o600))

		t.Setenv("SENSOR_HUB_API_KEY", key)
		stdout, stderr, err := runSensorHub(t, "", "auth", "me")
		require.NoError(t, err, stderr)
		assert.Contains(t, stdout, env.AdminUser)

		t.Setenv("SENSOR_HUB_API_KEY", "")
		_, stderr, err = runSensorHub(t, "", "auth", "me")
		require.Error(t, err, "an empty SENSOR_HUB_API_KEY leaves the config file's key in use")
		assert.Contains(t, stderr, "HTTP 401")
	})
}

func TestConfigInitCLI_WritesTheConfigWithTheKeyFromStdin(t *testing.T) {
	key := adminAPIKey(t)
	t.Setenv("SENSOR_HUB_API_KEY", "")

	_, stderr, err := runSensorHub(t, key+"\n", "config", "init", "--api-key-stdin", "--server", env.ServerURL)
	require.NoError(t, err, stderr)

	info, err := os.Stat(cliConfigPath(t))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	written, err := os.ReadFile(cliConfigPath(t))
	require.NoError(t, err)
	assert.Contains(t, string(written), "api_key: "+key)

	stdout, stderr, err := runSensorHub(t, "", "auth", "me")
	require.NoError(t, err, stderr)
	assert.Contains(t, stdout, env.AdminUser)
}

func TestConfigInitCLI_WritesNothingWhenTheKeyCannotBeChecked(t *testing.T) {
	for name, run := range map[string]struct {
		stdin string
		args  []string
	}{
		"refused key":     {"shk_not_a_real_key\n", []string{"--server", env.ServerURL}},
		"hub unreachable": {"shk_not_a_real_key\n", []string{"--server", "http://127.0.0.1:1"}},
		"no --server":     {"shk_not_a_real_key\n", nil},
		"no key":          {"\n", []string{"--server", env.ServerURL}},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := runSensorHub(t, run.stdin, append([]string{"config", "init", "--api-key-stdin"}, run.args...)...)

			require.Error(t, err)
			assert.NoFileExists(t, cliConfigPath(t))
		})
	}
}

func TestConfigInitCLI_DoesNotEchoTheKeyOnATerminal(t *testing.T) {
	key := adminAPIKey(t)

	term := startOnTerminal(t, "config", "init")
	term.answer("Enter Sensor Hub server URL", env.ServerURL)
	term.answer("Enter API key", key)
	output, err := term.wait()

	require.NoError(t, err, output)
	assert.NotContains(t, output, key, "the prompt does not echo the API key")
	assert.Contains(t, output, "Authenticated as "+env.AdminUser)
	written, err := os.ReadFile(cliConfigPath(t))
	require.NoError(t, err)
	assert.Contains(t, string(written), "api_key: "+key)
}
