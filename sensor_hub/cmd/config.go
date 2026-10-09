package cmd

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	gen "example/sensorHub/gen"
)

var configCmd = &cobra.Command{
	Use:     "config",
	GroupID: hubGroupID,
	Short:   "Manage CLI configuration",
}

// apiKeyEnvVar names the environment variable a command that talks to a hub
// takes its API key from, ahead of the config file. The key is never taken as
// a flag, where the process list and the shell history would show it.
const apiKeyEnvVar = "SENSOR_HUB_API_KEY"

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up the CLI's connection to a hub",
	Long: "Write the hub's URL and an API key to ~/.sensor-hub.yaml.\n\n" +
		"On a terminal it asks for the server URL, whether to skip TLS certificate verification for an https URL, " +
		"and the API key, which is not echoed. It tests the connection and the key before saving.\n\n" +
		"In a script, --api-key-stdin reads the API key from stdin (one line, trailing newline stripped) and asks nothing, " +
		"so the key never lands in the process list. The URL comes from --server, which it requires, and --insecure is " +
		"taken as given. Nothing is written when the hub cannot be reached or refuses the key.",
	Example: "  sensor-hub config init\n" +
		"  printf '%s\\n' \"$API_KEY\" | sensor-hub config init --server https://home.sensor-hub --api-key-stdin",
	RunE: runConfigInit,
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

	configInitCmd.Flags().Bool("api-key-stdin", false, "Read the API key from stdin and ask nothing; needs --server")

	rootCmd.PersistentFlags().String("server", "", "Sensor Hub server URL (overrides config file)")
	rootCmd.PersistentFlags().Bool("insecure", false, "Skip TLS certificate verification (for self-signed certs)")
}

func configFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".sensor-hub.yaml")
}

// loadClientConfig resolves the hub a command talks to. The server URL comes
// from --server, else the config file; the API key from SENSOR_HUB_API_KEY,
// else the config file. With both --server and the environment variable given,
// no config file is needed.
func loadClientConfig(cmd *cobra.Command) (serverURL string, apiKey string, insecure bool, err error) {
	serverFlag, _ := cmd.Flags().GetString("server")
	insecureFlag, _ := cmd.Flags().GetBool("insecure")
	apiKeyEnv := os.Getenv(apiKeyEnvVar)

	if serverFlag != "" && apiKeyEnv != "" {
		return serverFlag, apiKeyEnv, insecureFlag, nil
	}

	v := viper.New()
	v.SetConfigFile(configFilePath())
	v.SetConfigType("yaml")

	if readErr := v.ReadInConfig(); readErr != nil {
		if serverFlag == "" {
			return "", "", false, fmt.Errorf("no config file found at %s - run 'sensor-hub config init' to set up", configFilePath())
		}
	}

	if serverFlag == "" {
		serverURL = v.GetString("server")
	} else {
		serverURL = serverFlag
	}
	if apiKeyEnv == "" {
		apiKey = v.GetString("api_key")
	} else {
		apiKey = apiKeyEnv
	}
	if !insecureFlag {
		insecure = v.GetBool("insecure")
	} else {
		insecure = insecureFlag
	}

	if serverURL == "" {
		return "", "", false, fmt.Errorf("server URL not configured - run 'sensor-hub config init' or pass --server")
	}

	return serverURL, apiKey, insecure, nil
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	keyFromStdin, _ := cmd.Flags().GetBool("api-key-stdin")
	var cfg clientConfig
	var err error
	if keyFromStdin {
		cfg, err = configFromStdin(cmd)
	} else {
		cfg, err = configFromPrompts(cmd)
	}
	if err != nil {
		return err
	}
	return writeClientConfig(cmd, cfg)
}

// configFromStdin takes the server from --server and the API key from stdin,
// and refuses to go on unless the hub answers and accepts the key.
func configFromStdin(cmd *cobra.Command) (clientConfig, error) {
	serverURL, _ := cmd.Flags().GetString("server")
	if serverURL == "" {
		return clientConfig{}, errors.New("--api-key-stdin needs --server, the URL of the hub")
	}
	insecure, _ := cmd.Flags().GetBool("insecure")
	line, err := readSecretLine(cmd.InOrStdin())
	if err != nil {
		return clientConfig{}, err
	}
	cfg := clientConfig{serverURL: strings.TrimRight(serverURL, "/"), apiKey: strings.TrimSpace(line), insecure: insecure}
	if cfg.apiKey == "" {
		return clientConfig{}, errors.New("no API key on stdin")
	}

	out := cmd.OutOrStdout()
	client := configProbeClient(cfg.insecure)
	fmt.Fprintf(out, "Testing connection to %s...\n", cfg.serverURL)
	if err := checkHubReachable(client, cfg.serverURL); err != nil {
		return clientConfig{}, err
	}
	fmt.Fprintln(out, "✓ Server is reachable")
	username, err := checkAPIKey(client, cfg.serverURL, cfg.apiKey)
	if err != nil {
		return clientConfig{}, err
	}
	fmt.Fprintf(out, "✓ Authenticated as %s\n", username)
	return cfg, nil
}

// configFromPrompts asks for the server and the API key on the terminal,
// without echoing the key. A hub that cannot be reached or a refused key is
// reported, and the person at the terminal decides whether to save anyway.
func configFromPrompts(cmd *cobra.Command) (clientConfig, error) {
	fd, ok := stdinTerminal(cmd)
	if !ok {
		return clientConfig{}, errors.New("stdin is not a terminal, so config init cannot prompt; " +
			"pipe the API key in with --api-key-stdin and give the URL with --server")
	}
	reader := bufio.NewReader(cmd.InOrStdin())
	out := cmd.OutOrStdout()
	ask := func(prompt string) string {
		fmt.Fprint(out, prompt)
		answer, _ := reader.ReadString('\n')
		return strings.TrimSpace(answer)
	}

	cfg := clientConfig{serverURL: strings.TrimRight(ask("Enter Sensor Hub server URL [http://localhost:8080]: "), "/")}
	if cfg.serverURL == "" {
		cfg.serverURL = "http://localhost:8080"
	}
	if strings.HasPrefix(cfg.serverURL, "https://") {
		cfg.insecure = strings.ToLower(ask("Skip TLS certificate verification (for self-signed certs)? [y/N]: ")) == "y"
	}

	client := configProbeClient(cfg.insecure)
	fmt.Fprintf(out, "Testing connection to %s...\n", cfg.serverURL)
	if err := checkHubReachable(client, cfg.serverURL); err != nil {
		fmt.Fprintf(out, "⚠ %v\n", err)
		if strings.ToLower(ask("Continue anyway? [y/N]: ")) != "y" {
			return clientConfig{}, errors.New("setup cancelled")
		}
	} else {
		fmt.Fprintln(out, "✓ Server is reachable")
	}

	key, err := promptSecret(cmd, fd, "Enter API key (leave empty to skip): ")
	if err != nil {
		return clientConfig{}, err
	}
	cfg.apiKey = strings.TrimSpace(key)
	if cfg.apiKey != "" {
		fmt.Fprintln(out, "Testing API key authentication...")
		if username, err := checkAPIKey(client, cfg.serverURL, cfg.apiKey); err != nil {
			fmt.Fprintf(out, "⚠ %v\n", err)
		} else {
			fmt.Fprintf(out, "✓ Authenticated as %s\n", username)
		}
	}
	return cfg, nil
}

func configProbeClient(insecure bool) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // user-requested
	}
	return &http.Client{Timeout: 10 * time.Second, Transport: transport}
}

func checkHubReachable(client *http.Client, serverURL string) error {
	resp, err := client.Get(serverURL + "/api/health")
	if err != nil {
		return fmt.Errorf("could not connect to %s: %w", serverURL, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s/api/health answered HTTP %d", serverURL, resp.StatusCode)
	}
	return nil
}

// checkAPIKey asks the hub who the key belongs to, and returns the username.
func checkAPIKey(client *http.Client, serverURL, apiKey string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, serverURL+"/api/auth/me", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-API-Key", apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not test the API key: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the hub refused the API key (HTTP %d)", resp.StatusCode)
	}
	var me gen.MeResponse
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		return "", fmt.Errorf("could not read the user the API key belongs to: %w", err)
	}
	if me.User == nil {
		return "", errors.New("the hub did not say which user the API key belongs to")
	}
	return me.User.Username, nil
}

func writeClientConfig(cmd *cobra.Command, cfg clientConfig) error {
	cfgPath := configFilePath()
	content := fmt.Sprintf("server: %s\n", cfg.serverURL)
	if cfg.apiKey != "" {
		content += fmt.Sprintf("api_key: %s\n", cfg.apiKey)
	}
	if cfg.insecure {
		content += "insecure: true\n"
	}

	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Configuration saved to %s\n", cfgPath)
	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfgPath := configFilePath()

	v := viper.New()
	v.SetConfigFile(cfgPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("no config file found at %s - run 'sensor-hub config init'", cfgPath)
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
