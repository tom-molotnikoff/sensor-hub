package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gen "example/sensorHub/gen"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSensorsCapabilitiesCommand_PrintsCapabilitiesWithLatestReadings(t *testing.T) {
	t.Helper()

	valueOn := "ON"
	valueOff := "OFF"
	min := 0.0
	max := 2500.0
	latestState := "ON"
	latestPower := 42.5

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/sensors":
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode([]gen.Sensor{
				{
					Id:           7,
					Name:         "office-plug",
					SensorDriver: "mqtt-zigbee2mqtt",
					Status:       gen.SensorStatusActive,
					Config:       map[string]string{},
				},
			}))
		case "/api/sensors/by-id/7/capabilities":
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode([]gen.Capability{
				{
					Property: "state",
					Type:     gen.CapabilityType("binary"),
					ValueOn:  &valueOn,
					ValueOff: &valueOff,
				},
				{
					Property: "power",
					Type:     gen.CapabilityType("numeric"),
					Min:      &min,
					Max:      &max,
				},
			}))
		case "/api/readings/ws/current":
			conn, err := upgrader.Upgrade(w, r, nil)
			require.NoError(t, err)
			defer conn.Close()

			require.NoError(t, conn.WriteJSON([]gen.Reading{
				{
					SensorName:      "office-plug",
					MeasurementType: "state",
					TextState:       &latestState,
				},
				{
					SensorName:      "office-plug",
					MeasurementType: "power",
					NumericValue:    &latestPower,
					Unit:            "W",
				},
			}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	stdout, stderr, err := executeRootCommand(t, "--server", server.URL, "sensors", "capabilities", "7")
	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Contains(t, stdout, "PROPERTY")
	assert.Contains(t, stdout, "TYPE")
	assert.Contains(t, stdout, "ALLOWED")
	assert.Contains(t, stdout, "LATEST")
	assert.Contains(t, stdout, "state")
	assert.Contains(t, stdout, "binary")
	assert.Contains(t, stdout, "ON / OFF")
	assert.Contains(t, stdout, "power")
	assert.Contains(t, stdout, "numeric")
	assert.Contains(t, stdout, "0..2500")
	assert.Contains(t, stdout, "42.5")
}

func TestSensorsCommandCommand_PrintsAcceptedCommandDetails(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/sensors/7/command", r.URL.Path)

		var body gen.SensorCommandRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "state", body.Property)
		assert.Equal(t, "ON", body.Value)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		require.NoError(t, json.NewEncoder(w).Encode(gen.SensorCommandAccepted{
			Id:       42,
			Property: "state",
			Status:   gen.SensorCommandAcceptedStatus("sent"),
			Value:    "ON",
		}))
	}))
	defer server.Close()

	stdout, stderr, err := executeRootCommand(t, "--server", server.URL, "sensors", "command", "7", "state", "ON")
	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Contains(t, stdout, "ID")
	assert.Contains(t, stdout, "STATUS")
	assert.Contains(t, stdout, "42")
	assert.Contains(t, stdout, "sent")
}

func TestSensorsCommandCommand_ReturnsServerMessageOnHTTPError(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/sensors/7/command", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		require.NoError(t, json.NewEncoder(w).Encode(gen.ErrorResponse{
			Message: "sensor is disabled",
		}))
	}))
	defer server.Close()

	stdout, stderr, err := executeRootCommand(t, "--server", server.URL, "sensors", "command", "7", "state", "ON")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sensor is disabled")
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
}

func TestSensorsCommandsCommand_PrintsLatest20HistoryRows(t *testing.T) {
	t.Helper()

	now := time.Date(2026, 5, 9, 15, 30, 0, 0, time.UTC)
	history := make([]gen.CommandHistoryEntry, 0, 21)
	for i := 21; i >= 1; i-- {
		sentAt := now.Add(time.Duration(-i) * time.Minute)
		history = append(history, gen.CommandHistoryEntry{
			Id:       i,
			SentAt:   sentAt,
			Property: fmt.Sprintf("property-%03d", i),
			Value:    fmt.Sprintf("value-%03d", i),
			Status:   gen.CommandHistoryEntryStatusSent,
			User: &gen.CommandHistoryUser{
				Id:       i,
				Username: fmt.Sprintf("user-%03d", i),
			},
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/api/sensors/by-id/7/commands", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(history))
	}))
	defer server.Close()

	stdout, stderr, err := executeRootCommand(t, "--server", server.URL, "sensors", "commands", "7")
	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Contains(t, stdout, "ID")
	assert.Contains(t, stdout, "SENT_AT")
	assert.Contains(t, stdout, "PROPERTY")
	assert.Contains(t, stdout, "VALUE")
	assert.Contains(t, stdout, "STATUS")
	assert.Contains(t, stdout, "USER")
	assert.Contains(t, stdout, "property-021")
	assert.NotContains(t, stdout, "property-001")
	assert.Len(t, nonEmptyLines(stdout), 21)
}

func TestSensorsControlHelp_DocumentsIDArgumentConvention(t *testing.T) {
	t.Helper()

	capabilitiesHelp, _, err := executeRootCommand(t, "sensors", "capabilities", "--help")
	require.NoError(t, err)
	assert.Contains(t, capabilitiesHelp, "capabilities [id]")

	commandHelp, _, err := executeRootCommand(t, "sensors", "command", "--help")
	require.NoError(t, err)
	assert.Contains(t, commandHelp, "command [id] [property] [value]")

	commandsHelp, _, err := executeRootCommand(t, "sensors", "commands", "--help")
	require.NoError(t, err)
	assert.Contains(t, commandsHelp, "commands [id]")
}

func executeRootCommand(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs(args)
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	t.Cleanup(func() {
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs(nil)
		rootCmd.SilenceErrors = false
		rootCmd.SilenceUsage = false
	})

	_, err := rootCmd.ExecuteC()
	return stdout.String(), stderr.String(), err
}

func nonEmptyLines(value string) []string {
	lines := strings.Split(value, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

func TestSensorsListCommand_DefaultsToActive(t *testing.T) {
	var requested string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode([]gen.Sensor{}))
	}))
	defer server.Close()

	_, _, err := executeRootCommand(t, "--server", server.URL, "sensors", "list")
	require.NoError(t, err)
	assert.Equal(t, "/api/sensors/status/active", requested)
}

func TestSensorsListCommand_StatusAllListsEverySensor(t *testing.T) {
	var requested string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode([]gen.Sensor{}))
	}))
	defer server.Close()

	_, _, err := executeRootCommand(t, "--server", server.URL, "sensors", "list", "--status", "all")
	require.NoError(t, err)
	assert.Equal(t, "/api/sensors", requested)
}

func TestSensorsListCommand_RejectsUnknownStatus(t *testing.T) {
	_, _, err := executeRootCommand(t, "--server", "http://127.0.0.1:1", "sensors", "list", "--status", "gone")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --status")
}
