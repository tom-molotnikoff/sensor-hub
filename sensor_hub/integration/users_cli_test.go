//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"example/sensorHub/testharness"

	"github.com/creack/pty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hubFlags points CLI commands at the test hub with an admin API key made
// for the test.
func hubFlags(t *testing.T) []string {
	t.Helper()
	key, keyID := createApiKey(t, client, "cli-"+t.Name())
	t.Cleanup(func() { client.DeleteApiKey(keyID) })
	return []string{"--server", env.ServerURL, "--api-key", key}
}

func withFlags(flags []string, args ...string) []string {
	return append(args, flags...)
}

func createdUserID(t *testing.T, stdout string) int {
	t.Helper()
	var created struct {
		ID int `json:"id"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &created), stdout)
	require.NotZero(t, created.ID)
	return created.ID
}

func loginStatus(username, password string) int {
	return testharness.NewClient(nil, env.ServerURL).Login(username, password)
}

func TestUsersCLI_CreateAndChangePasswordReadThePasswordFromStdin(t *testing.T) {
	hub := hubFlags(t)
	stdout, stderr, err := runSensorHub(t, "stdin-first-pass\n",
		withFlags(hub, "users", "create", "--username", "cli-stdin-user", "--password-stdin")...)
	require.NoError(t, err, stderr)
	id := createdUserID(t, stdout)
	t.Cleanup(func() { client.DeleteUser(id) })
	assert.Equal(t, http.StatusOK, loginStatus("cli-stdin-user", "stdin-first-pass"))

	_, stderr, err = runSensorHub(t, "stdin-second-pass\n",
		withFlags(hub, "users", "change-password", "--user-id", strconv.Itoa(id), "--password-stdin")...)
	require.NoError(t, err, stderr)
	assert.Equal(t, http.StatusOK, loginStatus("cli-stdin-user", "stdin-second-pass"))
	assert.Equal(t, http.StatusUnauthorized, loginStatus("cli-stdin-user", "stdin-first-pass"))
}

func TestUsersCLI_RefusesToPromptWithoutATerminal(t *testing.T) {
	_, stderr, err := runSensorHub(t, "piped-but-not-asked-for\n",
		withFlags(hubFlags(t), "users", "create", "--username", "cli-no-terminal-user")...)

	require.Error(t, err)
	assert.Contains(t, stderr, "--password-stdin")
	assert.NotContains(t, listedUsernames(t), "cli-no-terminal-user")
}

func TestUsersCLI_PromptsTwiceOnATerminalAndRejectsAMismatch(t *testing.T) {
	hub := hubFlags(t)
	term := startOnTerminal(t, withFlags(hub, "users", "create", "--username", "cli-prompt-user")...)
	term.answer("Password: ", "prompt-first-pass")
	term.answer("Confirm password: ", "prompt-first-pass")
	output, err := term.wait()
	require.NoError(t, err, output)
	id := createdUserID(t, output[strings.Index(output, "{"):])
	t.Cleanup(func() { client.DeleteUser(id) })
	assert.Equal(t, http.StatusOK, loginStatus("cli-prompt-user", "prompt-first-pass"))
	assert.NotContains(t, output, "prompt-first-pass", "the prompt does not echo the password")

	term = startOnTerminal(t, withFlags(hub, "users", "change-password", "--user-id", strconv.Itoa(id))...)
	term.answer("Password: ", "prompt-second-pass")
	term.answer("Confirm password: ", "prompt-typo-pass")
	output, err = term.wait()
	require.Error(t, err)
	assert.Contains(t, output, "passwords do not match")
	assert.Equal(t, http.StatusOK, loginStatus("cli-prompt-user", "prompt-first-pass"))
}

func listedUsernames(t *testing.T) string {
	t.Helper()
	resp, status := client.ListUsers()
	require.Equal(t, http.StatusOK, status)
	return string(resp)
}

// terminalSession runs the CLI with a pseudo-terminal as its stdin, stdout and
// stderr, as a person at a shell would.
type terminalSession struct {
	t    *testing.T
	cmd  *exec.Cmd
	tty  io.WriteCloser
	mu   sync.Mutex
	out  bytes.Buffer
	read sync.WaitGroup
}

func startOnTerminal(t *testing.T, args ...string) *terminalSession {
	t.Helper()
	s := &terminalSession{t: t, cmd: exec.Command(buildSensorHub(t), args...)}
	tty, err := pty.Start(s.cmd)
	require.NoError(t, err)
	s.tty = tty
	t.Cleanup(func() {
		_ = s.cmd.Process.Kill()
		tty.Close()
	})
	s.read.Add(1)
	go func() {
		defer s.read.Done()
		buf := make([]byte, 1024)
		for {
			n, err := tty.Read(buf)
			s.mu.Lock()
			s.out.Write(buf[:n])
			s.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	return s
}

// answer waits for the prompt to appear after the output already answered,
// then types the line.
func (s *terminalSession) answer(prompt, line string) {
	s.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		s.mu.Lock()
		output := s.out.String()
		found := strings.Contains(output, prompt)
		if found {
			s.out.Reset()
		}
		s.mu.Unlock()
		if found {
			break
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("prompt %q never appeared; output so far: %q", prompt, output)
		}
		time.Sleep(20 * time.Millisecond)
	}
	_, err := io.WriteString(s.tty, line+"\n")
	require.NoError(s.t, err)
}

// wait returns everything written to the terminal after the last answer and
// the command's exit error.
func (s *terminalSession) wait() (string, error) {
	err := s.cmd.Wait()
	done := make(chan struct{})
	go func() { s.read.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		s.tty.Close()
		<-done
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		s.t.Fatalf("command did not run: %v", err)
	}
	return s.out.String(), err
}
