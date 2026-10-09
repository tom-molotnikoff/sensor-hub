package drivers

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZigbee2MQTT_Type(t *testing.T) {
	d := &Zigbee2MQTTDriver{}
	assert.Equal(t, "mqtt-zigbee2mqtt", d.Type())
}

func TestZigbee2MQTT_IdentifyDevice_Standard(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	name, err := d.IdentifyDevice("zigbee2mqtt/living-room-sensor", nil)
	require.NoError(t, err)
	assert.Equal(t, "living-room-sensor", name)
}

func TestZigbee2MQTT_IdentifyDevice_MultiSegment(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	name, err := d.IdentifyDevice("zigbee2mqtt/room/sensor", nil)
	require.NoError(t, err)
	assert.Equal(t, "room/sensor", name)
}

func TestZigbee2MQTT_IdentifyDevice_SkipBridge(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	// Direct bridge topics
	_, err := d.IdentifyDevice("zigbee2mqtt/bridge", nil)
	assert.Error(t, err)

	_, err = d.IdentifyDevice("zigbee2mqtt/bridge/state", nil)
	assert.Error(t, err)

	_, err = d.IdentifyDevice("zigbee2mqtt/bridge/log", nil)
	assert.Error(t, err)

	// Nested bridge topics (these slipped through before the fix)
	_, err = d.IdentifyDevice("zigbee2mqtt/bridge/response/permit_join", nil)
	assert.Error(t, err, "nested bridge/response topic should be filtered")

	_, err = d.IdentifyDevice("zigbee2mqtt/bridge/response/device/rename", nil)
	assert.Error(t, err, "deeply nested bridge topic should be filtered")

	_, err = d.IdentifyDevice("zigbee2mqtt/bridge/event", nil)
	assert.Error(t, err, "bridge/event should be filtered")

	_, err = d.IdentifyDevice("zigbee2mqtt/bridge/response/options", nil)
	assert.Error(t, err, "bridge/response/options should be filtered")
}

func TestZigbee2MQTT_IdentifyDevice_SkipCommandTopics(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	_, err := d.IdentifyDevice("zigbee2mqtt/device/set", nil)
	assert.Error(t, err)

	_, err = d.IdentifyDevice("zigbee2mqtt/device/get", nil)
	assert.Error(t, err)

	_, err = d.IdentifyDevice("zigbee2mqtt/device/availability", nil)
	assert.Error(t, err)
}

func TestZigbee2MQTT_IdentifyDevice_InvalidTopic(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	_, err := d.IdentifyDevice("singlesegment", nil)
	assert.Error(t, err)
}

func TestZigbee2MQTT_ParseMessage_TemperatureHumidity(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	payload := `{"temperature": 22.5, "humidity": 45.2, "battery": 87, "linkquality": 120}`
	readings, err := d.ParseMessage("zigbee2mqtt/sensor-1", []byte(payload))

	require.NoError(t, err)
	assert.Len(t, readings, 4)

	readingMap := make(map[string]float64)
	for _, r := range readings {
		if r.NumericValue != nil {
			readingMap[r.MeasurementType] = *r.NumericValue
		}
	}

	assert.InDelta(t, 22.5, readingMap["temperature"], 0.001)
	assert.InDelta(t, 45.2, readingMap["humidity"], 0.001)
	assert.InDelta(t, 87.0, readingMap["battery"], 0.001)
	assert.InDelta(t, 120.0, readingMap["link_quality"], 0.001)
}

func TestZigbee2MQTT_ParseMessage_BinarySensor(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	payload := `{"occupancy": true, "battery": 100, "linkquality": 80}`
	readings, err := d.ParseMessage("zigbee2mqtt/motion-1", []byte(payload))

	require.NoError(t, err)
	assert.Len(t, readings, 3)

	for _, r := range readings {
		if r.MeasurementType == "occupancy" {
			require.NotNil(t, r.TextState)
			assert.Equal(t, "true", *r.TextState)
		}
	}
}

func TestZigbee2MQTT_ParseMessage_ContactSensor(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	payload := `{"contact": false, "battery": 95}`
	readings, err := d.ParseMessage("zigbee2mqtt/door-1", []byte(payload))

	require.NoError(t, err)
	assert.Len(t, readings, 2)

	for _, r := range readings {
		if r.MeasurementType == "contact" {
			require.NotNil(t, r.TextState)
			assert.Equal(t, "false", *r.TextState)
		}
	}
}

func TestZigbee2MQTT_ParseMessage_BatteryLow(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	payload := `{"battery": 100, "battery_low": false, "contact": true, "linkquality": 204}`
	readings, err := d.ParseMessage("zigbee2mqtt/back-door-contact", []byte(payload))

	require.NoError(t, err)
	assert.Len(t, readings, 4)

	for _, r := range readings {
		if r.MeasurementType == "battery_low" {
			require.NotNil(t, r.TextState)
			assert.Equal(t, "false", *r.TextState)
			assert.Equal(t, "", r.Unit)
		}
	}
}

func TestZigbee2MQTT_ParseMessage_UnknownFieldsIgnored(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	payload := `{"temperature": 20.0, "unknown_field": "value", "another": 42}`
	readings, err := d.ParseMessage("zigbee2mqtt/sensor", []byte(payload))

	require.NoError(t, err)
	assert.Len(t, readings, 1)
	assert.Equal(t, "temperature", readings[0].MeasurementType)
}

func TestZigbee2MQTT_ParseMessage_InvalidJSON(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	_, err := d.ParseMessage("zigbee2mqtt/sensor", []byte(`not json`))
	assert.Error(t, err)
}

func TestZigbee2MQTT_ParseMessage_EmptyPayload(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	readings, err := d.ParseMessage("zigbee2mqtt/sensor", []byte(`{}`))
	require.NoError(t, err)
	assert.Empty(t, readings)
}

func TestZigbee2MQTT_ParseMessage_StringBoolValues(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	payload := `{"state": "ON"}`
	readings, err := d.ParseMessage("zigbee2mqtt/plug-1", []byte(payload))

	require.NoError(t, err)
	assert.Len(t, readings, 1)
	assert.Equal(t, "state", readings[0].MeasurementType)
	require.NotNil(t, readings[0].TextState)
	assert.Equal(t, "true", *readings[0].TextState)
}

func TestZigbee2MQTT_ParseMessage_SmartPlug(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	payload := `{"current":0.04,"energy":1.23,"energy_month":0.5,"energy_today":0.1,"energy_yesterday":0.4,"linkquality":255,"power":3.91,"state":"ON","voltage":231.15}`
	readings, err := d.ParseMessage("zigbee2mqtt/office-plug", []byte(payload))

	require.NoError(t, err)

	readingMap := make(map[string]gen.Reading)
	for _, r := range readings {
		readingMap[r.MeasurementType] = r
	}

	// Numeric fields
	assert.InDelta(t, 0.04, *readingMap["current"].NumericValue, 0.001)
	assert.InDelta(t, 1.23, *readingMap["energy"].NumericValue, 0.001)
	assert.InDelta(t, 0.5, *readingMap["energy_month"].NumericValue, 0.001)
	assert.InDelta(t, 0.1, *readingMap["energy_today"].NumericValue, 0.001)
	assert.InDelta(t, 0.4, *readingMap["energy_yesterday"].NumericValue, 0.001)
	assert.InDelta(t, 3.91, *readingMap["power"].NumericValue, 0.001)
	assert.InDelta(t, 255.0, *readingMap["link_quality"].NumericValue, 0.001)

	// Mains voltage stays as-is (231.15 V < 500 threshold)
	assert.InDelta(t, 231.15, *readingMap["voltage"].NumericValue, 0.001)
	assert.Equal(t, "V", readingMap["voltage"].Unit)

	// State is binary
	assert.Equal(t, "true", *readingMap["state"].TextState)
}

func TestZigbee2MQTT_ParseMessage_VoltageConversion(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	// Battery voltage in mV (>500) should be converted to V
	payload := `{"voltage": 3100, "battery": 95}`
	readings, err := d.ParseMessage("zigbee2mqtt/contact-sensor", []byte(payload))

	require.NoError(t, err)

	for _, r := range readings {
		if r.MeasurementType == "voltage" {
			require.NotNil(t, r.NumericValue)
			assert.InDelta(t, 3.1, *r.NumericValue, 0.001, "3100 mV should be converted to 3.1 V")
			assert.Equal(t, "V", r.Unit)
		}
	}

	// Mains voltage in V (<500) should stay as-is
	payload = `{"voltage": 231.72, "power": 0}`
	readings, err = d.ParseMessage("zigbee2mqtt/plug", []byte(payload))

	require.NoError(t, err)

	for _, r := range readings {
		if r.MeasurementType == "voltage" {
			require.NotNil(t, r.NumericValue)
			assert.InDelta(t, 231.72, *r.NumericValue, 0.001, "mains voltage should not be converted")
			assert.Equal(t, "V", r.Unit)
		}
	}
}

func TestZigbee2MQTT_ValidateSensor(t *testing.T) {
	d := &Zigbee2MQTTDriver{}
	err := d.ValidateSensor(context.Background(), gen.Sensor{})
	assert.NoError(t, err)
}

func TestZigbee2MQTT_ParseSystemMessage_BridgeDevices(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	payload := `[
		{
			"ieee_address": "0x00158d00018255df",
			"friendly_name": "front-door",
			"definition": {
				"model": "MCCGQ11LM",
				"vendor": "Aqara",
				"description": "Door and window sensor",
				"exposes": [
					{"type": "binary", "property": "contact", "name": "contact", "access": 1}
				]
			}
		}
	]`

	entries := d.ParseSystemMessage("zigbee2mqtt/bridge/devices", []byte(payload))

	require.Len(t, entries, 1)
	assert.Equal(t, "front-door", entries[0].FriendlyName)
	assert.Equal(t, "0x00158d00018255df", entries[0].IEEEAddress)
	assert.Equal(t, map[string]string{
		"manufacturer": "Aqara",
		"model":        "MCCGQ11LM",
		"description":  "Door and window sensor",
	}, entries[0].Metadata)
	assert.JSONEq(t, `[{"type":"binary","property":"contact","name":"contact","access":1}]`, string(entries[0].Exposes))
}

func TestZigbee2MQTT_ParseSystemMessage_NonSystemTopic(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	entries := d.ParseSystemMessage("zigbee2mqtt/front-door", []byte(`{"contact":true}`))

	assert.Nil(t, entries)
}

func TestZigbee2MQTT_ParseSystemMessage_MalformedPayload(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	entries := d.ParseSystemMessage("zigbee2mqtt/bridge/devices", []byte(`{not-json`))

	assert.Nil(t, entries)
}

func TestParseCapabilities_BinarySwitch(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	capabilities := d.ParseCapabilities(json.RawMessage(`[
		{"type":"binary","property":"state","name":"state","access":7,"value_on":"ON","value_off":"OFF"}
	]`))

	require.Len(t, capabilities, 1)
	assert.Equal(t, gen.Capability{
		Property: "state",
		Type:     gen.CapabilityTypeBinary,
		ValueOn:  strPtr("ON"),
		ValueOff: strPtr("OFF"),
	}, capabilities[0])
}

func TestParseCapabilities_BooleanBinaryValues(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	capabilities := d.ParseCapabilities(json.RawMessage(`[
		{"type":"switch","features":[
			{"type":"binary","property":"state","name":"state","access":7,"value_on":true,"value_off":false}
		]}
	]`))

	assert.Equal(t, []gen.Capability{{
		Property: "state",
		Type:     gen.CapabilityTypeBinary,
		ValueOn:  strPtr("true"),
		ValueOff: strPtr("false"),
	}}, capabilities)
}

// The exposes of an IKEA LED2103G5 bulb. Only state, brightness and
// color_temp are both published and settable and read back as readings of the
// same name.
func TestParseCapabilities_OffersOnlyPropertiesItCanConfirm(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	capabilities := d.ParseCapabilities(json.RawMessage(`[
		{"type":"light","features":[
			{"type":"binary","property":"state","name":"state","access":7,"value_on":"ON","value_off":"OFF","value_toggle":"TOGGLE"},
			{"type":"numeric","property":"brightness","name":"brightness","access":7,"value_min":0,"value_max":254},
			{"type":"numeric","property":"color_temp","name":"color_temp","access":7,"unit":"mired","value_min":250,"value_max":454},
			{"type":"composite","property":"level_config","name":"level_config","access":7,"features":[
				{"type":"binary","property":"execute_if_off","name":"execute_if_off","access":7,"value_on":true,"value_off":false},
				{"type":"numeric","property":"brightness","name":"brightness","access":7,"value_min":0,"value_max":254}
			]}
		]},
		{"type":"enum","property":"effect","name":"effect","access":2,"values":["blink","breathe","okay"]},
		{"type":"enum","property":"identify","name":"identify","access":2,"values":["identify"]},
		{"type":"enum","property":"power_on_behavior","name":"power_on_behavior","access":7,"values":["off","on","toggle","previous"]},
		{"type":"binary","property":"state","name":"state","access":2,"value_on":"ON","value_off":"OFF"},
		{"type":"binary","property":"brightness","name":"brightness","access":7,"value_on":"ON","value_off":"OFF"},
		{"type":"numeric","property":"linkquality","name":"linkquality","access":1,"unit":"lqi","value_min":0,"value_max":255}
	]`))

	assert.Equal(t, []gen.Capability{
		{Property: "state", Type: gen.CapabilityTypeBinary, ValueOn: strPtr("ON"), ValueOff: strPtr("OFF")},
		{Property: "brightness", Type: gen.CapabilityTypeNumeric, Min: floatPtr(0), Max: floatPtr(254)},
		{Property: "color_temp", Type: gen.CapabilityTypeNumeric, Min: floatPtr(250), Max: floatPtr(454), Unit: strPtr("mired")},
	}, capabilities)
}

// A cover's state is an enum (OPEN, CLOSE, STOP) and the hub reads state only
// as binary, so it can't confirm a cover command yet (#545).
func TestParseCapabilities_DoesNotOfferACoversEnumState(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	capabilities := d.ParseCapabilities(json.RawMessage(`[
		{"type":"cover","features":[
			{"type":"enum","property":"state","name":"state","access":3,"values":["OPEN","CLOSE","STOP"]},
			{"type":"numeric","property":"position","name":"position","access":7,"unit":"%","value_min":0,"value_max":100}
		]}
	]`))

	assert.Empty(t, capabilities)
}

// A command is acknowledged by the next reading named after its property, so
// every property offered must come back from ParseMessage under that name.
func TestParseCapabilities_EveryOfferedPropertyIsReadBackUnderItsOwnName(t *testing.T) {
	d := &Zigbee2MQTTDriver{}
	exposes := make([]map[string]any, 0, 2*len(knownFields))
	for field := range knownFields {
		for _, kind := range []string{"binary", "numeric"} {
			exposes = append(exposes, map[string]any{"type": kind, "property": field, "access": 7})
		}
	}
	exposesJSON, err := json.Marshal(exposes)
	require.NoError(t, err)

	capabilities := d.ParseCapabilities(exposesJSON)

	require.NotEmpty(t, capabilities)
	for _, capability := range capabilities {
		sample := map[gen.CapabilityType]string{gen.CapabilityTypeBinary: `"ON"`, gen.CapabilityTypeNumeric: `42`}[capability.Type]
		readings, err := d.ParseMessage("zigbee2mqtt/device", []byte(fmt.Sprintf(`{%q:%s}`, capability.Property, sample)))
		require.NoError(t, err)
		if assert.Len(t, readings, 1, capability.Property) {
			assert.Equal(t, capability.Property, readings[0].MeasurementType)
			if capability.Type == gen.CapabilityTypeNumeric {
				assert.NotNil(t, readings[0].NumericValue, capability.Property)
			} else {
				assert.NotNil(t, readings[0].TextState, capability.Property)
			}
		}
	}
}

func TestParseCapabilities_NoWritableFeatures(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	capabilities := d.ParseCapabilities(json.RawMessage(`[
		{"type":"binary","property":"contact","name":"contact","access":1},
		{"type":"numeric","property":"temperature","name":"temperature","access":1,"unit":"°C","value_min":-40,"value_max":80}
	]`))

	assert.Empty(t, capabilities)
}

func TestParseCapabilities_MixedFeatures(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	capabilities := d.ParseCapabilities(json.RawMessage(`[
		{"type":"binary","property":"state","name":"state","access":7,"value_on":"ON","value_off":"OFF"},
		{"type":"numeric","property":"brightness","name":"brightness","access":7,"unit":"%","value_min":0,"value_max":100},
		{"type":"numeric","property":"power","name":"power","access":1,"unit":"W","value_min":0,"value_max":2500}
	]`))

	assert.Equal(t, []gen.Capability{
		{Property: "state", Type: gen.CapabilityTypeBinary, ValueOn: strPtr("ON"), ValueOff: strPtr("OFF")},
		{Property: "brightness", Type: gen.CapabilityTypeNumeric, Min: floatPtr(0), Max: floatPtr(100), Unit: strPtr("%")},
	}, capabilities)
}

func TestParseCapabilities_MalformedExposes(t *testing.T) {
	d := &Zigbee2MQTTDriver{}

	capabilities := d.ParseCapabilities(json.RawMessage(`{not-json`))

	assert.Nil(t, capabilities)
}

func TestBuildCommand_State(t *testing.T) {
	d := &Zigbee2MQTTDriver{}
	sensor := gen.Sensor{
		Name: "office-plug",
		Metadata: &map[string]interface{}{
			"exposes": []interface{}{
				map[string]interface{}{
					"type":      "binary",
					"property":  "state",
					"access":    float64(7),
					"value_on":  "ON",
					"value_off": "OFF",
				},
			},
		},
	}

	topic, payload, err := d.BuildCommand(sensor, "state", "ON")

	require.NoError(t, err)
	assert.Equal(t, "zigbee2mqtt/office-plug/set", topic)
	assert.JSONEq(t, `{"state":"ON"}`, string(payload))
}

func TestBuildCommand_UnknownProperty(t *testing.T) {
	d := &Zigbee2MQTTDriver{}
	sensor := gen.Sensor{
		Name: "office-plug",
		Metadata: &map[string]interface{}{
			"exposes": []interface{}{
				map[string]interface{}{
					"type":      "binary",
					"property":  "state",
					"access":    float64(7),
					"value_on":  "ON",
					"value_off": "OFF",
				},
			},
		},
	}

	_, _, err := d.BuildCommand(sensor, "brightness", "100")

	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown command property "brightness"`)
}

func TestBuildCommand_NumericCapability(t *testing.T) {
	d := &Zigbee2MQTTDriver{}
	sensor := gen.Sensor{
		Name: "office-light",
		Metadata: &map[string]interface{}{
			"exposes": []interface{}{
				map[string]interface{}{
					"type":      "numeric",
					"property":  "brightness",
					"access":    float64(7),
					"value_min": float64(0),
					"value_max": float64(100),
				},
			},
		},
	}

	topic, payload, err := d.BuildCommand(sensor, "brightness", "42")

	require.NoError(t, err)
	assert.Equal(t, "zigbee2mqtt/office-light/set", topic)
	assert.JSONEq(t, `{"brightness":42}`, string(payload))
}

func TestBuildCommand_BooleanBinaryCapability(t *testing.T) {
	d := &Zigbee2MQTTDriver{}
	sensor := gen.Sensor{
		Name: "hallway-light",
		Metadata: &map[string]interface{}{
			"exposes": []interface{}{
				map[string]interface{}{
					"type":      "binary",
					"property":  "state",
					"access":    float64(7),
					"value_on":  "true",
					"value_off": "false",
				},
			},
		},
	}

	_, payload, err := d.BuildCommand(sensor, "state", "true")

	require.NoError(t, err)
	assert.JSONEq(t, `{"state":true}`, string(payload))
}

func TestBuildCommand_RefusesAPropertyItCannotConfirm(t *testing.T) {
	d := &Zigbee2MQTTDriver{}
	sensor := gen.Sensor{
		Name: "office-plug",
		Metadata: &map[string]interface{}{
			"exposes": []interface{}{
				map[string]interface{}{
					"type":      "binary",
					"property":  "state",
					"access":    float64(7),
					"value_on":  "ON",
					"value_off": "OFF",
				},
				map[string]interface{}{
					"type":      "binary",
					"property":  "network_indicator",
					"access":    float64(7),
					"value_on":  true,
					"value_off": false,
				},
			},
		},
	}

	_, _, err := d.BuildCommand(sensor, "network_indicator", "true")

	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown command property "network_indicator"`)
}

func TestZigbee2MQTT_SupportedMeasurementTypes(t *testing.T) {
	d := &Zigbee2MQTTDriver{}
	mts := d.SupportedMeasurementTypes()
	assert.NotEmpty(t, mts)

	names := make(map[string]bool)
	for _, mt := range mts {
		names[mt.Name] = true
	}
	assert.True(t, names["temperature"])
	assert.True(t, names["humidity"])
	assert.True(t, names["occupancy"])
}

func TestZigbee2MQTT_ImplementsPushDriver(t *testing.T) {
	var drv SensorDriver = &Zigbee2MQTTDriver{}
	_, ok := drv.(PushDriver)
	assert.True(t, ok, "Zigbee2MQTTDriver should implement PushDriver")
}

func strPtr(value string) *string {
	return &value
}

func floatPtr(value float64) *float64 {
	return &value
}
