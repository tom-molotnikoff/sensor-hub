package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	gen "example/sensorHub/gen"
)

var mqttClientsCmd = &cobra.Command{
	Use:   "clients",
	Short: "Manage the MQTT clients that dial in to the embedded broker",
}

func parseMQTTClientID(s string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("client ID must be a number")
	}
	return id, nil
}

var mqttClientsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List MQTT clients",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.ListMqttClients(ctx))
	},
}

var mqttClientsGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get an MQTT client by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseMQTTClientID(args[0])
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetMqttClient(ctx, id))
	},
}

var mqttClientsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an MQTT client and print its generated password",
	Long: "Creates a client for the embedded broker. The response carries the generated password, " +
		"which is shown once: put it in your device's MQTT config now.",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		topicPrefix, _ := cmd.Flags().GetString("topic-prefix")
		enabled, _ := cmd.Flags().GetBool("enabled")

		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		body := gen.CreateMqttClientJSONRequestBody{Name: name, TopicPrefix: topicPrefix, Enabled: &enabled}
		return consumeJSON(client.CreateMqttClient(ctx, body))
	},
}

var mqttClientsUpdateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "Change an MQTT client's name, topic prefix or enabled flag",
	Long: "Changes only the settings given. A connected client is disconnected, " +
		"so its next CONNECT is checked against the new settings.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseMQTTClientID(args[0])
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}

		resp, err := client.GetMqttClient(ctx, id)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return consumeJSON(resp, nil)
		}
		var current gen.MQTTClient
		if err := decodeBody(resp, &current); err != nil {
			return err
		}

		body := gen.UpdateMqttClientJSONRequestBody{Name: current.Name, TopicPrefix: current.TopicPrefix}
		if cmd.Flags().Changed("name") {
			body.Name, _ = cmd.Flags().GetString("name")
		}
		if cmd.Flags().Changed("topic-prefix") {
			body.TopicPrefix, _ = cmd.Flags().GetString("topic-prefix")
		}
		if cmd.Flags().Changed("enabled") {
			enabled, _ := cmd.Flags().GetBool("enabled")
			body.Enabled = &enabled
		}
		return consumeJSON(client.UpdateMqttClient(ctx, id, body))
	},
}

var mqttClientsDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Disconnect and delete an MQTT client",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseMQTTClientID(args[0])
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.DeleteMqttClient(ctx, id))
	},
}

var mqttClientsRotatePasswordCmd = &cobra.Command{
	Use:   "rotate-password [id]",
	Short: "Generate a new password for an MQTT client and print it",
	Long: "Replaces the client's password. The response carries the new password, which is shown once. " +
		"The old password stops working on the client's next CONNECT.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseMQTTClientID(args[0])
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.RotateMqttClientPassword(ctx, id))
	},
}

func init() {
	mqttClientsCreateCmd.Flags().String("name", "", "MQTT username the device connects with")
	mqttClientsCreateCmd.Flags().String("topic-prefix", "", "Topic tree the client is limited to, ending in / (e.g. zigbee2mqtt/)")
	mqttClientsCreateCmd.Flags().Bool("enabled", true, "Allow the client to connect")
	_ = mqttClientsCreateCmd.MarkFlagRequired("name")
	_ = mqttClientsCreateCmd.MarkFlagRequired("topic-prefix")

	mqttClientsUpdateCmd.Flags().String("name", "", "New MQTT username")
	mqttClientsUpdateCmd.Flags().String("topic-prefix", "", "New topic prefix, ending in /")
	mqttClientsUpdateCmd.Flags().Bool("enabled", true, "Allow the client to connect (--enabled=false disconnects it)")

	mqttClientsCmd.AddCommand(mqttClientsListCmd)
	mqttClientsCmd.AddCommand(mqttClientsGetCmd)
	mqttClientsCmd.AddCommand(mqttClientsCreateCmd)
	mqttClientsCmd.AddCommand(mqttClientsUpdateCmd)
	mqttClientsCmd.AddCommand(mqttClientsDeleteCmd)
	mqttClientsCmd.AddCommand(mqttClientsRotatePasswordCmd)
	mqttCmd.AddCommand(mqttClientsCmd)
}
