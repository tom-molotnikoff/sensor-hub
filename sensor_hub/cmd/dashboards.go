package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

var dashboardsCmd = &cobra.Command{
	Use:     "dashboards",
	GroupID: hubGroupID,
	Short:   "Manage dashboards",
}

func init() {
	dashboardsCreateCmd.Flags().String("name", "", "Dashboard name")
	_ = dashboardsCreateCmd.MarkFlagRequired("name")
	dashboardsUpdateCmd.Flags().String("file", "", "Path to JSON file with dashboard data")
	_ = dashboardsUpdateCmd.MarkFlagRequired("file")

	dashboardsCmd.AddCommand(dashboardsListCmd)
	dashboardsCmd.AddCommand(dashboardsGetCmd)
	dashboardsCmd.AddCommand(dashboardsCreateCmd)
	dashboardsCmd.AddCommand(dashboardsDeleteCmd)
	dashboardsCmd.AddCommand(dashboardsUpdateCmd)
	rootCmd.AddCommand(dashboardsCmd)
}

func parseDashboardID(s string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("dashboard ID must be a number")
	}
	return id, nil
}

var dashboardsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all dashboards",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.ListDashboards(ctx))
	},
}

var dashboardsGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a dashboard by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseDashboardID(args[0])
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetDashboard(ctx, id))
	},
}

var dashboardsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		// Use *WithBody to preserve historical wire format ({"name": "..."})
		// without emitting a zero-valued config object.
		body, err := rawJSONReader(map[string]any{"name": name})
		if err != nil {
			return err
		}
		return consumeJSON(client.CreateDashboardWithBody(ctx, "application/json", body))
	},
}

var dashboardsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a dashboard by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseDashboardID(args[0])
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.DeleteDashboard(ctx, id))
	},
}

var dashboardsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a dashboard by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseDashboardID(args[0])
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
		return consumeJSON(client.UpdateDashboardWithBody(ctx, id, "application/json", body))
	},
}
