//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"example/sensorHub/testharness"
)

var (
	env            *testharness.Env
	client         *testharness.Client
	mockSensorURLs []string
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	sensors, cleanupContainers, err := testharness.StartMockSensorsForMain(ctx, 2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start mock sensors: %v\n", err)
		os.Exit(1)
	}

	mockSensorURLs = make([]string, len(sensors))
	for i, s := range sensors {
		mockSensorURLs[i] = s.URL
	}

	e, cleanupServer, err := testharness.StartServerForMain(mockSensorURLs)
	if err != nil {
		cleanupContainers()
		fmt.Fprintf(os.Stderr, "failed to start server: %v\n", err)
		os.Exit(1)
	}
	env = e

	client = testharness.NewClient(nil, env.ServerURL)
	status := client.Login(env.AdminUser, env.AdminPass)
	if status != http.StatusOK {
		cleanupServer()
		cleanupContainers()
		fmt.Fprintf(os.Stderr, "admin login failed: %d\n", status)
		os.Exit(1)
	}

	// Admin user has MustChangePassword=true; change it to unlock the API
	changeStatus := client.ChangePassword(env.AdminPass)
	if changeStatus != http.StatusOK {
		cleanupServer()
		cleanupContainers()
		fmt.Fprintf(os.Stderr, "admin password change failed: %d\n", changeStatus)
		os.Exit(1)
	}

	code := m.Run()

	cleanupSharedZigbee2MQTTBridgeBroker()
	cleanupSensorHubBinary()
	cleanupLinuxSensorHub()
	cleanupServer()
	cleanupContainers()
	os.Exit(code)
}

func ptrStr(s string) *string { return &s }

func ptrInt(i int) *int { return &i }

func ptrBool(b bool) *bool { return &b }
