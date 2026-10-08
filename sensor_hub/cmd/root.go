package cmd

import (
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
	Long:  "Sensor Hub — a home temperature monitoring system.\nRun the server with 'local serve' or use the other commands to interact with a hub.",
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
		os.Exit(1)
	}
}
