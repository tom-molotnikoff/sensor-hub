package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	gen "example/sensorHub/gen"
)

var rolesCmd = &cobra.Command{
	Use:     "roles",
	GroupID: hubGroupID,
	Short:   "Manage roles and permissions",
}

func init() {
	rolesCmd.AddCommand(rolesListCmd)
	rolesCmd.AddCommand(rolesListPermissionsCmd)
	rolesCmd.AddCommand(rolesGetPermissionsCmd)
	rolesCmd.AddCommand(rolesAssignPermissionCmd)
	rolesCmd.AddCommand(rolesRemovePermissionCmd)
	rootCmd.AddCommand(rolesCmd)
}

func parseRoleID(s string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("role ID must be a number")
	}
	return id, nil
}

var rolesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all roles",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.ListRoles(ctx))
	},
}

var rolesListPermissionsCmd = &cobra.Command{
	Use:   "list-permissions",
	Short: "List all permissions",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.ListPermissions(ctx))
	},
}

var rolesGetPermissionsCmd = &cobra.Command{
	Use:   "get-permissions [roleId]",
	Short: "Get permissions for a specific role",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseRoleID(args[0])
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetRolePermissions(ctx, id))
	},
}

var rolesAssignPermissionCmd = &cobra.Command{
	Use:   "assign-permission [roleId]",
	Short: "Assign a permission to a role",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseRoleID(args[0])
		if err != nil {
			return err
		}
		permissionID, _ := cmd.Flags().GetInt("permission-id")
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		body := gen.AssignPermissionJSONRequestBody{PermissionId: permissionID}
		return consumeJSON(client.AssignPermission(ctx, id, body))
	},
}

func init() {
	rolesAssignPermissionCmd.Flags().Int("permission-id", 0, "Permission ID to assign")
	_ = rolesAssignPermissionCmd.MarkFlagRequired("permission-id")
}

var rolesRemovePermissionCmd = &cobra.Command{
	Use:   "remove-permission [roleId] [permissionId]",
	Short: "Remove a permission from a role",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		roleID, err := parseRoleID(args[0])
		if err != nil {
			return err
		}
		permissionID, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("permission ID must be a number")
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.RemovePermission(ctx, roleID, permissionID))
	},
}
