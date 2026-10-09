package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const passwordStdinHelp = "The password is never taken as a flag value. With --password-stdin it is read from stdin " +
	"(one line, trailing newline stripped), so a script can pipe it in without it landing in the process list. "

// passwordInputHelp is the help text for a command that takes a new password
// through --password-stdin or a prompt.
const passwordInputHelp = passwordStdinHelp +
	"Otherwise it is prompted for twice on the terminal, and a mismatch is rejected."

// existingPasswordInputHelp is the help text for a command that takes a
// password the user already has, such as at login, through --password-stdin or
// a prompt.
const existingPasswordInputHelp = passwordStdinHelp + "Otherwise it is prompted for once on the terminal."

// stdinTerminal returns the file descriptor of the command's stdin when it is
// a terminal.
func stdinTerminal(cmd *cobra.Command) (int, bool) {
	f, ok := cmd.InOrStdin().(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return 0, false
	}
	return int(f.Fd()), true
}

func stdinIsTerminal(cmd *cobra.Command) bool {
	_, ok := stdinTerminal(cmd)
	return ok
}

// readNewPassword reads a new password from stdin when fromStdin is set, and
// otherwise prompts for it twice on the terminal, rejecting a mismatch. It
// refuses an empty password.
func readNewPassword(cmd *cobra.Command, fromStdin bool) (string, error) {
	return requirePassword(readPassword(cmd, fromStdin, "Password: ", true))
}

// readExistingPassword reads a password the user already has, such as one to
// log in with, from stdin when fromStdin is set, and otherwise prompts for it
// once on the terminal. There is nothing to confirm, since it is checked
// against the one already set. It refuses an empty password.
func readExistingPassword(cmd *cobra.Command, fromStdin bool) (string, error) {
	return requirePassword(readPassword(cmd, fromStdin, "Password: ", false))
}

// readOptionalPassword reads a password that may be left empty, such as one
// the hub presents to an outbound broker. An empty line on stdin, or an empty
// entry at the prompt, means no password; a typed one is confirmed.
func readOptionalPassword(cmd *cobra.Command, fromStdin bool) (string, error) {
	return readPassword(cmd, fromStdin, "Password (leave empty for none): ", true)
}

func requirePassword(password string, err error) (string, error) {
	if err != nil {
		return "", err
	}
	if password == "" {
		return "", errors.New("password must not be empty")
	}
	return password, nil
}

func readPassword(cmd *cobra.Command, fromStdin bool, label string, confirm bool) (string, error) {
	if fromStdin {
		return readSecretLine(cmd.InOrStdin())
	}
	fd, ok := stdinTerminal(cmd)
	if !ok {
		return "", errors.New("stdin is not a terminal, so the password cannot be prompted for; pipe it in with --password-stdin")
	}
	first, err := promptSecret(cmd, fd, label)
	if err != nil || first == "" || !confirm {
		return first, err
	}
	second, err := promptSecret(cmd, fd, "Confirm password: ")
	if err != nil {
		return "", err
	}
	if first != second {
		return "", errors.New("passwords do not match")
	}
	return first, nil
}

// readSecretLine reads one line, such as a password or an API key piped in by
// a script, with its trailing newline stripped.
func readSecretLine(in io.Reader) (string, error) {
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("failed to read from stdin: %w", err)
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}

// promptSecret shows label on stderr and reads a line from the terminal on fd
// without echoing it.
func promptSecret(cmd *cobra.Command, fd int, label string) (string, error) {
	fmt.Fprint(cmd.ErrOrStderr(), label)
	b, err := term.ReadPassword(fd)
	fmt.Fprintln(cmd.ErrOrStderr())
	if err != nil {
		return "", fmt.Errorf("failed to read from the terminal: %w", err)
	}
	return string(b), nil
}
