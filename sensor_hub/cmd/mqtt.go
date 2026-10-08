package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	gen "example/sensorHub/gen"
)

var mqttCmd = &cobra.Command{
	Use:     "mqtt",
	GroupID: hubGroupID,
	Short:   "Manage MQTT brokers, clients, subscriptions, and view stats",
}

// ----------------------------------------------------------------------------
// Brokers
// ----------------------------------------------------------------------------

var mqttBrokersCmd = &cobra.Command{
	Use:   "brokers",
	Short: "Manage MQTT brokers",
}

var mqttBrokersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all MQTT brokers",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.ListMqttBrokers(ctx))
	},
}

func parseBrokerID(s string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("broker ID must be a number")
	}
	return id, nil
}

var mqttBrokersGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get an MQTT broker by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseBrokerID(args[0])
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetMqttBroker(ctx, id))
	},
}

var mqttBrokersCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new MQTT broker",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		brokerType, _ := cmd.Flags().GetString("type")
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		enabled, _ := cmd.Flags().GetBool("enabled")
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")

		body := gen.CreateMqttBrokerJSONRequestBody{
			Name:    name,
			Type:    brokerType,
			Enabled: enabled,
		}
		if brokerType != "embedded" {
			body.Host = &host
			body.Port = &port
		}
		if username != "" {
			body.Username = &username
		}
		if password != "" {
			body.Password = &password
		}

		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.CreateMqttBroker(ctx, body))
	},
}

var mqttBrokersUpdateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "Update an MQTT broker",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseBrokerID(args[0])
		if err != nil {
			return err
		}
		filePath, _ := cmd.Flags().GetString("file")
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		body, err := rawJSONReader(fileData)
		if err != nil {
			return err
		}
		return consumeJSON(client.UpdateMqttBrokerWithBody(ctx, id, "application/json", body))
	},
}

var mqttBrokersDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete an MQTT broker",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseBrokerID(args[0])
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.DeleteMqttBroker(ctx, id))
	},
}

var mqttBrokersEnableCmd = &cobra.Command{
	Use:   "enable [id]",
	Short: "Enable an MQTT broker",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return toggleBrokerEnabled(cmd, args[0], true)
	},
}

var mqttBrokersDisableCmd = &cobra.Command{
	Use:   "disable [id]",
	Short: "Disable an MQTT broker",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return toggleBrokerEnabled(cmd, args[0], false)
	},
}

func toggleBrokerEnabled(cmd *cobra.Command, idStr string, enabled bool) error {
	id, err := parseBrokerID(idStr)
	if err != nil {
		return err
	}

	client, ctx, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	resp, err := client.GetMqttBroker(ctx, id)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Re-use the standard error-printing path.
		return consumeJSON(resp, nil)
	}

	var broker map[string]any
	if err := decodeBody(resp, &broker); err != nil {
		return err
	}
	broker["enabled"] = enabled

	body, err := rawJSONReader(broker)
	if err != nil {
		return err
	}
	return consumeJSON(client.UpdateMqttBrokerWithBody(ctx, id, "application/json", body))
}

// ----------------------------------------------------------------------------
// Subscriptions
// ----------------------------------------------------------------------------

var mqttSubscriptionsCmd = &cobra.Command{
	Use:   "subscriptions",
	Short: "Manage MQTT subscriptions",
}

var mqttSubscriptionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List MQTT subscriptions",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		params := &gen.ListMqttSubscriptionsParams{}
		if brokerID, _ := cmd.Flags().GetInt("broker-id"); brokerID > 0 {
			params.BrokerId = &brokerID
		}
		return consumeJSON(client.ListMqttSubscriptions(ctx, params))
	},
}

func parseSubscriptionID(s string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("subscription ID must be a number")
	}
	return id, nil
}

var mqttSubscriptionsGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get an MQTT subscription by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseSubscriptionID(args[0])
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetMqttSubscription(ctx, id))
	},
}

var mqttSubscriptionsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new MQTT subscription",
	RunE: func(cmd *cobra.Command, args []string) error {
		brokerID, _ := cmd.Flags().GetInt("broker-id")
		topic, _ := cmd.Flags().GetString("topic")
		driverType, _ := cmd.Flags().GetString("driver")
		enabled, _ := cmd.Flags().GetBool("enabled")

		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		body := gen.CreateMqttSubscriptionJSONRequestBody{
			BrokerId:     brokerID,
			TopicPattern: topic,
			DriverType:   driverType,
			Enabled:      enabled,
		}
		return consumeJSON(client.CreateMqttSubscription(ctx, body))
	},
}

var mqttSubscriptionsUpdateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "Update an MQTT subscription",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseSubscriptionID(args[0])
		if err != nil {
			return err
		}
		filePath, _ := cmd.Flags().GetString("file")
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		body, err := rawJSONReader(fileData)
		if err != nil {
			return err
		}
		return consumeJSON(client.UpdateMqttSubscriptionWithBody(ctx, id, "application/json", body))
	},
}

var mqttSubscriptionsDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete an MQTT subscription",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseSubscriptionID(args[0])
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.DeleteMqttSubscription(ctx, id))
	},
}

// ----------------------------------------------------------------------------
// Stats
// ----------------------------------------------------------------------------

var mqttStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show live MQTT broker statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetMqttStats(ctx))
	},
}

func init() {
	mqttBrokersCreateCmd.Flags().String("name", "", "Broker name")
	mqttBrokersCreateCmd.Flags().String("type", "external", "Broker type (embedded or external)")
	mqttBrokersCreateCmd.Flags().String("host", "", "Broker host (external brokers only)")
	mqttBrokersCreateCmd.Flags().Int("port", 1883, "Broker port (external brokers only)")
	mqttBrokersCreateCmd.Flags().Bool("enabled", true, "Enable the broker")
	mqttBrokersCreateCmd.Flags().String("username", "", "Broker username")
	mqttBrokersCreateCmd.Flags().String("password", "", "Broker password")
	mqttBrokersCreateCmd.Flags().Bool("tls", false, "(Deprecated; use ca_cert_path/client_cert_path on update --file instead)")
	_ = mqttBrokersCreateCmd.MarkFlagRequired("name")

	mqttBrokersUpdateCmd.Flags().String("file", "", "Path to JSON file with broker data")
	_ = mqttBrokersUpdateCmd.MarkFlagRequired("file")

	mqttSubscriptionsListCmd.Flags().Int("broker-id", 0, "Filter by broker ID")

	mqttSubscriptionsCreateCmd.Flags().Int("broker-id", 0, "Broker ID")
	mqttSubscriptionsCreateCmd.Flags().String("topic", "", "MQTT topic pattern")
	mqttSubscriptionsCreateCmd.Flags().String("driver", "", "Driver type (e.g. mqtt-zigbee2mqtt)")
	mqttSubscriptionsCreateCmd.Flags().Int("qos", 0, "(Deprecated; not part of the API spec)")
	mqttSubscriptionsCreateCmd.Flags().Bool("enabled", true, "Enable the subscription")
	_ = mqttSubscriptionsCreateCmd.MarkFlagRequired("broker-id")
	_ = mqttSubscriptionsCreateCmd.MarkFlagRequired("topic")
	_ = mqttSubscriptionsCreateCmd.MarkFlagRequired("driver")

	mqttSubscriptionsUpdateCmd.Flags().String("file", "", "Path to JSON file with subscription data")
	_ = mqttSubscriptionsUpdateCmd.MarkFlagRequired("file")

	mqttBrokersCmd.AddCommand(mqttBrokersListCmd)
	mqttBrokersCmd.AddCommand(mqttBrokersGetCmd)
	mqttBrokersCmd.AddCommand(mqttBrokersCreateCmd)
	mqttBrokersCmd.AddCommand(mqttBrokersUpdateCmd)
	mqttBrokersCmd.AddCommand(mqttBrokersDeleteCmd)
	mqttBrokersCmd.AddCommand(mqttBrokersEnableCmd)
	mqttBrokersCmd.AddCommand(mqttBrokersDisableCmd)

	mqttSubscriptionsCmd.AddCommand(mqttSubscriptionsListCmd)
	mqttSubscriptionsCmd.AddCommand(mqttSubscriptionsGetCmd)
	mqttSubscriptionsCmd.AddCommand(mqttSubscriptionsCreateCmd)
	mqttSubscriptionsCmd.AddCommand(mqttSubscriptionsUpdateCmd)
	mqttSubscriptionsCmd.AddCommand(mqttSubscriptionsDeleteCmd)

	mqttCmd.AddCommand(mqttBrokersCmd)
	mqttCmd.AddCommand(mqttSubscriptionsCmd)
	mqttCmd.AddCommand(mqttStatsCmd)
	rootCmd.AddCommand(mqttCmd)
}
