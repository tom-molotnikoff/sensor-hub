package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	gen "example/sensorHub/gen"
)

var authCmd = &cobra.Command{
	Use:     "auth",
	GroupID: hubGroupID,
	Short:   "Authentication commands",
}

func init() {
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authMeCmd)
	authCmd.AddCommand(authSessionsCmd)
	authCmd.AddCommand(authRevokeSessionCmd)
	rootCmd.AddCommand(authCmd)
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with username and password",
	Long:  "Log in to the hub as a user.\n\n" + existingPasswordInputHelp,
	Example: "  sensor-hub auth login --username alice\n" +
		"  printf '%s\\n' \"$PASSWORD\" | sensor-hub auth login --username alice --password-stdin",
	RunE: func(cmd *cobra.Command, args []string) error {
		username, _ := cmd.Flags().GetString("username")
		fromStdin, _ := cmd.Flags().GetBool("password-stdin")

		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		password, err := readExistingPassword(cmd, fromStdin)
		if err != nil {
			return err
		}
		return consumeJSON(client.Login(ctx, gen.LoginJSONRequestBody{
			Username: username,
			Password: password,
		}))
	},
}

func init() {
	authLoginCmd.Flags().String("username", "", "Username")
	authLoginCmd.Flags().Bool("password-stdin", false, "Read the password from stdin instead of prompting for it")
	_ = authLoginCmd.MarkFlagRequired("username")
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "End the current session",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.Logout(ctx))
	},
}

var authMeCmd = &cobra.Command{
	Use:   "me",
	Short: "Get current authenticated user info",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetCurrentUser(ctx))
	},
}

var authSessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "List active sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.ListSessions(ctx))
	},
}

var authRevokeSessionCmd = &cobra.Command{
	Use:   "revoke-session [id]",
	Short: "Revoke a specific session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("session ID must be a number")
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.RevokeSession(ctx, id))
	},
}
