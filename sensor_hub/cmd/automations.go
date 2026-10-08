package cmd

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	gen "example/sensorHub/gen"
)

var automationsCmd = &cobra.Command{
	Use:     "automations",
	GroupID: hubGroupID,
	Short:   "Manage automations",
	Long: "Manage automations: triggers (a time of day, an interval, a sensor reading) followed by\n" +
		"set and wait steps that the hub runs. Run \"sensor-hub automations create --help\" for the JSON shape.",
}

const automationFileHelp = `The automation is JSON in the same shape as the API body, from --file <path>
or from stdin with --file -. For example:

  {
    "name": "Lamp timer",
    "mode": "single",
    "triggers": [
      { "type": "schedule", "at": "19:00", "days": ["mon", "tue", "wed", "thu", "fri"] }
    ],
    "steps": [
      { "type": "set", "sensor_id": 14, "property": "state", "value": "ON" },
      { "type": "wait", "seconds": 14400 },
      { "type": "set", "sensor_id": 14, "property": "state", "value": "OFF" }
    ]
  }

Trigger types: "schedule" (at, days), "interval" (seconds, at least 60) and
"reading" (sensor_id, measurement_type, operator, then threshold and
rearm_margin for "falls_below"/"rises_above", or value for "becomes",
and optional hold_seconds). Step types: "set" (sensor_id, property, value)
and "wait" (seconds).`

func init() {
	automationsCreateCmd.Flags().String("file", "", `Path to the automation JSON, or "-" for stdin`)
	_ = automationsCreateCmd.MarkFlagRequired("file")
	automationsUpdateCmd.Flags().String("file", "", `Path to the automation JSON, or "-" for stdin`)
	_ = automationsUpdateCmd.MarkFlagRequired("file")
	automationsMarginSuggestionCmd.Flags().Int("sensor-id", 0, "Sensor ID")
	automationsMarginSuggestionCmd.Flags().String("measurement-type", "", "Numeric measurement type, such as temperature")
	_ = automationsMarginSuggestionCmd.MarkFlagRequired("sensor-id")
	_ = automationsMarginSuggestionCmd.MarkFlagRequired("measurement-type")

	automationsCmd.AddCommand(automationsListCmd)
	automationsCmd.AddCommand(automationsGetCmd)
	automationsCmd.AddCommand(automationsCreateCmd)
	automationsCmd.AddCommand(automationsUpdateCmd)
	automationsCmd.AddCommand(automationsDeleteCmd)
	automationsCmd.AddCommand(automationsEnableCmd)
	automationsCmd.AddCommand(automationsDisableCmd)
	automationsCmd.AddCommand(automationsRunCmd)
	automationsCmd.AddCommand(automationsCancelCmd)
	automationsCmd.AddCommand(automationsRunsCmd)
	automationsCmd.AddCommand(automationsMarginSuggestionCmd)
	rootCmd.AddCommand(automationsCmd)
}

func parseAutomationID(s string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("automation ID must be a number")
	}
	return id, nil
}

func readAutomationFile(cmd *cobra.Command) (io.Reader, error) {
	path, _ := cmd.Flags().GetString("file")
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(cmd.InOrStdin())
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	return rawJSONReader(data)
}

var automationsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List automations with their status and next fire time",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.ListAutomations(ctx))
	},
}

var automationsGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get an automation by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseAutomationID(args[0])
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetAutomation(ctx, id))
	},
}

var automationsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an automation from JSON",
	Long: "Create an automation. It is switched on unless the JSON sets \"enabled\": false,\n" +
		"and in \"single\" mode unless it sets \"mode\": \"restart\".\n\n" + automationFileHelp,
	Example: "  sensor-hub automations create --file lamp-timer.json\n" +
		"  cat lamp-timer.json | sensor-hub automations create --file -",
	RunE: func(cmd *cobra.Command, args []string) error {
		body, err := readAutomationFile(cmd)
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.CreateAutomationWithBody(ctx, "application/json", body))
	},
}

var automationsUpdateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "Replace an automation's name, triggers and steps from JSON",
	Long: "Replace an automation. Leaving out \"enabled\" or \"mode\" keeps the current setting.\n" +
		"A running or waiting run carries on with the steps it started with.\n\n" + automationFileHelp,
	Example: "  sensor-hub automations get 3 | jq '.triggers[0].at = \"20:00\"' | sensor-hub automations update 3 --file -",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseAutomationID(args[0])
		if err != nil {
			return err
		}
		body, err := readAutomationFile(cmd)
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.UpdateAutomationWithBody(ctx, id, "application/json", body))
	},
}

var automationsDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete an automation with its triggers, steps and runs",
	Long: "Delete an automation with its triggers, steps and runs. Fails with HTTP 409 while it\n" +
		"has a running or waiting run: cancel the run first, or wait for it to finish.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseAutomationID(args[0])
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.DeleteAutomation(ctx, id))
	},
}

func setAutomationEnabled(cmd *cobra.Command, idArg string, enabled bool) error {
	id, err := parseAutomationID(idArg)
	if err != nil {
		return err
	}
	client, ctx, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	return consumeJSON(client.SetAutomationEnabled(ctx, id, gen.SetAutomationEnabledJSONRequestBody{Enabled: enabled}))
}

var automationsEnableCmd = &cobra.Command{
	Use:   "enable [id]",
	Short: "Switch an automation on",
	Long:  "Switch an automation on. Its interval triggers count from now.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setAutomationEnabled(cmd, args[0], true)
	},
}

var automationsDisableCmd = &cobra.Command{
	Use:   "disable [id]",
	Short: "Switch an automation off",
	Long:  "Switch an automation off. A run already in progress finishes.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setAutomationEnabled(cmd, args[0], false)
	},
}

var automationsRunCmd = &cobra.Command{
	Use:   "run [id]",
	Short: "Run an automation now",
	Long: "Start a run straight away, even when the automation is switched off, and print it.\n" +
		"In \"single\" mode an automation that is already running records a skipped run instead,\n" +
		"so check the printed run's \"status\": \"skipped\" means nothing was started. In \"restart\"\n" +
		"mode the active run is cancelled first. Fails with HTTP 409 when the automation is broken.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseAutomationID(args[0])
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.RunAutomation(ctx, id))
	},
}

var automationsCancelCmd = &cobra.Command{
	Use:   "cancel [id] [runId]",
	Short: "Cancel a running or waiting run",
	Long: "Cancel a running or waiting run so none of its later steps run, and print the run.\n" +
		"A command it already sent carries on. Fails with HTTP 409 when the run has already ended,\n" +
		"and 404 when the run does not belong to the automation.",
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseAutomationID(args[0])
		if err != nil {
			return err
		}
		runID, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("run ID must be a number")
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.CancelAutomationRun(ctx, id, runID))
	},
}

var automationsRunsCmd = &cobra.Command{
	Use:   "runs [id]",
	Short: "List an automation's runs, newest first, with their step outcomes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseAutomationID(args[0])
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.ListAutomationRuns(ctx, id))
	},
}

var automationsMarginSuggestionCmd = &cobra.Command{
	Use:   "margin-suggestion",
	Short: "Suggest a re-arm margin for a numeric reading trigger",
	Long: "Suggest a re-arm margin from a sensor's recent readings of one numeric measurement type.\n" +
		"\"suggested_margin\" is null when \"confidence\" is \"none\" (fewer than 30 readings).",
	Example: "  sensor-hub automations margin-suggestion --sensor-id 3 --measurement-type temperature",
	RunE: func(cmd *cobra.Command, args []string) error {
		sensorID, _ := cmd.Flags().GetInt("sensor-id")
		measurementType, _ := cmd.Flags().GetString("measurement-type")
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetMarginSuggestion(ctx, &gen.GetMarginSuggestionParams{
			SensorId:        sensorID,
			MeasurementType: measurementType,
		}))
	},
}
