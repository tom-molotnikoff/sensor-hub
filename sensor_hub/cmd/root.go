package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var Version = "dev"

const (
	localGroupID = "local"
	hubGroupID   = "hub"
)

var rootCmd = &cobra.Command{
	Use:   "sensor-hub",
	Short: "Home temperature monitoring system",
	Long: "Sensor Hub - a home temperature monitoring system.\n" +
		"Run the server with 'local serve' or use the other commands to interact with a hub.\n\n" +
		"Commands that talk to a hub take its URL from --server, else from ~/.sensor-hub.yaml, and the API key from the " +
		apiKeyEnvVar + " environment variable, else from ~/.sensor-hub.yaml. Run 'sensor-hub config init' to write that file.",
}

func init() {
	rootCmd.AddGroup(
		&cobra.Group{ID: localGroupID, Title: "Commands that act on this machine's install:"},
		&cobra.Group{ID: hubGroupID, Title: "Commands that talk to a hub:"},
	)
	rootCmd.SetHelpCommandGroupID(hubGroupID)
	rootCmd.SetCompletionCommandGroupID(hubGroupID)
}

func Execute(version string) {
	Version = version
	rootCmd.Version = version
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		var coded exitCodeError
		if errors.As(err, &coded) {
			os.Exit(coded.code)
		}
		os.Exit(1)
	}
}

// exitCodeError ends the process with a status other than 1, for a failure a
// script needs to tell apart from the rest.
type exitCodeError struct {
	code int
	err  error
}

func (e exitCodeError) Error() string { return e.err.Error() }
func (e exitCodeError) Unwrap() error { return e.err }
