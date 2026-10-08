package cmd

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	gen "example/sensorHub/gen"
)

var apiKeysCmd = &cobra.Command{
	Use:     "api-keys",
	GroupID: hubGroupID,
	Short:   "Manage API keys",
}

func init() {
	apiKeysCmd.AddCommand(apiKeysListCmd)
	apiKeysCmd.AddCommand(apiKeysCreateCmd)
	apiKeysCmd.AddCommand(apiKeysRevokeCmd)
	apiKeysCmd.AddCommand(apiKeysDeleteCmd)
	apiKeysCmd.AddCommand(apiKeysUpdateExpiryCmd)
	rootCmd.AddCommand(apiKeysCmd)
}

var apiKeysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List your API keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.ListApiKeys(ctx))
	},
}

var apiKeysCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.CreateApiKey(ctx, gen.CreateApiKeyJSONRequestBody{Name: name}))
	},
}

func init() {
	apiKeysCreateCmd.Flags().String("name", "", "Name for the API key")
}

func parseAPIKeyID(s string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("API key ID must be a number")
	}
	return id, nil
}

var apiKeysRevokeCmd = &cobra.Command{
	Use:   "revoke [id]",
	Short: "Revoke an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseAPIKeyID(args[0])
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.RevokeApiKey(ctx, id))
	},
}

var apiKeysDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseAPIKeyID(args[0])
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.DeleteApiKey(ctx, id))
	},
}

var apiKeysUpdateExpiryCmd = &cobra.Command{
	Use:   "update-expiry [id]",
	Short: "Update the expiry date of an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseAPIKeyID(args[0])
		if err != nil {
			return err
		}
		expiresAtRaw, _ := cmd.Flags().GetString("expires-at")

		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		var body gen.UpdateApiKeyExpiryJSONRequestBody
		if expiresAtRaw != "" {
			t, err := time.Parse(time.RFC3339, expiresAtRaw)
			if err != nil {
				return fmt.Errorf("invalid --expires-at: %w (expected RFC3339, e.g. 2026-12-31T23:59:59Z)", err)
			}
			body.ExpiresAt = &t
		}
		return consumeJSON(client.UpdateApiKeyExpiry(ctx, id, body))
	},
}

func init() {
	apiKeysUpdateExpiryCmd.Flags().String("expires-at", "", "New expiry date (RFC3339 format, e.g. 2026-12-31T23:59:59Z; omit to clear expiry)")
}
