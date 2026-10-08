package cmd

import (
	gen "example/sensorHub/gen"

	"github.com/spf13/cobra"
)

var emailCmd = &cobra.Command{
	Use:     "email",
	GroupID: hubGroupID,
	Short:   "SMTP settings for alert and notification emails",
}

var emailShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the SMTP settings",
	Long:  "Shows the SMTP settings, whether a password is stored, and how the last send went. The password itself is never shown.",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetEmailSettings(ctx))
	},
}

var emailSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Change the SMTP settings",
	Long: "Changes the SMTP settings given as flags and leaves the others as they are.\n\n" +
		"--security is starttls (the default, which fails if the server does not offer STARTTLS), implicit_tls " +
		"(TLS from the start, usually port 465) or none (everything, the password included, unencrypted). " +
		"Both TLS modes verify the server's certificate against the system's trusted roots.\n\n" +
		"The password is never taken as a flag value. With --password-stdin it is read from stdin (one line, " +
		"trailing newline stripped). Otherwise, on a terminal, it is prompted for twice, and leaving it empty keeps " +
		"the stored one; with no terminal the stored password is kept. The hub stores it encrypted and never returns it.",
	RunE: func(cmd *cobra.Command, args []string) error {
		passwordStdin, _ := cmd.Flags().GetBool("password-stdin")

		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		resp, err := client.GetEmailSettings(ctx)
		if err != nil {
			return err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return consumeJSON(resp, nil)
		}
		var current gen.EmailSettings
		err = decodeBody(resp, &current)
		resp.Body.Close()
		if err != nil {
			return err
		}

		body := map[string]any{
			"host":         current.Host,
			"port":         current.Port,
			"security":     current.Security,
			"username":     current.Username,
			"from_address": current.FromAddress,
		}
		for flag, field := range map[string]string{
			"host": "host", "security": "security", "username": "username", "from-address": "from_address",
		} {
			if cmd.Flags().Changed(flag) {
				body[field], _ = cmd.Flags().GetString(flag)
			}
		}
		if cmd.Flags().Changed("port") {
			body["port"], _ = cmd.Flags().GetInt("port")
		}

		switch {
		case passwordStdin:
			password, err := readNewPassword(cmd, true)
			if err != nil {
				return err
			}
			body["password"] = password
		case stdinIsTerminal(cmd):
			label := "SMTP password (leave empty to keep the stored one): "
			if current.PasswordStatus == nil || *current.PasswordStatus == "unset" {
				label = "SMTP password (leave empty for none): "
			}
			password, err := readPassword(cmd, false, label)
			if err != nil {
				return err
			}
			if password != "" {
				body["password"] = password
			}
		}

		reader, err := rawJSONReader(body)
		if err != nil {
			return err
		}
		return consumeJSON(client.UpdateEmailSettingsWithBody(ctx, "application/json", reader))
	},
}

var emailTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Send a test email to your own address",
	Long:  "Sends \"Sensor Hub test email\" to the email address of the user the CLI is signed in as, through the stored SMTP settings. A failure prints the SMTP server's error.",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.SendTestEmail(ctx))
	},
}

func init() {
	emailSetCmd.Flags().String("host", "", "SMTP server host name")
	emailSetCmd.Flags().Int("port", 587, "SMTP server port")
	emailSetCmd.Flags().String("security", "starttls", "How the connection is protected: starttls, implicit_tls or none")
	emailSetCmd.Flags().String("username", "", "Login name for SMTP authentication; empty for none")
	emailSetCmd.Flags().String("from-address", "", "Address the emails are sent from")
	emailSetCmd.Flags().Bool("password-stdin", false, "Read the SMTP password from stdin (one line)")

	emailCmd.AddCommand(emailShowCmd)
	emailCmd.AddCommand(emailSetCmd)
	emailCmd.AddCommand(emailTestCmd)
	rootCmd.AddCommand(emailCmd)
}
