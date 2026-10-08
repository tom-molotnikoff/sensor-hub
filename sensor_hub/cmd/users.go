package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	gen "example/sensorHub/gen"
)

var usersCmd = &cobra.Command{
	Use:     "users",
	GroupID: hubGroupID,
	Short:   "Manage users",
}

func init() {
	usersCmd.AddCommand(usersListCmd)
	usersCmd.AddCommand(usersCreateCmd)
	usersCmd.AddCommand(usersDeleteCmd)
	usersCmd.AddCommand(usersChangePasswordCmd)
	usersCmd.AddCommand(usersSetMustChangeCmd)
	usersCmd.AddCommand(usersSetRolesCmd)
	usersCmd.AddCommand(usersDisableCmd)
	usersCmd.AddCommand(usersEnableCmd)
	rootCmd.AddCommand(usersCmd)
}

func parseUserID(s string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("user ID must be a number")
	}
	return id, nil
}

var usersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all users",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.ListUsers(ctx))
	},
}

var usersCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new user",
	RunE: func(cmd *cobra.Command, args []string) error {
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")
		email, _ := cmd.Flags().GetString("email")

		if username == "" || password == "" {
			return fmt.Errorf("--username and --password are required")
		}

		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		body := gen.CreateUserJSONRequestBody{
			Username: username,
			Password: password,
		}
		if email != "" {
			body.Email = &email
		}
		return consumeJSON(client.CreateUser(ctx, body))
	},
}

func init() {
	usersCreateCmd.Flags().String("username", "", "Username")
	usersCreateCmd.Flags().String("password", "", "Password")
	usersCreateCmd.Flags().String("email", "", "Email (optional)")
}

var usersDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a user by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseUserID(args[0])
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.DeleteUser(ctx, id))
	},
}

var usersChangePasswordCmd = &cobra.Command{
	Use:   "change-password",
	Short: "Change a user's password",
	RunE: func(cmd *cobra.Command, args []string) error {
		userID, _ := cmd.Flags().GetInt("user-id")
		newPassword, _ := cmd.Flags().GetString("new-password")

		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		body := gen.ChangePasswordJSONRequestBody{
			NewPassword: newPassword,
		}
		if userID != 0 {
			body.UserId = &userID
		}
		return consumeJSON(client.ChangePassword(ctx, body))
	},
}

func init() {
	usersChangePasswordCmd.Flags().Int("user-id", 0, "User ID")
	usersChangePasswordCmd.Flags().String("new-password", "", "New password")
	_ = usersChangePasswordCmd.MarkFlagRequired("user-id")
	_ = usersChangePasswordCmd.MarkFlagRequired("new-password")
}

var usersSetMustChangeCmd = &cobra.Command{
	Use:   "set-must-change [id]",
	Short: "Set whether a user must change their password",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseUserID(args[0])
		if err != nil {
			return err
		}
		mustChange, _ := cmd.Flags().GetBool("must-change")

		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		body := gen.SetMustChangePasswordJSONRequestBody{MustChange: mustChange}
		return consumeJSON(client.SetMustChangePassword(ctx, id, body))
	},
}

func init() {
	usersSetMustChangeCmd.Flags().Bool("must-change", false, "Whether user must change password on next login")
}

var usersSetRolesCmd = &cobra.Command{
	Use:   "set-roles [id]",
	Short: "Set roles for a user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseUserID(args[0])
		if err != nil {
			return err
		}
		roles, _ := cmd.Flags().GetStringSlice("roles")

		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		body := gen.SetUserRolesJSONRequestBody{Roles: roles}
		return consumeJSON(client.SetUserRoles(ctx, id, body))
	},
}

func init() {
	usersSetRolesCmd.Flags().StringSlice("roles", nil, "Comma-separated list of role names")
}

var usersDisableCmd = newUsersSetDisabledCmd("disable", true,
	"Disable a user",
	"Disable a user. Their sessions end at once and their API keys stop working. You cannot disable yourself or the last enabled admin.")

var usersEnableCmd = newUsersSetDisabledCmd("enable", false,
	"Enable a disabled user",
	"Enable a disabled user so they can log in and use their API keys again.")

func newUsersSetDisabledCmd(verb string, disabled bool, short, long string) *cobra.Command {
	return &cobra.Command{
		Use:   verb + " [id]",
		Short: short,
		Long:  long,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseUserID(args[0])
			if err != nil {
				return err
			}
			client, ctx, err := newAPIClient(cmd)
			if err != nil {
				return err
			}
			body := gen.SetUserDisabledJSONRequestBody{Disabled: disabled}
			return consumeJSON(client.SetUserDisabled(ctx, id, body))
		},
	}
}
