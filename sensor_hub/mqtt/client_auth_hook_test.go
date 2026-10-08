package mqtt

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"testing"
	"time"

	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	"example/sensorHub/service"

	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// startAuthBroker starts an embedded broker that authenticates against a real
// MQTT client store holding an enabled "plug" and a disabled "retired" client,
// both with the password "right".
func startAuthBroker(t *testing.T) string {
	t.Helper()
	return startAuthBrokerWith(t, BrokerConfig{TCPAddress: "127.0.0.1:0"})
}

// startAuthBrokerWith is startAuthBroker with the broker configuration given.
func startAuthBrokerWith(t *testing.T, config BrokerConfig) string {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := database.Open(&appProps.ApplicationConfiguration{
		DatabasePath:              filepath.Join(t.TempDir(), "sensor_hub.db"),
		DatabaseReaderConnections: 1,
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	repo := database.NewMQTTClientRepository(db, logger)
	_, err = repo.Add(context.Background(), "plug", "p/", service.HashMQTTClientPassword("right"), true)
	require.NoError(t, err)
	_, err = repo.Add(context.Background(), "retired", "r/", service.HashMQTTClientPassword("right"), false)
	require.NoError(t, err)

	clients := service.NewMQTTClientService(repo, logger)
	broker := NewEmbeddedBroker(config, clients, logger)
	clients.SetSessions(broker)
	require.NoError(t, broker.Start())
	t.Cleanup(func() { _ = broker.Stop() })
	listener, ok := broker.Server().Listeners.Get("sensor-hub-tcp")
	require.True(t, ok)
	return listener.Address()
}

// connackCode sends a CONNECT at the given protocol version and returns the
// CONNACK's return code (v3.1.1) or reason code (v5).
func connackCode(t *testing.T, address string, version byte, username, password string) byte {
	t.Helper()
	conn, err := net.Dial("tcp", address)
	require.NoError(t, err)
	defer conn.Close()

	pk := packets.Packet{
		FixedHeader:     packets.FixedHeader{Type: packets.Connect},
		ProtocolVersion: version,
		Connect: packets.ConnectParams{
			ProtocolName:     []byte("MQTT"),
			Clean:            true,
			Keepalive:        30,
			ClientIdentifier: "auth-hook-test",
			UsernameFlag:     username != "",
			Username:         []byte(username),
			PasswordFlag:     password != "",
			Password:         []byte(password),
		},
	}
	var buf bytes.Buffer
	require.NoError(t, pk.ConnectEncode(&buf))
	_, err = conn.Write(buf.Bytes())
	require.NoError(t, err)

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	header := make([]byte, 4) // fixed header, remaining length, ack flags, code
	_, err = io.ReadFull(conn, header)
	require.NoError(t, err)
	require.Equal(t, packets.Connack, header[0]>>4)
	return header[3]
}

func TestClientAuthHook_RefusesV5WithNotAuthorized(t *testing.T) {
	address := startAuthBroker(t)

	assert.Equal(t, packets.ErrNotAuthorized.Code, connackCode(t, address, 5, "plug", "wrong"))
	assert.Equal(t, packets.CodeSuccess.Code, connackCode(t, address, 5, "plug", "right"))
}

func TestClientAuthHook_CountsRefusalsByReason(t *testing.T) {
	reader := recordMetrics(t)

	address := startAuthBroker(t)
	connackCode(t, address, 4, "plug", "wrong")
	connackCode(t, address, 4, "stranger", "right")
	connackCode(t, address, 4, "retired", "right")

	assert.Equal(t, map[string]int64{"auth": 2, "disabled": 1}, refusalCounts(t, reader))
}

// recordMetrics points the global meter provider at a manual reader for the
// rest of the test.
func recordMetrics(t *testing.T) *sdkmetric.ManualReader {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	t.Cleanup(func() { otel.SetMeterProvider(previous) })
	return reader
}

// refusalCounts returns the refused-CONNECT counter's value by reason.
func refusalCounts(t *testing.T, reader *sdkmetric.ManualReader) map[string]int64 {
	t.Helper()
	var collected metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &collected))
	counts := map[string]int64{}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != "sensor_hub.mqtt.connect.refused" {
				continue
			}
			for _, point := range m.Data.(metricdata.Sum[int64]).DataPoints {
				reason, _ := point.Attributes.Value("reason")
				counts[reason.AsString()] = point.Value
			}
		}
	}
	return counts
}
