package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	gen "example/sensorHub/gen"
)

var sensorsCmd = &cobra.Command{
	Use:   "sensors",
	Short: "Manage sensors",
}

func init() {
	sensorsCmd.AddCommand(sensorsListCmd)
	sensorsCmd.AddCommand(sensorsGetCmd)
	sensorsCmd.AddCommand(sensorsAddCmd)
	sensorsCmd.AddCommand(sensorsUpdateCmd)
	sensorsCmd.AddCommand(sensorsDeleteCmd)
	sensorsCmd.AddCommand(sensorsExistsCmd)
	sensorsCmd.AddCommand(sensorsListByDriverCmd)
	sensorsCmd.AddCommand(sensorsEnableCmd)
	sensorsCmd.AddCommand(sensorsDisableCmd)
	sensorsCmd.AddCommand(sensorsHealthCmd)
	sensorsCmd.AddCommand(sensorsStatsCmd)
	sensorsCmd.AddCommand(sensorsCollectCmd)
	sensorsCmd.AddCommand(sensorsCapabilitiesCmd)
	sensorsCmd.AddCommand(sensorsCommandCmd)
	sensorsCmd.AddCommand(sensorsCommandsCmd)
	sensorsCmd.AddCommand(sensorsPendingCmd)
	sensorsCmd.AddCommand(sensorsApproveCmd)
	sensorsCmd.AddCommand(sensorsDismissCmd)
	rootCmd.AddCommand(sensorsCmd)
}

var sensorsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active sensors (use --status to see pending, dismissed or all)",
	RunE: func(cmd *cobra.Command, args []string) error {
		status, _ := cmd.Flags().GetString("status")
		if status != "all" && !gen.GetSensorsByStatusParamsStatus(status).Valid() {
			return fmt.Errorf("invalid --status %q: must be active, pending, dismissed or all", status)
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		if status == "all" {
			return consumeJSON(client.GetAllSensors(ctx))
		}
		return consumeJSON(client.GetSensorsByStatus(ctx, gen.GetSensorsByStatusParamsStatus(status)))
	},
}

func init() {
	sensorsListCmd.Flags().String("status", "active", "Lifecycle status to list: active, pending, dismissed or all")
}

var sensorsGetCmd = &cobra.Command{
	Use:   "get [name]",
	Short: "Get a sensor by name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetSensorByName(ctx, args[0]))
	},
}

// sensorBody builds a partial JSON body matching the historical CLI wire
// format ({name, sensor_driver, config}) for AddSensor / UpdateSensorById.
// We use the *WithBody* generated client variants so we don't need to
// populate every field of `gen.Sensor`, several of which are server-managed
// (id, status, health_*).
func sensorBody(name, driver string, config map[string]string) map[string]any {
	return map[string]any{
		"name":          name,
		"sensor_driver": driver,
		"config":        config,
	}
}

var sensorsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new sensor",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		driver, _ := cmd.Flags().GetString("driver")
		configPairs, _ := cmd.Flags().GetStringSlice("config")
		if name == "" || driver == "" {
			return fmt.Errorf("--name and --driver are required")
		}
		config, err := parseKVPairs(configPairs)
		if err != nil {
			return err
		}

		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		body, err := rawJSONReader(sensorBody(name, driver, config))
		if err != nil {
			return err
		}
		return consumeJSON(client.AddSensorWithBody(ctx, "application/json", body))
	},
}

func init() {
	sensorsAddCmd.Flags().String("name", "", "Sensor name")
	sensorsAddCmd.Flags().String("driver", "", "Sensor driver (e.g. sensor-hub-http-temperature)")
	sensorsAddCmd.Flags().StringSlice("config", nil, "Config key=value pairs (repeatable, e.g. --config url=http://...)")
}

var sensorsDeleteCmd = &cobra.Command{
	Use:   "delete [name]",
	Short: "Delete a sensor",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.DeleteSensorByName(ctx, args[0]))
	},
}

var sensorsEnableCmd = &cobra.Command{
	Use:   "enable [name]",
	Short: "Enable a sensor",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.EnableSensor(ctx, args[0]))
	},
}

var sensorsDisableCmd = &cobra.Command{
	Use:   "disable [name]",
	Short: "Disable a sensor",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.DisableSensor(ctx, args[0]))
	},
}

var sensorsHealthCmd = &cobra.Command{
	Use:   "health [name]",
	Short: "Get sensor health status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetSensorHealthHistoryByName(ctx, args[0]))
	},
}

var sensorsStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Get total readings per sensor",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetTotalReadingsPerSensor(ctx))
	},
}

var sensorsCollectCmd = &cobra.Command{
	Use:   "collect [name]",
	Short: "Trigger sensor data collection (all or specific sensor)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		if len(args) > 0 {
			return consumeJSON(client.CollectFromSensor(ctx, args[0]))
		}
		return consumeJSON(client.CollectAllSensorReadings(ctx))
	},
}

var sensorsCapabilitiesCmd = &cobra.Command{
	Use:   "capabilities [id]",
	Short: "List controllable capabilities for a sensor ID",
	Long:  "List controllable capabilities for a sensor ID, including allowed values/range and the latest reading for each property.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseSensorIDArg(args[0])
		if err != nil {
			return err
		}

		client, ctx, cfg, err := newAPIClientWithResponses(cmd)
		if err != nil {
			return err
		}

		sensor, err := lookupSensorByID(ctx, client, id)
		if err != nil {
			return err
		}

		capResp, err := client.GetSensorCapabilitiesWithResponse(ctx, id)
		if err != nil {
			return err
		}
		if capResp.StatusCode() != 200 || capResp.JSON200 == nil {
			return apiResponseError(capResp.StatusCode(), capResp.Body)
		}

		readings, err := currentReadingsSnapshot(ctx, cfg)
		if err != nil {
			return err
		}

		writeCapabilityTable(cmd, *capResp.JSON200, latestReadingsByProperty(sensor.Name, readings))
		return nil
	},
}

var sensorsCommandCmd = &cobra.Command{
	Use:   "command [id] [property] [value]",
	Short: "Send a command to a controllable sensor by ID",
	Long:  "Send a command to a controllable sensor by ID and print the accepted command record.",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseSensorIDArg(args[0])
		if err != nil {
			return err
		}

		client, ctx, _, err := newAPIClientWithResponses(cmd)
		if err != nil {
			return err
		}

		response, err := client.SendSensorCommandWithResponse(ctx, id, gen.SendSensorCommandJSONRequestBody{
			Property: args[1],
			Value:    args[2],
		})
		if err != nil {
			return err
		}
		if response.StatusCode() != 202 || response.JSON202 == nil {
			return apiResponseError(response.StatusCode(), response.Body)
		}

		writeAcceptedCommandTable(cmd, *response.JSON202)
		return nil
	},
}

var sensorsCommandsCmd = &cobra.Command{
	Use:   "commands [id]",
	Short: "List recent command history for a sensor ID",
	Long:  "List the most recent command history entries for a sensor ID.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseSensorIDArg(args[0])
		if err != nil {
			return err
		}

		client, ctx, _, err := newAPIClientWithResponses(cmd)
		if err != nil {
			return err
		}

		response, err := client.GetSensorCommandHistoryWithResponse(ctx, id)
		if err != nil {
			return err
		}
		if response.StatusCode() != 200 || response.JSON200 == nil {
			return apiResponseError(response.StatusCode(), response.Body)
		}

		history := *response.JSON200
		if len(history) > 20 {
			history = history[:20]
		}
		writeCommandHistoryTable(cmd, history)
		return nil
	},
}

var sensorsUpdateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "Update an existing sensor",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("sensor ID must be a number")
		}
		name, _ := cmd.Flags().GetString("name")
		driver, _ := cmd.Flags().GetString("driver")
		configPairs, _ := cmd.Flags().GetStringSlice("config")
		config, err := parseKVPairs(configPairs)
		if err != nil {
			return err
		}

		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		body, err := rawJSONReader(sensorBody(name, driver, config))
		if err != nil {
			return err
		}
		return consumeJSON(client.UpdateSensorByIdWithBody(ctx, id, "application/json", body))
	},
}

func init() {
	sensorsUpdateCmd.Flags().String("name", "", "Sensor name")
	sensorsUpdateCmd.Flags().String("driver", "", "Sensor driver")
	sensorsUpdateCmd.Flags().StringSlice("config", nil, "Config key=value pairs (repeatable)")
}

var sensorsExistsCmd = &cobra.Command{
	Use:   "exists [name]",
	Short: "Check if a sensor exists",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		status, err := consumeStatus(client.SensorExists(ctx, args[0]))
		if err != nil {
			return err
		}
		if status == 200 {
			fmt.Println("Sensor exists")
		} else {
			fmt.Println("Sensor not found")
		}
		return nil
	},
}

var sensorsListByDriverCmd = &cobra.Command{
	Use:   "list-by-driver [driver]",
	Short: "List sensors by driver",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetSensorsByDriver(ctx, args[0]))
	},
}

var sensorsPendingCmd = &cobra.Command{
	Use:   "pending",
	Short: "List pending (auto-discovered) sensors awaiting approval",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetSensorsByStatus(ctx, gen.GetSensorsByStatusParamsStatus("pending")))
	},
}

var sensorsApproveCmd = &cobra.Command{
	Use:   "approve [id]",
	Short: "Approve a pending sensor (sets status to active)",
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
		return consumeJSON(client.ApproveSensor(ctx, id))
	},
}

var sensorsDismissCmd = &cobra.Command{
	Use:   "dismiss [id]",
	Short: "Dismiss a pending sensor (hides from pending list)",
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
		return consumeJSON(client.DismissSensor(ctx, id))
	},
}

func parseSensorIDArg(value string) (int, error) {
	id, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("sensor ID must be a number")
	}
	return id, nil
}

func lookupSensorByID(ctx context.Context, client *gen.ClientWithResponses, id int) (*gen.Sensor, error) {
	response, err := client.GetAllSensorsWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if response.StatusCode() != 200 || response.JSON200 == nil {
		return nil, apiResponseError(response.StatusCode(), response.Body)
	}

	for _, sensor := range *response.JSON200 {
		if sensor.Id == id {
			s := sensor
			return &s, nil
		}
	}
	return nil, fmt.Errorf("sensor %d not found", id)
}

func writeCapabilityTable(cmd *cobra.Command, capabilities []gen.Capability, latestValues map[string]string) {
	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(writer, "PROPERTY\tTYPE\tALLOWED\tLATEST")
	for _, capability := range capabilities {
		_, _ = fmt.Fprintf(
			writer,
			"%s\t%s\t%s\t%s\n",
			capability.Property,
			capability.Type,
			capabilityAllowedValues(capability),
			latestValues[capability.Property],
		)
	}
	_ = writer.Flush()
}

func writeAcceptedCommandTable(cmd *cobra.Command, accepted gen.SensorCommandAccepted) {
	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(writer, "ID\tSTATUS\tPROPERTY\tVALUE")
	_, _ = fmt.Fprintf(writer, "%d\t%s\t%s\t%s\n", accepted.Id, accepted.Status, accepted.Property, accepted.Value)
	_ = writer.Flush()
}

func writeCommandHistoryTable(cmd *cobra.Command, history []gen.CommandHistoryEntry) {
	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(writer, "ID\tSENT_AT\tPROPERTY\tVALUE\tSTATUS\tUSER")
	for _, entry := range history {
		_, _ = fmt.Fprintf(
			writer,
			"%d\t%s\t%s\t%s\t%s\t%s\n",
			entry.Id,
			entry.SentAt.Format(time.RFC3339),
			entry.Property,
			entry.Value,
			entry.Status,
			commandUserName(entry.User),
		)
	}
	_ = writer.Flush()
}

func capabilityAllowedValues(capability gen.Capability) string {
	switch {
	case capability.ValueOn != nil || capability.ValueOff != nil:
		return strings.TrimSpace(strings.Join([]string{derefOrEmpty(capability.ValueOn), "/", derefOrEmpty(capability.ValueOff)}, " "))
	case capability.Values != nil && len(*capability.Values) > 0:
		return strings.Join(*capability.Values, ", ")
	case capability.Min != nil || capability.Max != nil:
		return strings.TrimSpace(fmt.Sprintf("%s..%s %s", formatOptionalFloat(capability.Min), formatOptionalFloat(capability.Max), derefOrEmpty(capability.Unit)))
	default:
		return ""
	}
}

func latestReadingsByProperty(sensorName string, readings []gen.Reading) map[string]string {
	values := make(map[string]string)
	for _, reading := range readings {
		if reading.SensorName != sensorName {
			continue
		}
		values[reading.MeasurementType] = readingDisplayValue(reading)
	}
	return values
}

func readingDisplayValue(reading gen.Reading) string {
	if reading.TextState != nil {
		return *reading.TextState
	}
	if reading.NumericValue != nil {
		value := formatFloat(*reading.NumericValue)
		if reading.Unit == "" {
			return value
		}
		return value + " " + reading.Unit
	}
	return ""
}

func formatOptionalFloat(value *float64) string {
	if value == nil {
		return ""
	}
	return formatFloat(*value)
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func derefOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func commandUserName(user *gen.CommandHistoryUser) string {
	if user == nil {
		return ""
	}
	return user.Username
}
