package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:     "health",
	GroupID: hubGroupID,
	Short:   "Check connectivity to a Sensor Hub server",
	RunE:    runHealth,
}

func init() {
	rootCmd.AddCommand(healthCmd)
}

func runHealth(cmd *cobra.Command, args []string) error {
	// Health check works without an API key — we honour the configured
	// server (and --insecure flag) but don't require authentication.
	serverURL, _, insecure, err := loadClientConfig(cmd)
	if err != nil {
		serverFlag, _ := cmd.Flags().GetString("server")
		if serverFlag != "" {
			serverURL = serverFlag
			insecure, _ = cmd.Flags().GetBool("insecure")
		} else {
			return err
		}
	}

	client, ctx, err := newAPIClientNoAuth(serverURL, insecure)
	if err != nil {
		return err
	}

	start := time.Now()
	resp, err := client.GetHealth(ctx)
	latency := time.Since(start)
	if err != nil {
		result := map[string]any{
			"status":  "error",
			"server":  serverURL,
			"message": err.Error(),
		}
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))
		return fmt.Errorf("could not connect to %s", serverURL)
	}
	defer resp.Body.Close()

	result := map[string]any{
		"status":     "ok",
		"server":     serverURL,
		"latency_ms": latency.Milliseconds(),
		"http_code":  resp.StatusCode,
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
	return nil
}
