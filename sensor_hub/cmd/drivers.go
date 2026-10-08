package cmd

import (
	"github.com/spf13/cobra"
)

var driversCmd = &cobra.Command{
	Use:     "drivers",
	GroupID: hubGroupID,
	Short:   "Manage sensor drivers",
}

var driversListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available sensor drivers",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.ListDrivers(ctx, nil))
	},
}

func init() {
	rootCmd.AddCommand(driversCmd)
	driversCmd.AddCommand(driversListCmd)
}
