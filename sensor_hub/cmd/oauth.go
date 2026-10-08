package cmd

import (
	"github.com/spf13/cobra"

	gen "example/sensorHub/gen"
)

var oauthCmd = &cobra.Command{
	Use:     "oauth",
	GroupID: hubGroupID,
	Short:   "OAuth configuration commands",
}

func init() {
	oauthCmd.AddCommand(oauthStatusCmd)
	oauthCmd.AddCommand(oauthAuthorizeCmd)
	oauthCmd.AddCommand(oauthSubmitCodeCmd)
	oauthCmd.AddCommand(oauthReloadCmd)
	rootCmd.AddCommand(oauthCmd)
}

var oauthStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get OAuth configuration status",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetOAuthStatus(ctx))
	},
}

var oauthAuthorizeCmd = &cobra.Command{
	Use:   "authorize",
	Short: "Start OAuth authorization flow",
	Long:  "Initiates the OAuth authorization flow and returns an auth URL to visit along with a state token.",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.GetOAuthAuthorizeUrl(ctx))
	},
}

var oauthSubmitCodeCmd = &cobra.Command{
	Use:   "submit-code",
	Short: "Submit an OAuth authorization code",
	Long:  "Submit the authorization code received from the OAuth provider along with the state token from the authorize step.",
	RunE: func(cmd *cobra.Command, args []string) error {
		code, _ := cmd.Flags().GetString("code")
		state, _ := cmd.Flags().GetString("state")

		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.SubmitOAuthCode(ctx, gen.SubmitOAuthCodeJSONRequestBody{
			Code:  code,
			State: state,
		}))
	},
}

func init() {
	oauthSubmitCodeCmd.Flags().String("code", "", "Authorization code from OAuth provider")
	oauthSubmitCodeCmd.Flags().String("state", "", "State token from the authorize step")
	_ = oauthSubmitCodeCmd.MarkFlagRequired("code")
	_ = oauthSubmitCodeCmd.MarkFlagRequired("state")
}

var oauthReloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reload OAuth configuration from disk",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		return consumeJSON(client.ReloadOAuth(ctx))
	},
}
