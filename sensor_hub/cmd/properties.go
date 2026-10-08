package cmd

import (
	gen "example/sensorHub/gen"

	"github.com/spf13/cobra"
)

var propertiesCmd = &cobra.Command{
	Use:     "properties",
	GroupID: hubGroupID,
	Short:   "Manage application properties",
}

func init() {
	propertiesCmd.AddCommand(propertiesGetCmd)
	propertiesCmd.AddCommand(propertiesSetCmd)
	rootCmd.AddCommand(propertiesCmd)
}

var propertiesGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get all application properties",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetProperties(ctx))
	},
}

var propertiesSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Update application properties",
	RunE: func(cmd *cobra.Command, args []string) error {
		key, _ := cmd.Flags().GetString("key")
		value, _ := cmd.Flags().GetString("value")

		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		body := gen.UpdatePropertiesJSONRequestBody{key: value}
		return consumeJSON(client.UpdateProperties(ctx, body))
	},
}

func init() {
	propertiesSetCmd.Flags().String("key", "", "Property key")
	propertiesSetCmd.Flags().String("value", "", "Property value")
}
