package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	gen "example/sensorHub/gen"
)

var measurementTypesCmd = &cobra.Command{
	Use:     "measurement-types",
	GroupID: hubGroupID,
	Short:   "Query measurement types",
}

func init() {
	measurementTypesCmd.AddCommand(measurementTypesListCmd)
	measurementTypesCmd.AddCommand(measurementTypesForSensorCmd)
	rootCmd.AddCommand(measurementTypesCmd)
}

var measurementTypesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all measurement types",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		params := &gen.GetAllMeasurementTypesParams{}
		if hasReadings, _ := cmd.Flags().GetBool("has-readings"); hasReadings {
			t := true
			params.HasReadings = &t
		}
		return consumeJSON(client.GetAllMeasurementTypes(ctx, params))
	},
}

var measurementTypesForSensorCmd = &cobra.Command{
	Use:   "for-sensor [sensor-id]",
	Short: "List measurement types supported by a sensor",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("sensor ID must be a number")
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetSensorMeasurementTypes(ctx, id))
	},
}

func init() {
	measurementTypesListCmd.Flags().Bool("has-readings", false, "Only return types that have at least one reading")
}
