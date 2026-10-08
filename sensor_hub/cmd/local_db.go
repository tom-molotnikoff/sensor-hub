package cmd

import (
	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	"fmt"

	"github.com/spf13/cobra"
)

var localDbCmd = &cobra.Command{
	Use:   "db",
	Short: "Work with this install's database",
}

var localDbBackupCmd = &cobra.Command{
	Use:   "backup <path>",
	Short: "Write a consistent copy of the live database to a new file",
	Long: "Copies the database at database.path to <path> with VACUUM INTO while the server keeps running.\n" +
		"The file is created with mode 0600. An existing <path> is never overwritten.",
	Args: cobra.ExactArgs(1),
	RunE: runLocalDbBackup,
}

func init() {
	localDbCmd.AddCommand(localDbBackupCmd)
	localCmd.AddCommand(localDbCmd)
}

func runLocalDbBackup(cmd *cobra.Command, args []string) error {
	useWarningLogger(cmd)
	if err := appProps.InitialiseConfig(localConfigDir); err != nil {
		return fmt.Errorf("failed to initialise application configuration: %w", err)
	}
	return database.Backup(cmd.Context(), appProps.AppConfig().DatabasePath, args[0])
}
