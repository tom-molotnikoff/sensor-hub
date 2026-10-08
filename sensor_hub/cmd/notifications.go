package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	gen "example/sensorHub/gen"
)

var notificationsCmd = &cobra.Command{
	Use:     "notifications",
	GroupID: hubGroupID,
	Short:   "Manage notifications",
}

func init() {
	notificationsCmd.AddCommand(notificationsListCmd)
	notificationsCmd.AddCommand(notificationsReadCmd)
	notificationsCmd.AddCommand(notificationsDismissCmd)
	notificationsCmd.AddCommand(notificationsUnreadCountCmd)
	notificationsCmd.AddCommand(notificationsBulkReadCmd)
	notificationsCmd.AddCommand(notificationsBulkDismissCmd)
	notificationsCmd.AddCommand(notificationsPreferencesCmd)
	notificationsCmd.AddCommand(notificationsSetPreferenceCmd)
	rootCmd.AddCommand(notificationsCmd)
}

func parseNotificationID(s string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("notification ID must be a number")
	}
	return id, nil
}

var notificationsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List notifications",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		params := &gen.ListNotificationsParams{}
		if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
			params.Limit = &limit
		}
		if offset, _ := cmd.Flags().GetInt("offset"); offset > 0 {
			params.Offset = &offset
		}
		if includeDismissed, _ := cmd.Flags().GetBool("include-dismissed"); includeDismissed {
			v := gen.ListNotificationsParamsIncludeDismissed("true")
			params.IncludeDismissed = &v
		}
		return consumeJSON(client.ListNotifications(ctx, params))
	},
}

func init() {
	notificationsListCmd.Flags().Int("limit", 0, "Maximum number of notifications (default 50)")
	notificationsListCmd.Flags().Int("offset", 0, "Offset for pagination")
	notificationsListCmd.Flags().Bool("include-dismissed", false, "Include dismissed notifications")
}

var notificationsReadCmd = &cobra.Command{
	Use:   "read [id]",
	Short: "Mark a notification as read",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseNotificationID(args[0])
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.MarkAsRead(ctx, id))
	},
}

var notificationsDismissCmd = &cobra.Command{
	Use:   "dismiss [id]",
	Short: "Dismiss a notification",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseNotificationID(args[0])
		if err != nil {
			return err
		}
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.DismissNotification(ctx, id))
	},
}

var notificationsUnreadCountCmd = &cobra.Command{
	Use:   "unread-count",
	Short: "Get count of unread notifications",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetUnreadCount(ctx))
	},
}

var notificationsBulkReadCmd = &cobra.Command{
	Use:   "bulk-read",
	Short: "Mark all notifications as read",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.BulkMarkAsRead(ctx))
	},
}

var notificationsBulkDismissCmd = &cobra.Command{
	Use:   "bulk-dismiss",
	Short: "Dismiss all notifications",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.BulkDismiss(ctx))
	},
}

var notificationsPreferencesCmd = &cobra.Command{
	Use:   "preferences",
	Short: "Get notification channel preferences",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetChannelPreferences(ctx))
	},
}

var notificationsSetPreferenceCmd = &cobra.Command{
	Use:   "set-preference",
	Short: "Set a notification channel preference",
	RunE: func(cmd *cobra.Command, args []string) error {
		category, _ := cmd.Flags().GetString("category")
		emailEnabled, _ := cmd.Flags().GetBool("email-enabled")
		inappEnabled, _ := cmd.Flags().GetBool("inapp-enabled")

		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		body := gen.SetChannelPreferenceJSONRequestBody{
			Category:     gen.ChannelPreferenceCategory(category),
			EmailEnabled: &emailEnabled,
			InappEnabled: &inappEnabled,
		}
		return consumeJSON(client.SetChannelPreference(ctx, body))
	},
}

func init() {
	notificationsSetPreferenceCmd.Flags().String("category", "", "Notification category (required)")
	notificationsSetPreferenceCmd.Flags().Bool("email-enabled", false, "Enable email notifications")
	notificationsSetPreferenceCmd.Flags().Bool("inapp-enabled", false, "Enable in-app notifications")
	_ = notificationsSetPreferenceCmd.MarkFlagRequired("category")
}
