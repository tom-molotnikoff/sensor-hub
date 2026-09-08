//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	mqttpkg "example/sensorHub/mqtt"
	"example/sensorHub/service"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	zigbee2MQTTBridgeBrokerOnce sync.Once
	zigbee2MQTTBridgeBroker     *mqttpkg.EmbeddedBroker
	zigbee2MQTTBridgeBrokerPort int
	zigbee2MQTTBridgeBrokerErr  error
)

type zigbee2MQTTBridgeFixture struct {
	ctx         context.Context
	port        int
	sensorRepo  *database.SensorRepository
	brokerRepo  *database.MQTTBrokerRepository
	subRepo     *database.MQTTSubscriptionRepository
	connManager *mqttpkg.ConnectionManager
	brokerID    int
	subID       int
}

func setupZigbee2MQTTBridgeFixture(t *testing.T, brokerName string, cleanupSensors ...string) zigbee2MQTTBridgeFixture {
	t.Helper()

	ctx := context.Background()
	logger := slog.Default()
	port := sharedZigbee2MQTTBridgeBrokerPort(t)

	sensorRepo := database.NewSensorRepository(env.DB, logger)
	mtRepo := database.NewMeasurementTypeRepository(env.DB, logger)
	readingsRepo := database.NewReadingsRepository(env.DB, sensorRepo, mtRepo, logger)
	brokerRepo := database.NewMQTTBrokerRepository(env.DB, logger)
	subRepo := database.NewMQTTSubscriptionRepository(env.DB, logger)

	sensorService := service.NewSensorService(sensorRepo, readingsRepo, mtRepo, nil, nil, service.NewReadingsSampler(readingsRepo, logger), logger)
	connManager := mqttpkg.NewConnectionManager(sensorService, subRepo, brokerRepo, logger)

	resolvedBrokerName := fmt.Sprintf("%s-%d", brokerName, time.Now().UnixNano())
	brokerID, err := brokerRepo.Add(ctx, gen.MQTTBroker{
		Name:    resolvedBrokerName,
		Host:    "127.0.0.1",
		Port:    port,
		Type:    "external",
		Enabled: true,
	})
	require.NoError(t, err)

	subID, err := subRepo.Add(ctx, gen.MQTTSubscription{
		BrokerId:     brokerID,
		TopicPattern: "zigbee2mqtt/#",
		DriverType:   "mqtt-zigbee2mqtt",
		Enabled:      true,
	})
	require.NoError(t, err)

	require.NoError(t, connManager.ConnectBroker(ctx, gen.MQTTBroker{
		Id:      &brokerID,
		Name:    resolvedBrokerName,
		Host:    "127.0.0.1",
		Port:    port,
		Type:    "external",
		Enabled: true,
	}))
	require.Eventually(t, func() bool {
		return connManager.IsConnected(brokerID)
	}, 5*time.Second, 100*time.Millisecond)

	t.Cleanup(func() {
		connManager.DisconnectBroker(brokerID)
		_ = subRepo.Delete(ctx, subID)
		_ = brokerRepo.Delete(ctx, brokerID)
		for _, sensorName := range cleanupSensors {
			_ = client.DeleteSensor(sensorName)
		}
	})

	return zigbee2MQTTBridgeFixture{
		ctx:         ctx,
		port:        port,
		sensorRepo:  sensorRepo,
		brokerRepo:  brokerRepo,
		subRepo:     subRepo,
		connManager: connManager,
		brokerID:    brokerID,
		subID:       subID,
	}
}

func (f zigbee2MQTTBridgeFixture) newPublisher(t *testing.T, clientID string) pahomqtt.Client {
	t.Helper()

	mqttClient := pahomqtt.NewClient(
		pahomqtt.NewClientOptions().
			AddBroker(fmt.Sprintf("tcp://127.0.0.1:%d", f.port)).
			SetClientID(clientID),
	)
	token := mqttClient.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	t.Cleanup(func() {
		mqttClient.Disconnect(250)
	})
	return mqttClient
}

func sharedZigbee2MQTTBridgeBrokerPort(t *testing.T) int {
	t.Helper()

	zigbee2MQTTBridgeBrokerOnce.Do(func() {
		zigbee2MQTTBridgeBrokerPort, zigbee2MQTTBridgeBrokerErr = reserveTCPPortNumber()
		if zigbee2MQTTBridgeBrokerErr != nil {
			return
		}

		zigbee2MQTTBridgeBroker = mqttpkg.NewEmbeddedBroker(mqttpkg.BrokerConfig{
			TCPAddress: fmt.Sprintf(":%d", zigbee2MQTTBridgeBrokerPort),
		}, slog.Default())
		zigbee2MQTTBridgeBrokerErr = zigbee2MQTTBridgeBroker.Start()
	})

	require.NoError(t, zigbee2MQTTBridgeBrokerErr)
	return zigbee2MQTTBridgeBrokerPort
}

func cleanupSharedZigbee2MQTTBridgeBroker() {
	if zigbee2MQTTBridgeBroker == nil {
		return
	}
	_ = zigbee2MQTTBridgeBroker.Stop()
}

func TestZigbee2MQTTBridgeDevices_BackfillsSensorMetadata(t *testing.T) {
	port := reserveTCPPort(t)
	sensorName := fmt.Sprintf("front-door-%d", port)
	fixture := setupZigbee2MQTTBridgeFixture(t, "integration-z2m", sensorName)

	_, status := client.AddSensor(gen.Sensor{
		Name:         sensorName,
		SensorDriver: "mqtt-zigbee2mqtt",
		Config:       map[string]string{},
	})
	require.Equal(t, http.StatusCreated, status)

	mqttClient := fixture.newPublisher(t, fmt.Sprintf("integration-z2m-publisher-%d", port))

	payload := fmt.Sprintf(`[
		{
			"ieee_address": "0x00158d00018255df",
			"friendly_name": %q,
			"definition": {
				"model": "MCCGQ11LM",
				"vendor": "Aqara",
				"description": "Door and window sensor",
				"exposes": [
					{"type": "binary", "property": "contact", "name": "contact", "access": 1}
				]
			}
		}
	]`, sensorName)

	pub := mqttClient.Publish("zigbee2mqtt/bridge/devices", 0, false, payload)
	require.True(t, pub.WaitTimeout(5*time.Second))
	require.NoError(t, pub.Error())

	var sensor gen.Sensor
	require.Eventually(t, func() bool {
		var currentStatus int
		sensor, currentStatus = client.GetSensorByName(sensorName)
		if currentStatus != http.StatusOK || sensor.Metadata == nil {
			return false
		}
		metadata := *sensor.Metadata
		return metadata["manufacturer"] == "Aqara" &&
			metadata["model"] == "MCCGQ11LM" &&
			metadata["description"] == "Door and window sensor" &&
			metadata["ieee_address"] == "0x00158d00018255df"
	}, 5*time.Second, 100*time.Millisecond)

	exposesJSON, err := json.Marshal((*sensor.Metadata)["exposes"])
	require.NoError(t, err)
	assert.JSONEq(t, `[{"type":"binary","property":"contact","name":"contact","access":1}]`, string(exposesJSON))
}

func TestZigbee2MQTTBridgeDevices_ReportsWritableCapabilitiesViaAPI(t *testing.T) {
	port := reserveTCPPort(t)
	sensorName := fmt.Sprintf("office-plug-%d", port)
	fixture := setupZigbee2MQTTBridgeFixture(t, "integration-z2m-capabilities", sensorName)

	_, status := client.AddSensor(gen.Sensor{
		Name:         sensorName,
		SensorDriver: "mqtt-zigbee2mqtt",
		Config:       map[string]string{},
	})
	require.Equal(t, http.StatusCreated, status)

	mqttClient := fixture.newPublisher(t, fmt.Sprintf("integration-z2m-capabilities-publisher-%d", port))

	payload := fmt.Sprintf(`[
		{
			"ieee_address": "0x00158d0001826601",
			"friendly_name": %q,
			"definition": {
				"model": "TS011F",
				"vendor": "Tuya",
				"description": "Smart plug",
				"exposes": [
					{"type":"switch","features":[
						{"type":"binary","property":"state","name":"state","access":7,"value_on":"ON","value_off":"OFF"}
					]},
					{"type":"binary","property":"network_indicator","name":"network_indicator","access":7,"value_on":true,"value_off":false},
					{"type":"numeric","property":"power","name":"power","access":1,"unit":"W","value_min":0,"value_max":2500}
				]
			}
		}
	]`, sensorName)

	pub := mqttClient.Publish("zigbee2mqtt/bridge/devices", 0, false, payload)
	require.True(t, pub.WaitTimeout(5*time.Second))
	require.NoError(t, pub.Error())

	var sensor gen.Sensor
	require.Eventually(t, func() bool {
		var currentStatus int
		sensor, currentStatus = client.GetSensorByName(sensorName)
		if currentStatus != http.StatusOK || sensor.Metadata == nil || sensor.Capabilities == nil {
			return false
		}
		metadata := *sensor.Metadata
		if metadata["model"] != "TS011F" || len(*sensor.Capabilities) != 2 {
			return false
		}

		properties := make(map[string]bool, len(*sensor.Capabilities))
		for _, capability := range *sensor.Capabilities {
			properties[capability.Property] = true
		}

		return properties["state"] && properties["network_indicator"]
	}, 5*time.Second, 100*time.Millisecond)

	capabilities, status := client.GetSensorCapabilities(sensor.Id)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, capabilities, 2)
	assert.Equal(t, *sensor.Capabilities, capabilities)

	byProperty := make(map[string]gen.Capability, len(capabilities))
	for _, capability := range capabilities {
		byProperty[capability.Property] = capability
	}

	require.NotNil(t, byProperty["state"].ValueOn)
	require.NotNil(t, byProperty["state"].ValueOff)
	assert.Equal(t, "ON", *byProperty["state"].ValueOn)
	assert.Equal(t, "OFF", *byProperty["state"].ValueOff)
	require.NotNil(t, byProperty["network_indicator"].ValueOn)
	require.NotNil(t, byProperty["network_indicator"].ValueOff)
	assert.Equal(t, "true", *byProperty["network_indicator"].ValueOn)
	assert.Equal(t, "false", *byProperty["network_indicator"].ValueOff)
}

func TestZigbee2MQTTBridgeDevices_ReadOnlySensorsReturnEmptyCapabilities(t *testing.T) {
	port := reserveTCPPort(t)
	sensorName := fmt.Sprintf("temperature-probe-%d", port)
	fixture := setupZigbee2MQTTBridgeFixture(t, "integration-z2m-read-only", sensorName)

	_, status := client.AddSensor(gen.Sensor{
		Name:         sensorName,
		SensorDriver: "mqtt-zigbee2mqtt",
		Config:       map[string]string{},
	})
	require.Equal(t, http.StatusCreated, status)

	mqttClient := fixture.newPublisher(t, fmt.Sprintf("integration-z2m-read-only-publisher-%d", port))

	payload := fmt.Sprintf(`[
		{
			"ieee_address": "0x00158d0001826602",
			"friendly_name": %q,
			"definition": {
				"model": "WSDCGQ11LM",
				"vendor": "Aqara",
				"description": "Temperature sensor",
				"exposes": [
					{"type":"numeric","property":"temperature","name":"temperature","access":1,"unit":"°C","value_min":-40,"value_max":80}
				]
			}
		}
	]`, sensorName)

	pub := mqttClient.Publish("zigbee2mqtt/bridge/devices", 0, false, payload)
	require.True(t, pub.WaitTimeout(5*time.Second))
	require.NoError(t, pub.Error())

	var sensor gen.Sensor
	require.Eventually(t, func() bool {
		var currentStatus int
		sensor, currentStatus = client.GetSensorByName(sensorName)
		if currentStatus != http.StatusOK || sensor.Metadata == nil || sensor.Capabilities == nil {
			return false
		}
		metadata := *sensor.Metadata
		return metadata["model"] == "WSDCGQ11LM" && len(*sensor.Capabilities) == 0
	}, 5*time.Second, 100*time.Millisecond)

	capabilities, status := client.GetSensorCapabilities(sensor.Id)
	require.Equal(t, http.StatusOK, status)
	assert.Empty(t, capabilities)
	assert.Empty(t, *sensor.Capabilities)
}

func TestZigbee2MQTTBridgeDevices_RenamesPhantomIEEESensorInPlace(t *testing.T) {
	port := reserveTCPPort(t)
	friendlyName := fmt.Sprintf("front-door-renamed-%d", port)
	ieeeName := "0x00158d00018255df"
	fixture := setupZigbee2MQTTBridgeFixture(t, "integration-z2m-rename", friendlyName, ieeeName)

	require.NoError(t, fixture.sensorRepo.AddSensor(fixture.ctx, gen.Sensor{
		Name:         ieeeName,
		ExternalId:   &ieeeName,
		SensorDriver: "mqtt-zigbee2mqtt",
		Config:       map[string]string{},
		Status:       gen.SensorStatusPending,
	}))

	phantom, err := fixture.sensorRepo.GetSensorByName(fixture.ctx, ieeeName)
	require.NoError(t, err)

	mqttClient := fixture.newPublisher(t, fmt.Sprintf("integration-z2m-rename-publisher-%d", port))

	payload := fmt.Sprintf(`[
		{
			"ieee_address": %q,
			"friendly_name": %q,
			"definition": {
				"model": "MCCGQ11LM",
				"vendor": "Aqara",
				"description": "Door and window sensor",
				"exposes": [
					{"type": "binary", "property": "contact", "name": "contact", "access": 1}
				]
			}
		}
	]`, ieeeName, friendlyName)

	pub := mqttClient.Publish("zigbee2mqtt/bridge/devices", 0, false, payload)
	require.True(t, pub.WaitTimeout(5*time.Second))
	require.NoError(t, pub.Error())

	pub = mqttClient.Publish(fmt.Sprintf("zigbee2mqtt/%s", ieeeName), 0, false, `{"contact":false,"battery":95}`)
	require.True(t, pub.WaitTimeout(5*time.Second))
	require.NoError(t, pub.Error())

	var renamed gen.Sensor
	require.Eventually(t, func() bool {
		var currentStatus int
		renamed, currentStatus = client.GetSensorByName(friendlyName)
		if currentStatus != http.StatusOK || renamed.Metadata == nil {
			return false
		}
		metadata := *renamed.Metadata
		return renamed.Id == phantom.Id &&
			renamed.ExternalId != nil &&
			*renamed.ExternalId == ieeeName &&
			renamed.Status == gen.SensorStatusPending &&
			metadata["manufacturer"] == "Aqara" &&
			metadata["model"] == "MCCGQ11LM" &&
			metadata["description"] == "Door and window sensor" &&
			metadata["ieee_address"] == ieeeName
	}, 5*time.Second, 100*time.Millisecond)

	_, err = fixture.sensorRepo.GetSensorByName(fixture.ctx, ieeeName)
	assert.Error(t, err)
}

func TestZigbee2MQTTBridgeDevices_AutoDiscoversFriendlySensorFromIEEEWithMetadata(t *testing.T) {
	port := reserveTCPPort(t)
	friendlyName := fmt.Sprintf("front-door-new-%d", port)
	ieeeName := "0x00158d00018255aa"
	fixture := setupZigbee2MQTTBridgeFixture(t, "integration-z2m-autodiscover", friendlyName, ieeeName)
	mqttClient := fixture.newPublisher(t, fmt.Sprintf("integration-z2m-autodiscover-publisher-%d", port))

	payload := fmt.Sprintf(`[
		{
			"ieee_address": %q,
			"friendly_name": %q,
			"definition": {
				"model": "MCCGQ11LM",
				"vendor": "Aqara",
				"description": "Door and window sensor",
				"exposes": [
					{"type": "binary", "property": "contact", "name": "contact", "access": 1}
				]
			}
		}
	]`, ieeeName, friendlyName)

	pub := mqttClient.Publish("zigbee2mqtt/bridge/devices", 0, false, payload)
	require.True(t, pub.WaitTimeout(5*time.Second))
	require.NoError(t, pub.Error())

	pub = mqttClient.Publish(fmt.Sprintf("zigbee2mqtt/%s", ieeeName), 0, false, `{"contact":true,"battery":100}`)
	require.True(t, pub.WaitTimeout(5*time.Second))
	require.NoError(t, pub.Error())

	var sensor gen.Sensor
	require.Eventually(t, func() bool {
		var currentStatus int
		sensor, currentStatus = client.GetSensorByName(friendlyName)
		if currentStatus != http.StatusOK || sensor.Metadata == nil {
			return false
		}
		metadata := *sensor.Metadata
		return sensor.Status == gen.SensorStatusPending &&
			metadata["manufacturer"] == "Aqara" &&
			metadata["model"] == "MCCGQ11LM" &&
			metadata["description"] == "Door and window sensor" &&
			metadata["ieee_address"] == ieeeName
	}, 5*time.Second, 100*time.Millisecond)
}

func reserveTCPPort(t *testing.T) int {
	t.Helper()

	port, err := reserveTCPPortNumber()
	require.NoError(t, err)
	return port
}

func reserveTCPPortNumber() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port, nil
}
