//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	gen "example/sensorHub/gen"
	"example/sensorHub/testharness"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// MQTT Broker CRUD
// ============================================================================

func TestMQTTBroker_CreateAndList(t *testing.T) {
	broker := gen.MQTTBroker{
		Name:     "test-broker",
		Host:     "mqtt-test-host.example.com",
		Port:     1883,
		Type:     "external",
		ClientId: ptrStr("sensor-hub-test"),
		Enabled:  true,
	}
	resp, status := client.CreateMQTTBroker(broker)
	require.Equal(t, http.StatusCreated, status, "body: %s", string(resp))

	list, status := client.ListMQTTBrokers()
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(list), "test-broker")
}

func TestMQTTBroker_GetByID(t *testing.T) {
	// Create a broker to get
	broker := gen.MQTTBroker{
		Name:    "get-test-broker",
		Host:    "192.168.1.100",
		Port:    1883,
		Type:    "external",
		Enabled: false,
	}
	resp, status := client.CreateMQTTBroker(broker)
	require.Equal(t, http.StatusCreated, status)

	var created struct {
		ID int `json:"id"`
	}
	require.NoError(t, json.Unmarshal(resp, &created))

	detail, status := client.GetMQTTBroker(created.ID)
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(detail), "get-test-broker")
	assert.Contains(t, string(detail), "192.168.1.100")
}

func TestMQTTBroker_GetByID_NotFound(t *testing.T) {
	_, status := client.GetMQTTBroker(99999)
	assert.Equal(t, http.StatusNotFound, status)
}

func TestMQTTBroker_Update(t *testing.T) {
	broker := gen.MQTTBroker{
		Name:    "update-test-broker",
		Host:    "update-host.example.com",
		Port:    1883,
		Type:    "external",
		Enabled: true,
	}
	resp, status := client.CreateMQTTBroker(broker)
	require.Equal(t, http.StatusCreated, status)

	var created struct {
		ID int `json:"id"`
	}
	json.Unmarshal(resp, &created)

	updated := gen.MQTTBroker{
		Id:       &created.ID,
		Name:     "update-test-broker-renamed",
		Host:     "10.0.0.1",
		Port:     8883,
		Type:     "external",
		Enabled:  false,
		Username: ptrStr("mqttuser"),
	}
	_, status = client.UpdateMQTTBroker(created.ID, updated)
	require.Equal(t, http.StatusOK, status)

	detail, status := client.GetMQTTBroker(created.ID)
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(detail), "update-test-broker-renamed")
	assert.Contains(t, string(detail), "10.0.0.1")
}

func TestMQTTBroker_Delete(t *testing.T) {
	broker := gen.MQTTBroker{
		Name:    "delete-test-broker",
		Host:    "delete-host.example.com",
		Port:    1883,
		Type:    "external",
		Enabled: false,
	}
	resp, status := client.CreateMQTTBroker(broker)
	require.Equal(t, http.StatusCreated, status)

	var created struct {
		ID int `json:"id"`
	}
	json.Unmarshal(resp, &created)

	status = client.DeleteMQTTBroker(created.ID)
	assert.Equal(t, http.StatusNoContent, status)

	_, status = client.GetMQTTBroker(created.ID)
	assert.Equal(t, http.StatusNotFound, status)
}

func TestMQTTBroker_Delete_NotFound(t *testing.T) {
	status := client.DeleteMQTTBroker(99999)
	assert.Equal(t, http.StatusNotFound, status)
}

func TestMQTTBroker_Create_Validation(t *testing.T) {
	// Missing name
	broker := gen.MQTTBroker{
		Host: "localhost",
		Port: 1883,
		Type: "external",
	}
	_, status := client.CreateMQTTBroker(broker)
	assert.Equal(t, http.StatusBadRequest, status)
}

func TestMQTTBroker_DuplicateName(t *testing.T) {
	broker := gen.MQTTBroker{
		Name:    "duplicate-broker",
		Host:    "dup-host.example.com",
		Port:    1883,
		Type:    "external",
		Enabled: false,
	}
	_, status := client.CreateMQTTBroker(broker)
	require.Equal(t, http.StatusCreated, status)

	// Same name again
	_, status = client.CreateMQTTBroker(broker)
	assert.Equal(t, http.StatusConflict, status)
}

// ============================================================================
// MQTT Subscription CRUD
// ============================================================================

func TestMQTTSubscription_CreateAndList(t *testing.T) {
	// First create a broker to attach subscriptions to
	broker := gen.MQTTBroker{
		Name:    "sub-test-broker",
		Host:    "sub-test-host.example.com",
		Port:    1883,
		Type:    "external",
		Enabled: true,
	}
	resp, status := client.CreateMQTTBroker(broker)
	require.Equal(t, http.StatusCreated, status)

	var created struct {
		ID int `json:"id"`
	}
	json.Unmarshal(resp, &created)

	sub := gen.MQTTSubscription{
		BrokerId:     created.ID,
		TopicPattern: "zigbee2mqtt/#",
		DriverType:   "mqtt-zigbee2mqtt",
		Enabled:      true,
	}
	subResp, status := client.CreateMQTTSubscription(sub)
	require.Equal(t, http.StatusCreated, status, "body: %s", string(subResp))

	list, status := client.ListMQTTSubscriptions()
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(list), "zigbee2mqtt/#")
}

func TestMQTTSubscription_GetByID(t *testing.T) {
	// Create broker
	broker := gen.MQTTBroker{
		Name: "sub-get-broker", Host: "sub-get-host.example.com", Port: 1883, Type: "external",
	}
	bResp, _ := client.CreateMQTTBroker(broker)
	var b struct {
		ID int `json:"id"`
	}
	json.Unmarshal(bResp, &b)

	sub := gen.MQTTSubscription{
		BrokerId:     b.ID,
		TopicPattern: "rtl_433/#",
		DriverType:   "mqtt-zigbee2mqtt",
		Enabled:      false,
	}
	resp, status := client.CreateMQTTSubscription(sub)
	require.Equal(t, http.StatusCreated, status)

	var created struct {
		ID int `json:"id"`
	}
	json.Unmarshal(resp, &created)

	detail, status := client.GetMQTTSubscription(created.ID)
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(detail), "rtl_433/#")
}

func TestMQTTSubscription_Update(t *testing.T) {
	broker := gen.MQTTBroker{
		Name: "sub-update-broker", Host: "sub-update-host.example.com", Port: 1883, Type: "external",
	}
	bResp, _ := client.CreateMQTTBroker(broker)
	var b struct {
		ID int `json:"id"`
	}
	json.Unmarshal(bResp, &b)

	sub := gen.MQTTSubscription{
		BrokerId:     b.ID,
		TopicPattern: "old/topic/#",
		DriverType:   "mqtt-zigbee2mqtt",
		Enabled:      true,
	}
	resp, status := client.CreateMQTTSubscription(sub)
	require.Equal(t, http.StatusCreated, status)

	var created struct {
		ID int `json:"id"`
	}
	json.Unmarshal(resp, &created)

	updated := gen.MQTTSubscription{
		Id:           &created.ID,
		BrokerId:     b.ID,
		TopicPattern: "new/topic/#",
		DriverType:   "mqtt-zigbee2mqtt",
		Enabled:      false,
	}
	_, status = client.UpdateMQTTSubscription(created.ID, updated)
	require.Equal(t, http.StatusOK, status)

	detail, status := client.GetMQTTSubscription(created.ID)
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(detail), "new/topic/#")
}

func TestMQTTSubscription_Delete(t *testing.T) {
	broker := gen.MQTTBroker{
		Name: "sub-delete-broker", Host: "sub-delete-host.example.com", Port: 1883, Type: "external",
	}
	bResp, _ := client.CreateMQTTBroker(broker)
	var b struct {
		ID int `json:"id"`
	}
	json.Unmarshal(bResp, &b)

	sub := gen.MQTTSubscription{
		BrokerId:     b.ID,
		TopicPattern: "delete/me/#",
		DriverType:   "mqtt-zigbee2mqtt",
		Enabled:      false,
	}
	resp, status := client.CreateMQTTSubscription(sub)
	require.Equal(t, http.StatusCreated, status)

	var created struct {
		ID int `json:"id"`
	}
	json.Unmarshal(resp, &created)

	status = client.DeleteMQTTSubscription(created.ID)
	assert.Equal(t, http.StatusNoContent, status)

	_, status = client.GetMQTTSubscription(created.ID)
	assert.Equal(t, http.StatusNotFound, status)
}

func TestMQTTSubscription_Create_Validation(t *testing.T) {
	// Missing topic pattern
	sub := gen.MQTTSubscription{
		BrokerId:   1,
		DriverType: "mqtt-zigbee2mqtt",
	}
	_, status := client.CreateMQTTSubscription(sub)
	assert.Equal(t, http.StatusBadRequest, status)
}

// ============================================================================
// MQTT RBAC — viewer cannot manage
// ============================================================================

func TestMQTTBroker_ViewerCannotCreate(t *testing.T) {
	// Create a viewer user with the viewer role
	viewerClient := testharness.NewClient(t, env.ServerURL)
	_, status := client.CreateUser(gen.CreateUserRequest{
		Username: "mqtt-viewer",
		Password: "viewerpass123",
		Email:    ptrStr("mqttviewer@test.com"),
		Roles:    &[]string{"viewer"},
	})
	if status != http.StatusCreated {
		// User may already exist from another test
		t.Logf("user creation returned %d, trying login anyway", status)
	}
	viewerClient.Login("mqtt-viewer", "viewerpass123")
	viewerClient.ChangePassword("viewerpass123")

	// Viewer should be able to list but not create
	_, listStatus := viewerClient.ListMQTTBrokers()
	assert.Equal(t, http.StatusOK, listStatus)

	broker := gen.MQTTBroker{
		Name: "viewer-broker", Host: "viewer-host.example.com", Port: 1883, Type: "external",
	}
	_, createStatus := viewerClient.CreateMQTTBroker(broker)
	assert.Equal(t, http.StatusForbidden, createStatus)
}

// ============================================================================
// Sensor Status Flow
// ============================================================================

func TestSensorStatus_GetPendingSensors(t *testing.T) {
	_, status := client.GetSensorsByStatus("pending")
	require.Equal(t, http.StatusOK, status)
}

func TestSensorStatus_ApproveAndDismiss(t *testing.T) {
	// Add a sensor that starts as "active" (normal flow)
	sensor := gen.Sensor{
		Name:         "status-test-sensor",
		SensorDriver: "sensor-hub-http-temperature",
		Config:       map[string]string{"url": mockSensorURLs[0]},
	}
	_, status := client.AddSensor(sensor)
	require.Equal(t, http.StatusCreated, status)

	// Get the sensor to find its ID
	s, status := client.GetSensorByName("status-test-sensor")
	require.Equal(t, http.StatusOK, status)

	// Dismiss it
	_, status = client.DismissSensor(s.Id)
	require.Equal(t, http.StatusOK, status)

	// Verify it appears in dismissed list
	dismissed, status := client.GetSensorsByStatus("dismissed")
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(dismissed), "status-test-sensor")

	// Approve it back
	_, status = client.ApproveSensor(s.Id)
	require.Equal(t, http.StatusOK, status)

	// Verify it's active again
	updated, status := client.GetSensorByName("status-test-sensor")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, gen.SensorStatusActive, updated.Status)
}

func TestSensorStatus_ApproveNotFound(t *testing.T) {
	_, status := client.ApproveSensor(99999)
	assert.Equal(t, http.StatusInternalServerError, status)
}
