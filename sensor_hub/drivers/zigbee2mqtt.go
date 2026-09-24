package drivers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"example/sensorHub/gen"
)

func init() {
	Register(&Zigbee2MQTTDriver{})
}

// fieldMapping maps a Zigbee2MQTT JSON field to a measurement type definition.
type fieldMapping struct {
	MeasurementType string
	DisplayName     string
	Unit            string
	Category        string // "numeric" or "binary"
}

// knownFields maps Zigbee2MQTT JSON field names to their measurement type
// definitions. Zigbee2MQTT normalises field names across hardware vendors,
// so "temperature" is always "temperature" regardless of the device model.
var knownFields = map[string]fieldMapping{
	"temperature":      {MeasurementType: "temperature", DisplayName: "Temperature", Unit: "°C", Category: "numeric"},
	"humidity":         {MeasurementType: "humidity", DisplayName: "Humidity", Unit: "%", Category: "numeric"},
	"pressure":         {MeasurementType: "pressure", DisplayName: "Pressure", Unit: "hPa", Category: "numeric"},
	"battery":          {MeasurementType: "battery", DisplayName: "Battery", Unit: "%", Category: "numeric"},
	"voltage":          {MeasurementType: "voltage", DisplayName: "Voltage", Unit: "V", Category: "numeric"},
	"linkquality":      {MeasurementType: "link_quality", DisplayName: "Link Quality", Unit: "lqi", Category: "numeric"},
	"illuminance":      {MeasurementType: "illuminance", DisplayName: "Illuminance", Unit: "lx", Category: "numeric"},
	"illuminance_lux":  {MeasurementType: "illuminance", DisplayName: "Illuminance", Unit: "lx", Category: "numeric"},
	"power":            {MeasurementType: "power", DisplayName: "Power", Unit: "W", Category: "numeric"},
	"energy":           {MeasurementType: "energy", DisplayName: "Energy", Unit: "kWh", Category: "numeric"},
	"energy_today":     {MeasurementType: "energy_today", DisplayName: "Energy Today", Unit: "kWh", Category: "numeric"},
	"energy_month":     {MeasurementType: "energy_month", DisplayName: "Energy Month", Unit: "kWh", Category: "numeric"},
	"energy_yesterday": {MeasurementType: "energy_yesterday", DisplayName: "Energy Yesterday", Unit: "kWh", Category: "numeric"},
	"current":          {MeasurementType: "current", DisplayName: "Current", Unit: "A", Category: "numeric"},
	"co2":              {MeasurementType: "co2", DisplayName: "CO₂", Unit: "ppm", Category: "numeric"},
	"voc":              {MeasurementType: "voc", DisplayName: "VOC", Unit: "ppb", Category: "numeric"},
	"formaldehyde":     {MeasurementType: "formaldehyde", DisplayName: "Formaldehyde", Unit: "mg/m³", Category: "numeric"},
	"pm25":             {MeasurementType: "pm25", DisplayName: "PM2.5", Unit: "µg/m³", Category: "numeric"},
	"soil_moisture":    {MeasurementType: "soil_moisture", DisplayName: "Soil Moisture", Unit: "%", Category: "numeric"},
	"occupancy":        {MeasurementType: "occupancy", DisplayName: "Occupancy", Unit: "", Category: "binary"},
	"contact":          {MeasurementType: "contact", DisplayName: "Contact", Unit: "", Category: "binary"},
	"water_leak":       {MeasurementType: "water_leak", DisplayName: "Water Leak", Unit: "", Category: "binary"},
	"smoke":            {MeasurementType: "smoke", DisplayName: "Smoke", Unit: "", Category: "binary"},
	"carbon_monoxide":  {MeasurementType: "carbon_monoxide", DisplayName: "Carbon Monoxide", Unit: "", Category: "binary"},
	"tamper":           {MeasurementType: "tamper", DisplayName: "Tamper", Unit: "", Category: "binary"},
	"battery_low":      {MeasurementType: "battery_low", DisplayName: "Battery Low", Unit: "", Category: "binary"},
	"vibration":        {MeasurementType: "vibration", DisplayName: "Vibration", Unit: "", Category: "binary"},
	"state":            {MeasurementType: "state", DisplayName: "State", Unit: "", Category: "binary"},
}

// Zigbee2MQTTDriver parses MQTT messages from a Zigbee2MQTT bridge.
//
// Zigbee2MQTT publishes to topics like: zigbee2mqtt/<device_friendly_name>
// Each message is a flat JSON object with normalised field names.
type Zigbee2MQTTDriver struct{}

var _ PushDriver = (*Zigbee2MQTTDriver)(nil)
var _ CommandDriver = (*Zigbee2MQTTDriver)(nil)
var _ SystemMessageHandler = (*Zigbee2MQTTDriver)(nil)

func (d *Zigbee2MQTTDriver) Type() string        { return "mqtt-zigbee2mqtt" }
func (d *Zigbee2MQTTDriver) DisplayName() string { return "Zigbee2MQTT" }
func (d *Zigbee2MQTTDriver) Description() string {
	return "Zigbee devices via the Zigbee2MQTT bridge — temperature, humidity, contact sensors, smart plugs, and more"
}

func (d *Zigbee2MQTTDriver) ConfigFields() []ConfigFieldSpec {
	return nil // Push drivers have no sensor-level config
}

func (d *Zigbee2MQTTDriver) SupportedMeasurementTypes() []gen.MeasurementType {
	var mts []gen.MeasurementType
	seen := make(map[string]bool)
	for _, fm := range knownFields {
		if seen[fm.MeasurementType] {
			continue
		}
		seen[fm.MeasurementType] = true
		mts = append(mts, gen.MeasurementType{
			Name:        fm.MeasurementType,
			DisplayName: fm.DisplayName,
			Unit:        fm.Unit,
			Category:    gen.MeasurementTypeCategory(fm.Category),
		})
	}
	return mts
}

func (d *Zigbee2MQTTDriver) ValidateSensor(_ context.Context, _ gen.Sensor) error {
	return nil // Push-based sensors have nothing to validate locally
}

// IdentifyDevice extracts the device name from a Zigbee2MQTT topic.
// Topics follow the pattern: <base_topic>/<friendly_name>
// For example: zigbee2mqtt/living-room-sensor → "living-room-sensor"
//
// System topics (bridge/..., bridge/log, bridge/state) are ignored.
func (d *Zigbee2MQTTDriver) IdentifyDevice(topic string, _ []byte) (string, error) {
	parts := strings.Split(topic, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid zigbee2mqtt topic: %s", topic)
	}

	deviceName := parts[len(parts)-1]

	// Skip system topics — anything under <base_topic>/bridge/...
	if len(parts) >= 2 && parts[1] == "bridge" {
		return "", fmt.Errorf("system topic, not a device: %s", topic)
	}

	// Skip sub-paths like zigbee2mqtt/device/set or zigbee2mqtt/device/get
	if deviceName == "set" || deviceName == "get" || deviceName == "availability" {
		return "", fmt.Errorf("command/availability topic, not a device message: %s", topic)
	}

	// For topics with more than 2 parts (e.g., zigbee2mqtt/room/sensor),
	// use the last segment after the base topic prefix
	if len(parts) > 2 {
		deviceName = strings.Join(parts[1:], "/")
		// But strip trailing /set, /get, /availability
		for _, suffix := range []string{"/set", "/get", "/availability"} {
			deviceName = strings.TrimSuffix(deviceName, suffix)
		}
	}

	return deviceName, nil
}

type zigbeeBridgeDevice struct {
	IEEEAddress  string `json:"ieee_address"`
	FriendlyName string `json:"friendly_name"`
	Definition   struct {
		Model       string          `json:"model"`
		Vendor      string          `json:"vendor"`
		Description string          `json:"description"`
		Exposes     json.RawMessage `json:"exposes"`
	} `json:"definition"`
}

func (d *Zigbee2MQTTDriver) ParseSystemMessage(topic string, payload []byte) []DeviceMetadata {
	if topic != "zigbee2mqtt/bridge/devices" {
		return nil
	}

	var devices []zigbeeBridgeDevice
	if err := json.Unmarshal(payload, &devices); err != nil {
		return nil
	}

	metadata := make([]DeviceMetadata, 0, len(devices))
	for _, device := range devices {
		metadata = append(metadata, DeviceMetadata{
			FriendlyName: device.FriendlyName,
			IEEEAddress:  device.IEEEAddress,
			Metadata: map[string]string{
				"manufacturer": device.Definition.Vendor,
				"model":        device.Definition.Model,
				"description":  device.Definition.Description,
			},
			Exposes: device.Definition.Exposes,
		})
	}

	return metadata
}

type zigbeeCapabilityExpose struct {
	Type     string                   `json:"type"`
	Property string                   `json:"property"`
	Access   *int                     `json:"access"`
	ValueOn  *string                  `json:"value_on"`
	ValueOff *string                  `json:"value_off"`
	ValueMin *float64                 `json:"value_min"`
	ValueMax *float64                 `json:"value_max"`
	Unit     *string                  `json:"unit"`
	Values   []string                 `json:"values"`
	Features []zigbeeCapabilityExpose `json:"features"`
}

func (d *Zigbee2MQTTDriver) ParseCapabilities(metadata json.RawMessage) []gen.Capability {
	var exposePayloads []json.RawMessage
	if err := json.Unmarshal(metadata, &exposePayloads); err != nil {
		return nil
	}

	exposes := make([]zigbeeCapabilityExpose, 0, len(exposePayloads))
	for _, exposePayload := range exposePayloads {
		expose, err := parseCapabilityExposeJSON(exposePayload)
		if err != nil {
			continue
		}
		exposes = append(exposes, expose)
	}

	if len(exposes) == 0 {
		return []gen.Capability{}
	}

	capabilities := make([]gen.Capability, 0)
	for _, expose := range exposes {
		capabilities = append(capabilities, parseCapabilityExpose(expose)...)
	}

	return capabilities
}

type zigbeeCapabilityExposePayload struct {
	Type     string            `json:"type"`
	Property string            `json:"property"`
	Access   *int              `json:"access"`
	ValueOn  json.RawMessage   `json:"value_on"`
	ValueOff json.RawMessage   `json:"value_off"`
	ValueMin *float64          `json:"value_min"`
	ValueMax *float64          `json:"value_max"`
	Unit     *string           `json:"unit"`
	Values   []string          `json:"values"`
	Features []json.RawMessage `json:"features"`
}

func parseCapabilityExposeJSON(payload json.RawMessage) (zigbeeCapabilityExpose, error) {
	var raw zigbeeCapabilityExposePayload
	if err := json.Unmarshal(payload, &raw); err != nil {
		return zigbeeCapabilityExpose{}, err
	}

	valueOn, err := parseCapabilityStringValue(raw.ValueOn)
	if err != nil {
		return zigbeeCapabilityExpose{}, err
	}

	valueOff, err := parseCapabilityStringValue(raw.ValueOff)
	if err != nil {
		return zigbeeCapabilityExpose{}, err
	}

	expose := zigbeeCapabilityExpose{
		Type:     raw.Type,
		Property: raw.Property,
		Access:   raw.Access,
		ValueOn:  valueOn,
		ValueOff: valueOff,
		ValueMin: raw.ValueMin,
		ValueMax: raw.ValueMax,
		Unit:     raw.Unit,
		Values:   raw.Values,
		Features: make([]zigbeeCapabilityExpose, 0, len(raw.Features)),
	}

	for _, featurePayload := range raw.Features {
		feature, err := parseCapabilityExposeJSON(featurePayload)
		if err != nil {
			continue
		}
		expose.Features = append(expose.Features, feature)
	}

	return expose, nil
}

func parseCapabilityStringValue(raw json.RawMessage) (*string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var stringValue string
	if err := json.Unmarshal(raw, &stringValue); err == nil {
		return &stringValue, nil
	}

	var boolValue bool
	if err := json.Unmarshal(raw, &boolValue); err == nil {
		value := strconv.FormatBool(boolValue)
		return &value, nil
	}

	return nil, fmt.Errorf("unsupported capability value type: %s", string(raw))
}

func (d *Zigbee2MQTTDriver) BuildCommand(sensor gen.Sensor, property string, value string) (string, []byte, error) {
	capabilities, err := d.capabilitiesForSensor(sensor)
	if err != nil {
		return "", nil, err
	}

	capability, ok := findCapability(capabilities, property)
	if !ok {
		return "", nil, fmt.Errorf("unknown command property %q", property)
	}

	typedValue, err := commandPayloadValue(capability, value)
	if err != nil {
		return "", nil, err
	}

	payload, err := json.Marshal(map[string]any{property: typedValue})
	if err != nil {
		return "", nil, fmt.Errorf("marshal command payload: %w", err)
	}

	return fmt.Sprintf("zigbee2mqtt/%s/set", sensor.Name), payload, nil
}

func (d *Zigbee2MQTTDriver) capabilitiesForSensor(sensor gen.Sensor) ([]gen.Capability, error) {
	if sensor.Metadata == nil {
		return nil, fmt.Errorf("sensor metadata is required for command building")
	}

	exposesValue, ok := (*sensor.Metadata)["exposes"]
	if !ok || exposesValue == nil {
		return nil, fmt.Errorf("sensor exposes metadata is required for command building")
	}

	exposesJSON, err := json.Marshal(exposesValue)
	if err != nil {
		return nil, fmt.Errorf("marshal sensor exposes metadata: %w", err)
	}

	capabilities := d.ParseCapabilities(exposesJSON)
	if len(capabilities) == 0 {
		return nil, fmt.Errorf("sensor has no writable capabilities")
	}

	return capabilities, nil
}

func findCapability(capabilities []gen.Capability, property string) (gen.Capability, bool) {
	for _, capability := range capabilities {
		if capability.Property == property {
			return capability, true
		}
	}
	return gen.Capability{}, false
}

func commandPayloadValue(capability gen.Capability, value string) (any, error) {
	switch capability.Type {
	case gen.CapabilityTypeBinary:
		return binaryCommandPayloadValue(capability, value)
	case gen.CapabilityTypeNumeric:
		return numericCommandPayloadValue(value)
	case gen.CapabilityTypeEnum:
		if capability.Values != nil {
			for _, allowed := range *capability.Values {
				if allowed == value {
					return value, nil
				}
			}
			return nil, fmt.Errorf("invalid enum value %q for property %q", value, capability.Property)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported capability type %q", capability.Type)
	}
}

func binaryCommandPayloadValue(capability gen.Capability, value string) (any, error) {
	valueOn := "ON"
	valueOff := "OFF"
	if capability.ValueOn != nil {
		valueOn = *capability.ValueOn
	}
	if capability.ValueOff != nil {
		valueOff = *capability.ValueOff
	}

	switch {
	case strings.EqualFold(value, valueOn):
		return canonicalBinaryPayloadValue(valueOn, true), nil
	case strings.EqualFold(value, valueOff):
		return canonicalBinaryPayloadValue(valueOff, false), nil
	default:
		return nil, fmt.Errorf("invalid binary value %q for property %q", value, capability.Property)
	}
}

func canonicalBinaryPayloadValue(canonical string, state bool) any {
	switch strings.ToLower(strings.TrimSpace(canonical)) {
	case "true", "false":
		return state
	default:
		return canonical
	}
}

func numericCommandPayloadValue(value string) (any, error) {
	if !strings.ContainsAny(value, ".eE") {
		if integerValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return integerValue, nil
		}
	}

	floatValue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid numeric value %q", value)
	}

	return floatValue, nil
}

func parseCapabilityExpose(expose zigbeeCapabilityExpose) []gen.Capability {
	capabilities := make([]gen.Capability, 0)
	for _, feature := range expose.Features {
		capabilities = append(capabilities, parseCapabilityExpose(feature)...)
	}

	if expose.Access == nil || *expose.Access&2 == 0 || expose.Property == "" {
		return capabilities
	}

	switch expose.Type {
	case "binary":
		valueOn := expose.ValueOn
		valueOff := expose.ValueOff
		if valueOn == nil {
			valueOn = capabilityStringPtr("ON")
		}
		if valueOff == nil {
			valueOff = capabilityStringPtr("OFF")
		}
		capabilities = append(capabilities, gen.Capability{
			Property: expose.Property,
			Type:     gen.CapabilityTypeBinary,
			ValueOn:  valueOn,
			ValueOff: valueOff,
		})
	case "numeric":
		capabilities = append(capabilities, gen.Capability{
			Property: expose.Property,
			Type:     gen.CapabilityTypeNumeric,
			Min:      expose.ValueMin,
			Max:      expose.ValueMax,
			Unit:     expose.Unit,
		})
	case "enum":
		capability := gen.Capability{
			Property: expose.Property,
			Type:     gen.CapabilityTypeEnum,
		}
		if len(expose.Values) > 0 {
			values := append([]string(nil), expose.Values...)
			capability.Values = &values
		}
		capabilities = append(capabilities, capability)
	}

	return capabilities
}

// ParseMessage extracts readings from a Zigbee2MQTT JSON payload.
// It maps known JSON fields to typed readings and ignores unknown fields.
func (d *Zigbee2MQTTDriver) ParseMessage(topic string, payload []byte) ([]gen.Reading, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("invalid JSON payload: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var readings []gen.Reading

	for key, val := range data {
		fm, ok := knownFields[key]
		if !ok {
			continue
		}

		reading := gen.Reading{
			MeasurementType: fm.MeasurementType,
			Unit:            fm.Unit,
			Time:            now,
		}

		switch fm.Category {
		case "numeric":
			numVal, ok := toFloat64(val)
			if !ok {
				continue
			}
			// Normalise millivolt battery readings to volts.
			// Battery devices report mV (e.g. 3100), mains devices report V
			// (e.g. 231.15). No battery goes below 500 mV; no mains exceeds 500 V.
			if fm.MeasurementType == "voltage" && numVal > 500 {
				numVal = numVal / 1000.0
			}
			reading.NumericValue = &numVal
		case "binary":
			textVal := toBoolString(val)
			reading.TextState = &textVal
		}

		readings = append(readings, reading)
	}

	return readings, nil
}

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func toBoolString(v interface{}) string {
	switch b := v.(type) {
	case bool:
		if b {
			return "true"
		}
		return "false"
	case string:
		lower := strings.ToLower(b)
		if lower == "on" || lower == "true" || lower == "1" {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func capabilityStringPtr(value string) *string {
	return &value
}
