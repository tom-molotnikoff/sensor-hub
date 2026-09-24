package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	gen "example/sensorHub/gen"
)

var alertsCmd = &cobra.Command{
	Use:   "alerts",
	Short: "Manage alert rules",
}

func init() {
	alertsCmd.AddCommand(alertsListCmd)
	alertsCmd.AddCommand(alertsGetCmd)
	alertsCmd.AddCommand(alertsCreateCmd)
	alertsCmd.AddCommand(alertsUpdateCmd)
	alertsCmd.AddCommand(alertsDeleteCmd)
	alertsCmd.AddCommand(alertsHistoryCmd)
	rootCmd.AddCommand(alertsCmd)
}

var alertsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all alert rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetAllAlertRules(ctx))
	},
}

var alertsGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get an alert rule by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("alert ID must be a number")
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetAlertRuleById(ctx, id))
	},
}

var alertsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new alert rule",
	RunE: func(cmd *cobra.Command, args []string) error {
		sensorID, _ := cmd.Flags().GetInt("sensor-id")
		measurementTypeID, _ := cmd.Flags().GetInt("measurement-type-id")
		alertType, _ := cmd.Flags().GetString("type")
		threshold, _ := cmd.Flags().GetFloat64("threshold")

		if sensorID == 0 || alertType == "" || measurementTypeID == 0 {
			return fmt.Errorf("--sensor-id, --measurement-type-id, and --type are required")
		}

		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		body := gen.CreateAlertRuleJSONRequestBody{
			SensorId:          sensorID,
			MeasurementTypeId: measurementTypeID,
			AlertType:         gen.AlertRuleAlertType(alertType),
			HighThreshold:     threshold,
		}
		return consumeJSON(client.CreateAlertRule(ctx, body))
	},
}

func init() {
	alertsCreateCmd.Flags().Int("sensor-id", 0, "Sensor ID")
	alertsCreateCmd.Flags().Int("measurement-type-id", 0, "Measurement type ID (e.g. 1 for temperature)")
	alertsCreateCmd.Flags().String("type", "", "Alert type")
	alertsCreateCmd.Flags().Float64("threshold", 0, "Threshold value")
}

var alertsDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete an alert rule by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("alert ID must be a number")
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.DeleteAlertRule(ctx, id))
	},
}

var alertsHistoryCmd = &cobra.Command{
	Use:   "history [sensorId]",
	Short: "Get alert history for a sensor",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sensorID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("sensor ID must be a number")
		}

		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		params := &gen.GetAlertHistoryParams{}
		if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
			params.Limit = &limit
		}
		return consumeJSON(client.GetAlertHistory(ctx, sensorID, params))
	},
}

func init() {
	alertsHistoryCmd.Flags().Int("limit", 0, "Maximum number of history records (1-100, default 50)")
}

var alertsUpdateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "Update an alert rule by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("alert ID must be a number")
		}
		alertType, _ := cmd.Flags().GetString("alert-type")
		highThreshold, _ := cmd.Flags().GetFloat64("high-threshold")
		lowThreshold, _ := cmd.Flags().GetFloat64("low-threshold")
		enabled, _ := cmd.Flags().GetBool("enabled")
		rateLimitSeconds, _ := cmd.Flags().GetInt("rate-limit-seconds")

		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		body := gen.UpdateAlertRuleJSONRequestBody{
			AlertType:        gen.AlertRuleAlertType(alertType),
			HighThreshold:    highThreshold,
			LowThreshold:     lowThreshold,
			Enabled:          enabled,
			RateLimitSeconds: rateLimitSeconds,
		}
		return consumeJSON(client.UpdateAlertRule(ctx, id, body))
	},
}

func init() {
	alertsUpdateCmd.Flags().String("alert-type", "", "Alert type")
	alertsUpdateCmd.Flags().Float64("high-threshold", 0, "High threshold")
	alertsUpdateCmd.Flags().Float64("low-threshold", 0, "Low threshold")
	alertsUpdateCmd.Flags().Bool("enabled", true, "Whether the alert is enabled")
	alertsUpdateCmd.Flags().Int("rate-limit-seconds", 0, "Rate limit in seconds (e.g. 3600 = 1 hour)")
}
