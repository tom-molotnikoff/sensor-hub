package cmd

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:     "config",
	GroupID: hubGroupID,
	Short:   "Manage CLI configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Interactive setup wizard for CLI configuration",
	RunE:  runConfigInit,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current CLI configuration",
	RunE:  runConfigShow,
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)

	rootCmd.PersistentFlags().String("server", "", "Sensor Hub server URL (overrides config file)")
	rootCmd.PersistentFlags().String("api-key", "", "API key (overrides config file)")
	rootCmd.PersistentFlags().Bool("insecure", false, "Skip TLS certificate verification (for self-signed certs)")
}

func configFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".sensor-hub.yaml")
}

func loadClientConfig(cmd *cobra.Command) (serverURL string, apiKey string, insecure bool, err error) {
	serverFlag, _ := cmd.Flags().GetString("server")
	apiKeyFlag, _ := cmd.Flags().GetString("api-key")
	insecureFlag, _ := cmd.Flags().GetBool("insecure")

	if serverFlag != "" && apiKeyFlag != "" {
		return serverFlag, apiKeyFlag, insecureFlag, nil
	}

	v := viper.New()
	v.SetConfigFile(configFilePath())
	v.SetConfigType("yaml")

	if readErr := v.ReadInConfig(); readErr != nil {
		if serverFlag == "" {
			return "", "", false, fmt.Errorf("no config file found at %s — run 'sensor-hub config init' to set up", configFilePath())
		}
	}

	if serverFlag == "" {
		serverURL = v.GetString("server")
	} else {
		serverURL = serverFlag
	}
	if apiKeyFlag == "" {
		apiKey = v.GetString("api_key")
	} else {
		apiKey = apiKeyFlag
	}
	if !insecureFlag {
		insecure = v.GetBool("insecure")
	} else {
		insecure = insecureFlag
	}

	if serverURL == "" {
		return "", "", false, fmt.Errorf("server URL not configured — run 'sensor-hub config init' or pass --server")
	}

	return serverURL, apiKey, insecure, nil
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter Sensor Hub server URL [http://localhost:8080]: ")
	serverURL, _ := reader.ReadString('\n')
	serverURL = strings.TrimSpace(serverURL)
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}
	serverURL = strings.TrimRight(serverURL, "/")

	insecure := false
	if strings.HasPrefix(serverURL, "https://") {
		fmt.Print("Skip TLS certificate verification (for self-signed certs)? [y/N]: ")
		tlsAnswer, _ := reader.ReadString('\n')
		insecure = strings.TrimSpace(strings.ToLower(tlsAnswer)) == "y"
	}

	// Test connectivity
	fmt.Printf("Testing connection to %s...\n", serverURL)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // user-requested
	}
	client := &http.Client{Timeout: 10 * time.Second, Transport: transport}
	resp, err := client.Get(serverURL + "/api/health")
	if err != nil {
		fmt.Printf("⚠ Could not connect: %v\n", err)
		fmt.Print("Continue anyway? [y/N]: ")
		confirm, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(confirm)) != "y" {
			return fmt.Errorf("setup cancelled")
		}
	} else {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			fmt.Println("✓ Server is reachable")
		} else {
			fmt.Printf("⚠ Server returned status %d\n", resp.StatusCode)
		}
	}

	fmt.Print("Enter API key (leave empty to skip): ")
	apiKey, _ := reader.ReadString('\n')
	apiKey = strings.TrimSpace(apiKey)

	if apiKey != "" {
		fmt.Print("Testing API key authentication...")
		req, _ := http.NewRequest("GET", serverURL+"/api/auth/me", nil)
		req.Header.Set("X-API-Key", apiKey)
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("\n⚠ Auth test failed: %v\n", err)
		} else {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				var user map[string]interface{}
				if json.Unmarshal(body, &user) == nil {
					fmt.Printf("\n✓ Authenticated as %s\n", user["username"])
				} else {
					fmt.Println("\n✓ Authentication successful")
				}
			} else {
				fmt.Printf("\n⚠ Auth returned status %d — key may be invalid\n", resp.StatusCode)
			}
		}
	}

	// Write config file
	cfgPath := configFilePath()
	content := fmt.Sprintf("server: %s\n", serverURL)
	if apiKey != "" {
		content += fmt.Sprintf("api_key: %s\n", apiKey)
	}
	if insecure {
		content += "insecure: true\n"
	}

	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("✓ Configuration saved to %s\n", cfgPath)
	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfgPath := configFilePath()

	v := viper.New()
	v.SetConfigFile(cfgPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("no config file found at %s — run 'sensor-hub config init'", cfgPath)
	}

	server := v.GetString("server")
	apiKey := v.GetString("api_key")
	insecure := v.GetBool("insecure")

	fmt.Printf("Config file: %s\n", cfgPath)
	fmt.Printf("Server:      %s\n", server)
	if apiKey != "" {
		if len(apiKey) > 12 {
			fmt.Printf("API Key:     %s...\n", apiKey[:12])
		} else {
			fmt.Printf("API Key:     %s\n", apiKey)
		}
	} else {
		fmt.Println("API Key:     (not set)")
	}
	if insecure {
		fmt.Println("Insecure:    true (TLS verification disabled)")
	}

	return nil
}
