package cmd

import (
	"errors"
	"fmt"
	"io"
	"time"

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

var localSecretsCheckSealCmd = &cobra.Command{
	Use:   "check-seal",
	Short: "Replace a sealed secret-store key the TPM will not unseal",
	Long: "Checks that the key sealed with the TPM in <config-dir>/secrets.key.cred still unseals. systemd will not start a " +
		"unit whose credential cannot be decrypted, so a TPM that refuses the key after a firmware or boot change would " +
		"keep the hub from starting at all. When the key cannot be unsealed, after a few tries, it is kept as " +
		"secrets.key.cred.unsealable-<time> and a new key is sealed in its place, with the TPM or, failing that, the " +
		"host's own credential secret. The hub then starts, and every secret stored under the old key has to be " +
		"entered again.\n\n" +
		"The package runs this as root before each start of the service, through sensor-hub-key-check.service. It does " +
		"nothing when the key is not sealed, and changes nothing when systemd-creds cannot be run.",
	Args: cobra.NoArgs,
	RunE: runLocalSecretsCheckSeal,
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
	localSecretsCmd.AddCommand(localSecretsInitKeyCmd, localSecretsShowKeyCmd, localSecretsCheckSealCmd)
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

func runLocalSecretsCheckSeal(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	l := secrets.LocationsFor(localConfigDir, "")
	check, err := secrets.CheckSealedKey(l, time.Now())
	if err != nil {
		return err
	}
	if !check.Replaced {
		return nil
	}
	out := cmd.ErrOrStderr()
	fmt.Fprintf(out, "The sealed secret-store key %s could not be unsealed: %v\n", l.SealedKeyFile(), check.Reason)
	fmt.Fprintf(out, "Kept it as %s and sealed a new key in its place with --with-key=%s.\n", check.SetAside, check.SealedWith)
	fmt.Fprintln(out, "The hub will start, and every secret stored under the old key has to be entered again.")
	fmt.Fprintln(out, "Keep a copy of the new key with 'sensor-hub local secrets show-key'.")
	return nil
}
