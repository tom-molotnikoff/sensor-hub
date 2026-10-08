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

// passwordInputHelp is the help text for a command that takes a new password
// through --password-stdin or a prompt.
const passwordInputHelp = "The password is never taken as a flag value. With --password-stdin it is read from stdin " +
	"(one line, trailing newline stripped), so a script can pipe it in without it landing in the process list. " +
	"Otherwise it is prompted for twice on the terminal, and a mismatch is rejected."

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
	var password string
	var err error
	if fromStdin {
		password, err = readPasswordLine(cmd.InOrStdin())
	} else {
		fd, ok := stdinTerminal(cmd)
		if !ok {
			return "", errors.New("stdin is not a terminal, so the password cannot be prompted for; pipe it in with --password-stdin")
		}
		password, err = promptNewPassword(cmd, fd)
	}
	if err != nil {
		return "", err
	}
	if password == "" {
		return "", errors.New("password must not be empty")
	}
	return password, nil
}

func readPasswordLine(in io.Reader) (string, error) {
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("failed to read password from stdin: %w", err)
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}

func promptNewPassword(cmd *cobra.Command, fd int) (string, error) {
	prompt := func(label string) (string, error) {
		fmt.Fprint(cmd.ErrOrStderr(), label)
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(cmd.ErrOrStderr())
		if err != nil {
			return "", fmt.Errorf("failed to read password: %w", err)
		}
		return string(b), nil
	}
	first, err := prompt("Password: ")
	if err != nil {
		return "", err
	}
	second, err := prompt("Confirm password: ")
	if err != nil {
		return "", err
	}
	if first != second {
		return "", errors.New("passwords do not match")
	}
	return first, nil
}
