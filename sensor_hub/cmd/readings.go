package cmd

import (
	"github.com/spf13/cobra"

	gen "example/sensorHub/gen"
)

var readingsCmd = &cobra.Command{
	Use:   "readings",
	Short: "Query sensor readings",
}

func init() {
	readingsCmd.AddCommand(readingsBetweenCmd)
	rootCmd.AddCommand(readingsCmd)
}

var readingsBetweenCmd = &cobra.Command{
	Use:   "between",
	Short: "Get readings between two dates (auto-aggregated for large ranges)",
	RunE: func(cmd *cobra.Command, args []string) error {
		sensor, _ := cmd.Flags().GetString("sensor")
		measurementType, _ := cmd.Flags().GetString("type")
		start, _ := cmd.Flags().GetString("start")
		end, _ := cmd.Flags().GetString("end")
		aggregation, _ := cmd.Flags().GetString("aggregation")
		aggregationFn, _ := cmd.Flags().GetString("aggregation-function")

		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		params := &gen.GetReadingsBetweenDatesParams{
			Start: start,
			End:   end,
		}
		if sensor != "" {
			params.Sensor = &sensor
		}
		if measurementType != "" {
			params.Type = &measurementType
		}
		if aggregation != "" {
			a := gen.GetReadingsBetweenDatesParamsAggregation(aggregation)
			params.Aggregation = &a
		}
		if aggregationFn != "" {
			f := gen.GetReadingsBetweenDatesParamsAggregationFunction(aggregationFn)
			params.AggregationFunction = &f
		}
		return consumeJSON(client.GetReadingsBetweenDates(ctx, params))
	},
}

func init() {
	readingsBetweenCmd.Flags().String("sensor", "", "Sensor name")
	readingsBetweenCmd.Flags().String("type", "", "Measurement type (e.g. temperature, contact); required for --aggregation-function")
	readingsBetweenCmd.Flags().String("start", "", "Start date (YYYY-MM-DD) or datetime (ISO 8601, e.g. 2024-01-15T10:30:00Z)")
	readingsBetweenCmd.Flags().String("end", "", "End date (YYYY-MM-DD) or datetime (ISO 8601, e.g. 2024-01-15T11:30:00Z)")
	readingsBetweenCmd.Flags().String("aggregation", "", "Override aggregation interval (ISO 8601 duration, e.g. PT1H, PT5M)")
	readingsBetweenCmd.Flags().String("aggregation-function", "", "Override aggregation function (avg, count, last, min, max, increase); requires --type, see measurement-types list for what each type supports")
}
