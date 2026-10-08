package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var localConfigDir string

var localCmd = &cobra.Command{
	Use:               "local",
	GroupID:           localGroupID,
	Short:             "Act on this machine's Sensor Hub install",
	Long:              "Commands that run on the machine where Sensor Hub is installed and read its configuration directory directly.",
	PersistentPreRunE: requireConfigFiles,
}

func init() {
	localCmd.PersistentFlags().StringVar(&localConfigDir, "config-dir", "/etc/sensor-hub", "Path to configuration directory")
	rootCmd.AddCommand(localCmd)
}

func requireConfigFiles(cmd *cobra.Command, args []string) error {
	for _, name := range []string{"application.properties", "database.properties"} {
		_, err := os.Stat(filepath.Join(localConfigDir, name))
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("configuration directory %s has no %s", localConfigDir, name)
		}
		if err != nil {
			return fmt.Errorf("configuration directory %s: %w", localConfigDir, err)
		}
	}
	return nil
}

// The server's info-level startup logging, such as the configuration dump,
// would bury the output of a one-shot command.
func useWarningLogger(cmd *cobra.Command) *slog.Logger {
	logger := slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), &slog.HandlerOptions{Level: slog.LevelWarn}))
	slog.SetDefault(logger)
	return logger
}
