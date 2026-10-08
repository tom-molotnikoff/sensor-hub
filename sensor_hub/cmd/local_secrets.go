package cmd

import (
	"errors"
	"fmt"
	"io"

	"example/sensorHub/secrets"

	"github.com/spf13/cobra"
)

var localSecretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage this install's secret-store key",
	Long: "The secret store holds the credentials the hub presents to other systems, such as outbound MQTT broker passwords, " +
		"encrypted under a key kept outside the database.",
}

var localSecretsInitKeyCmd = &cobra.Command{
	Use:   "init-key",
	Short: "Create the secret-store key",
	Long: "Generates a key and writes it to <config-dir>/secrets.key with mode 0600. Run as root, the file takes the owner of " +
		"application.properties, so the service can read it.\n\n" +
		"With --from-stdin the key is read from stdin instead: one line of standard base64 encoding 32 bytes, as show-key prints it.\n\n" +
		"With --seal the key is sealed with the TPM by systemd-creds into <config-dir>/secrets.key.cred, and a drop-in at " +
		secrets.SystemdDropInPath + " has systemd decrypt it for the hub. This needs root; run 'systemctl daemon-reload' and " +
		"restart the service afterwards.\n\n" +
		"Refuses when a key already exists in any form, naming it, and exits with status 3. A drop-in for the unit that passes " +
		"the hub a key counts. There is no way to overwrite a key: secrets stored under it could no longer be decrypted.",
	Args: cobra.NoArgs,
	RunE: runLocalSecretsInitKey,
}

var localSecretsShowKeyCmd = &cobra.Command{
	Use:   "show-key",
	Short: "Print the secret-store key",
	Long: "Prints the key the hub uses as one line of base64 on stdout and nothing else, so it can be piped into a password " +
		"manager. Keep a copy: without the key, stored secrets cannot be decrypted and have to be entered again.\n\n" +
		"A key sealed with the TPM is decrypted with systemd-creds, which needs root.",
	Args: cobra.NoArgs,
	RunE: runLocalSecretsShowKey,
}

// keyExistsExitCode is init-key's exit status when a key already exists, so
// the package's postinstall can tell that from a failure.
const keyExistsExitCode = 3

var initKeyFromStdin bool
var initKeySeal bool
var showKeySecretsKeyFile string

func init() {
	localSecretsInitKeyCmd.Flags().BoolVar(&initKeyFromStdin, "from-stdin", false, "Read the key from stdin rather than generating one")
	localSecretsInitKeyCmd.Flags().BoolVar(&initKeySeal, "seal", false, "Seal the key with the TPM through systemd-creds (root only)")
	localSecretsShowKeyCmd.Flags().StringVar(&showKeySecretsKeyFile, "secrets-key-file", "", "The --secrets-key-file the hub is run with, if any")
	localSecretsCmd.AddCommand(localSecretsInitKeyCmd, localSecretsShowKeyCmd)
	localCmd.AddCommand(localSecretsCmd)
}

func runLocalSecretsInitKey(cmd *cobra.Command, args []string) error {
	// The package's postinstall shows what init-key says when it fails. A
	// failure from here on is not a usage mistake, and Execute prints it once.
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	key, err := initialKey(cmd)
	if err != nil {
		return err
	}
	path, err := secrets.InitKey(secrets.LocationsFor(localConfigDir, ""), key, initKeySeal)
	if errors.Is(err, secrets.ErrKeyExists) {
		return exitCodeError{code: keyExistsExitCode, err: err}
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Wrote the secret-store key to %s.\n", path)
	if initKeySeal {
		fmt.Fprintf(cmd.ErrOrStderr(), "Wrote %s. Run 'systemctl daemon-reload' and restart sensor-hub to use it.\n", secrets.SystemdDropInPath)
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "Keep a copy of it with 'sensor-hub local secrets show-key'.")
	return nil
}

func initialKey(cmd *cobra.Command) (secrets.Key, error) {
	if !initKeyFromStdin {
		return secrets.GenerateKey()
	}
	text, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 1024))
	if err != nil {
		return secrets.Key{}, fmt.Errorf("failed to read the key from stdin: %w", err)
	}
	key, err := secrets.ParseKey(text)
	if err != nil {
		return secrets.Key{}, fmt.Errorf("the key on stdin is not usable: %w", err)
	}
	return key, nil
}

func runLocalSecretsShowKey(cmd *cobra.Command, args []string) error {
	key, err := secrets.ShowKey(secrets.LocationsFor(localConfigDir, showKeySecretsKeyFile))
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), key.Encode())
	return nil
}
