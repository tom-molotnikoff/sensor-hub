package mqtt

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"example/sensorHub/drivers"
	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Mocks
// ============================================================================

type MockSensorService struct {
	mock.Mock
}

func (m *MockSensorService) ServiceGetSensorByName(ctx context.Context, name string) (*gen.Sensor, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.Sensor), args.Error(1)
}

func (m *MockSensorService) ServiceSensorExists(ctx context.Context, name string) (bool, error) {
	args := m.Called(ctx, name)
	return args.Bool(0), args.Error(1)
}

func (m *MockSensorService) ServiceAddSensor(ctx context.Context, sensor gen.Sensor) error {
	return m.Called(ctx, sensor).Error(0)
}

func (m *MockSensorService) ServiceUpdateSensorHealthById(ctx context.Context, sensorId int, healthStatus gen.SensorHealthStatus, healthReason string) {
	m.Called(ctx, sensorId, healthStatus, healthReason)
}

func (m *MockSensorService) ServiceUpdateSensorById(ctx context.Context, sensor gen.Sensor, retentionHoursPresent bool) error {
	return m.Called(ctx, sensor, retentionHoursPresent).Error(0)
}
func (m *MockSensorService) ServiceDeleteSensorByName(ctx context.Context, name string) error {
	return m.Called(ctx, name).Error(0)
}
func (m *MockSensorService) ServiceGetAllSensors(ctx context.Context) ([]gen.Sensor, error) {
	args := m.Called(ctx)
	return args.Get(0).([]gen.Sensor), args.Error(1)
}
func (m *MockSensorService) ServiceGetSensorsByDriver(ctx context.Context, sensorDriver string) ([]gen.Sensor, error) {
	args := m.Called(ctx, sensorDriver)
	return args.Get(0).([]gen.Sensor), args.Error(1)
}
func (m *MockSensorService) ServiceGetSensorIdByName(ctx context.Context, name string) (int, error) {
	args := m.Called(ctx, name)
	return args.Int(0), args.Error(1)
}
func (m *MockSensorService) ServiceCollectAndStoreAllSensorReadings(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}
func (m *MockSensorService) ServiceCollectFromSensorByName(ctx context.Context, sensorName string) error {
	return m.Called(ctx, sensorName).Error(0)
}
func (m *MockSensorService) ServiceCollectReadingToValidateSensor(ctx context.Context, sensor gen.Sensor) error {
	return m.Called(ctx, sensor).Error(0)
}
func (m *MockSensorService) ServiceStartPeriodicSensorCollection(ctx context.Context) {
	m.Called(ctx)
}
func (m *MockSensorService) ServiceDiscoverSensors(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}
func (m *MockSensorService) ServiceValidateSensorConfig(ctx context.Context, sensor gen.Sensor) error {
	return m.Called(ctx, sensor).Error(0)
}
func (m *MockSensorService) ServiceSetEnabledSensorByName(ctx context.Context, name string, enabled bool) error {
	return m.Called(ctx, name, enabled).Error(0)
}
func (m *MockSensorService) ServiceGetSensorHealthHistoryByName(ctx context.Context, name string) ([]gen.SensorHealthHistory, error) {
	args := m.Called(ctx, name)
	return args.Get(0).([]gen.SensorHealthHistory), args.Error(1)
}
func (m *MockSensorService) ServiceGetTotalReadingsForEachSensor() gen.TotalReadingsSample {
	args := m.Called()
	return args.Get(0).(gen.TotalReadingsSample)
}
func (m *MockSensorService) ServiceGetSensorsByStatus(ctx context.Context, status string) ([]gen.Sensor, error) {
	args := m.Called(ctx, status)
	return args.Get(0).([]gen.Sensor), args.Error(1)
}
func (m *MockSensorService) ServiceApproveSensor(ctx context.Context, sensorId int) error {
	return m.Called(ctx, sensorId).Error(0)
}
func (m *MockSensorService) ServiceDismissSensor(ctx context.Context, sensorId int) error {
	return m.Called(ctx, sensorId).Error(0)
}
func (m *MockSensorService) ServiceProcessPushReadings(ctx context.Context, sensor gen.Sensor, readings []gen.Reading) error {
	return m.Called(ctx, sensor, readings).Error(0)
}
func (m *MockSensorService) ServiceGetMeasurementTypesForSensor(ctx context.Context, sensorId int) ([]gen.MeasurementType, error) {
	args := m.Called(ctx, sensorId)
	return args.Get(0).([]gen.MeasurementType), args.Error(1)
}
func (m *MockSensorService) ServiceGetAllMeasurementTypes(ctx context.Context) ([]gen.MeasurementType, error) {
	args := m.Called(ctx)
	return args.Get(0).([]gen.MeasurementType), args.Error(1)
}
func (m *MockSensorService) ServiceGetSensorByExternalId(ctx context.Context, externalId string) (*gen.Sensor, error) {
	args := m.Called(ctx, externalId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.Sensor), args.Error(1)
}
func (m *MockSensorService) ServiceGetSensorById(ctx context.Context, id int) (*gen.Sensor, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.Sensor), args.Error(1)
}
func (m *MockSensorService) ServiceGetSensorCapabilities(ctx context.Context, id int) ([]gen.Capability, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]gen.Capability), args.Error(1)
}
func (m *MockSensorService) ServiceSensorExistsByExternalId(ctx context.Context, externalId string) (bool, error) {
	args := m.Called(ctx, externalId)
	return args.Bool(0), args.Error(1)
}
func (m *MockSensorService) ServiceGetAllMeasurementTypesWithReadings(ctx context.Context) ([]gen.MeasurementType, error) {
	args := m.Called(ctx)
	return args.Get(0).([]gen.MeasurementType), args.Error(1)
}

type MockSubRepo struct {
	mock.Mock
}

func (m *MockSubRepo) Add(ctx context.Context, sub gen.MQTTSubscription) (int, error) {
	args := m.Called(ctx, sub)
	return args.Int(0), args.Error(1)
}
func (m *MockSubRepo) GetByID(ctx context.Context, id int) (*gen.MQTTSubscription, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.MQTTSubscription), args.Error(1)
}
func (m *MockSubRepo) GetAll(ctx context.Context) ([]gen.MQTTSubscription, error) {
	args := m.Called(ctx)
	return args.Get(0).([]gen.MQTTSubscription), args.Error(1)
}
func (m *MockSubRepo) GetByBrokerID(ctx context.Context, brokerID int) ([]gen.MQTTSubscription, error) {
	args := m.Called(ctx, brokerID)
	return args.Get(0).([]gen.MQTTSubscription), args.Error(1)
}
func (m *MockSubRepo) GetEnabledByBrokerID(ctx context.Context, brokerID int) ([]gen.MQTTSubscription, error) {
	args := m.Called(ctx, brokerID)
	return args.Get(0).([]gen.MQTTSubscription), args.Error(1)
}
func (m *MockSubRepo) GetEnabledByDriverType(ctx context.Context, driverType string) (*gen.MQTTSubscription, error) {
	args := m.Called(ctx, driverType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.MQTTSubscription), args.Error(1)
}
func (m *MockSubRepo) Update(ctx context.Context, sub gen.MQTTSubscription) error {
	return m.Called(ctx, sub).Error(0)
}
func (m *MockSubRepo) Delete(ctx context.Context, id int) error {
	return m.Called(ctx, id).Error(0)
}

type MockBrokerRepo struct {
	mock.Mock
}

func (m *MockBrokerRepo) Add(ctx context.Context, broker gen.MQTTBroker) (int, error) {
	args := m.Called(ctx, broker)
	return args.Int(0), args.Error(1)
}
func (m *MockBrokerRepo) GetByID(ctx context.Context, id int) (*gen.MQTTBroker, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.MQTTBroker), args.Error(1)
}
func (m *MockBrokerRepo) GetByName(ctx context.Context, name string) (*gen.MQTTBroker, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.MQTTBroker), args.Error(1)
}
func (m *MockBrokerRepo) GetAll(ctx context.Context) ([]gen.MQTTBroker, error) {
	args := m.Called(ctx)
	return args.Get(0).([]gen.MQTTBroker), args.Error(1)
}
func (m *MockBrokerRepo) GetEnabled(ctx context.Context) ([]gen.MQTTBroker, error) {
	args := m.Called(ctx)
	return args.Get(0).([]gen.MQTTBroker), args.Error(1)
}
func (m *MockBrokerRepo) Update(ctx context.Context, broker gen.MQTTBroker) error {
	return m.Called(ctx, broker).Error(0)
}
func (m *MockBrokerRepo) Delete(ctx context.Context, id int) error {
	return m.Called(ctx, id).Error(0)
}

// ============================================================================
// Stub PushDriver
// ============================================================================

type stubPushDriver struct{}

func (s *stubPushDriver) Type() string                                                { return "test-push-driver" }
func (s *stubPushDriver) DisplayName() string                                         { return "Test Push" }
func (s *stubPushDriver) Description() string                                         { return "test" }
func (s *stubPushDriver) ConfigFields() []drivers.ConfigFieldSpec                     { return nil }
func (s *stubPushDriver) SupportedMeasurementTypes() []gen.MeasurementType            { return nil }
func (s *stubPushDriver) ValidateSensor(ctx context.Context, sensor gen.Sensor) error { return nil }

func (s *stubPushDriver) ParseMessage(topic string, payload []byte) ([]gen.Reading, error) {
	val := 22.5
	return []gen.Reading{
		{NumericValue: &val, MeasurementType: "temperature", Unit: "°C"},
	}, nil
}

func (s *stubPushDriver) IdentifyDevice(topic string, payload []byte) (string, error) {
	if strings.HasPrefix(topic, "zigbee2mqtt/") && topic != "zigbee2mqtt/bridge/devices" {
		return strings.TrimPrefix(topic, "zigbee2mqtt/"), nil
	}
	return "mqtt-device-1", nil
}

func (s *stubPushDriver) ParseSystemMessage(topic string, payload []byte) []drivers.DeviceMetadata {
	if topic != "zigbee2mqtt/bridge/devices" {
		return nil
	}
	model := "MCCGQ11LM"
	if strings.Contains(string(payload), "broker-1") {
		model = "broker-1"
	}
	if strings.Contains(string(payload), "broker-2") {
		model = "broker-2"
	}
	return []drivers.DeviceMetadata{
		{
			FriendlyName: "front-door",
			IEEEAddress:  "0x00158d00018255df",
			Metadata: map[string]string{
				"manufacturer": "Aqara",
				"model":        model,
				"description":  "Door and window sensor",
			},
			Exposes: []byte(`[{"type":"binary","property":"contact","name":"contact","access":1}]`),
		},
	}
}

func init() {
	drivers.Register(&stubPushDriver{})
}

// ============================================================================
// Tests
// ============================================================================

func TestConnectionManager_HandleMessage_KnownSensor(t *testing.T) {
	mockSensor := &MockSensorService{}
	mockSub := &MockSubRepo{}
	mockBroker := &MockBrokerRepo{}

	cm := NewConnectionManager(mockSensor, mockSub, mockBroker, slog.Default())

	sensor := &gen.Sensor{Id: 1, Name: "mqtt-device-1", SensorDriver: "test-push-driver", Status: gen.SensorStatusActive, Enabled: true}
	mockSensor.On("ServiceGetSensorByExternalId", mock.Anything, "mqtt-device-1").Return(sensor, nil)
	mockSensor.On("ServiceProcessPushReadings", mock.Anything, *sensor, mock.AnythingOfType("[]gen.Reading")).Return(nil)

	cm.handleMessage(context.Background(), 1, "test-push-driver", "test/topic", []byte(`{}`))

	mockSensor.AssertExpectations(t)
}

func TestConnectionManager_Publish_ReturnsErrorWhenBrokerNotConnected(t *testing.T) {
	cm := NewConnectionManager(&MockSensorService{}, &MockSubRepo{}, &MockBrokerRepo{}, slog.Default())

	err := cm.Publish(42, "zigbee2mqtt/office-plug/set", []byte(`{"state":"ON"}`), 1)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "broker 42 is not connected")
}

func TestConnectionManager_HandleMessage_AutoDiscovery(t *testing.T) {
	mockSensor := &MockSensorService{}
	mockSub := &MockSubRepo{}
	mockBroker := &MockBrokerRepo{}

	cm := NewConnectionManager(mockSensor, mockSub, mockBroker, slog.Default())

	mockSensor.On("ServiceGetSensorByExternalId", mock.Anything, "mqtt-device-1").Return(nil, fmt.Errorf("not found"))
	mockSensor.On("ServiceGetSensorByName", mock.Anything, "mqtt-device-1").Return(nil, fmt.Errorf("not found"))
	mockSensor.On("ServiceSensorExistsByExternalId", mock.Anything, "mqtt-device-1").Return(false, nil)
	mockSensor.On("ServiceSensorExists", mock.Anything, "mqtt-device-1").Return(false, nil)
	mockSensor.On("ServiceAddSensor", mock.Anything, mock.MatchedBy(func(s gen.Sensor) bool {
		return s.Name == "mqtt-device-1" && s.Status == gen.SensorStatusPending && s.ExternalId != nil && *s.ExternalId == "mqtt-device-1"
	})).Return(nil)

	cm.handleMessage(context.Background(), 1, "test-push-driver", "test/topic", []byte(`{}`))

	mockSensor.AssertExpectations(t)
}

func TestConnectionManager_HandleMessage_InactiveSensor(t *testing.T) {
	mockSensor := &MockSensorService{}
	mockSub := &MockSubRepo{}
	mockBroker := &MockBrokerRepo{}

	cm := NewConnectionManager(mockSensor, mockSub, mockBroker, slog.Default())

	sensor := &gen.Sensor{Id: 2, Name: "mqtt-device-1", Status: gen.SensorStatusPending, Enabled: true}
	mockSensor.On("ServiceGetSensorByExternalId", mock.Anything, "mqtt-device-1").Return(sensor, nil)

	cm.handleMessage(context.Background(), 1, "test-push-driver", "test/topic", []byte(`{}`))

	// Should NOT call ServiceProcessPushReadings since sensor is pending
	mockSensor.AssertNotCalled(t, "ServiceProcessPushReadings")
}

func TestConnectionManager_HandleMessage_UnknownDriver(t *testing.T) {
	mockSensor := &MockSensorService{}
	mockSub := &MockSubRepo{}
	mockBroker := &MockBrokerRepo{}

	cm := NewConnectionManager(mockSensor, mockSub, mockBroker, slog.Default())

	cm.handleMessage(context.Background(), 1, "nonexistent-driver", "test/topic", []byte(`{}`))

	// No service calls should be made
	mockSensor.AssertNotCalled(t, "ServiceGetSensorByName")
}

func TestConnectionManager_HandleMessage_DisabledSensor(t *testing.T) {
	mockSensor := &MockSensorService{}
	mockSub := &MockSubRepo{}
	mockBroker := &MockBrokerRepo{}

	cm := NewConnectionManager(mockSensor, mockSub, mockBroker, slog.Default())

	sensor := &gen.Sensor{Id: 3, Name: "mqtt-device-1", Status: gen.SensorStatusActive, Enabled: false}
	mockSensor.On("ServiceGetSensorByExternalId", mock.Anything, "mqtt-device-1").Return(sensor, nil)

	cm.handleMessage(context.Background(), 1, "test-push-driver", "test/topic", []byte(`{}`))

	// Should NOT process readings for disabled sensors
	mockSensor.AssertNotCalled(t, "ServiceProcessPushReadings")
}

func TestConnectionManager_HandleMessage_SystemMessageBackfillsExistingSensors(t *testing.T) {
	mockSensor := &MockSensorService{}
	mockSub := &MockSubRepo{}
	mockBroker := &MockBrokerRepo{}

	cm := NewConnectionManager(mockSensor, mockSub, mockBroker, slog.Default())

	existing := gen.Sensor{
		Id:           42,
		Name:         "front-door",
		SensorDriver: "test-push-driver",
		Status:       gen.SensorStatusDismissed,
		Enabled:      false,
		Config:       map[string]string{},
	}

	mockSensor.On("ServiceGetSensorsByDriver", mock.Anything, "test-push-driver").Return([]gen.Sensor{existing}, nil)
	mockSensor.On("ServiceUpdateSensorById", mock.Anything, mock.MatchedBy(func(sensor gen.Sensor) bool {
		if sensor.Id != existing.Id || sensor.Name != existing.Name {
			return false
		}
		if sensor.Metadata == nil {
			return false
		}
		metadata := *sensor.Metadata
		return metadata["manufacturer"] == "Aqara" &&
			metadata["model"] == "MCCGQ11LM" &&
			metadata["description"] == "Door and window sensor" &&
			metadata["ieee_address"] == "0x00158d00018255df"
	}), false).Return(nil)

	cm.handleMessage(context.Background(), 7, "test-push-driver", "zigbee2mqtt/bridge/devices", []byte(`[]`))

	mockSensor.AssertExpectations(t)
	mockSensor.AssertNotCalled(t, "ServiceGetSensorByExternalId")
	mockSensor.AssertNotCalled(t, "ServiceProcessPushReadings")

	cachedFriendly, ok := cm.bridgeDevices.ieeeToFriendly[bridgeCacheKey{brokerID: 7, key: "0x00158d00018255df"}]
	assert.True(t, ok)
	assert.Equal(t, "front-door", cachedFriendly)

	cachedMetadata, ok := cm.bridgeDevices.deviceMetadata[bridgeCacheKey{brokerID: 7, key: "front-door"}]
	assert.True(t, ok)
	assert.Equal(t, "Aqara", cachedMetadata.Metadata["manufacturer"])
}

func TestConnectionManager_HandleMessage_SystemMessageCacheIsBrokerScoped(t *testing.T) {
	mockSensor := &MockSensorService{}
	mockSub := &MockSubRepo{}
	mockBroker := &MockBrokerRepo{}

	cm := NewConnectionManager(mockSensor, mockSub, mockBroker, slog.Default())

	mockSensor.On("ServiceGetSensorsByDriver", mock.Anything, "test-push-driver").Return([]gen.Sensor{}, nil).Twice()

	cm.handleMessage(context.Background(), 1, "test-push-driver", "zigbee2mqtt/bridge/devices", []byte(`broker-1`))
	cm.handleMessage(context.Background(), 2, "test-push-driver", "zigbee2mqtt/bridge/devices", []byte(`broker-2`))

	cachedOne, ok := cm.bridgeDevices.deviceMetadata[bridgeCacheKey{brokerID: 1, key: "front-door"}]
	assert.True(t, ok)
	assert.Equal(t, "broker-1", cachedOne.Metadata["model"])

	cachedTwo, ok := cm.bridgeDevices.deviceMetadata[bridgeCacheKey{brokerID: 2, key: "front-door"}]
	assert.True(t, ok)
	assert.Equal(t, "broker-2", cachedTwo.Metadata["model"])
}

func TestConnectionManager_HandleMessage_IEEECacheHitRoutesToFriendlySensor(t *testing.T) {
	mockSensor := &MockSensorService{}
	mockSub := &MockSubRepo{}
	mockBroker := &MockBrokerRepo{}

	cm := NewConnectionManager(mockSensor, mockSub, mockBroker, slog.Default())

	mockSensor.On("ServiceGetSensorsByDriver", mock.Anything, "test-push-driver").Return([]gen.Sensor{}, nil).Once()

	friendly := &gen.Sensor{
		Id:           11,
		Name:         "front-door",
		SensorDriver: "test-push-driver",
		Status:       gen.SensorStatusActive,
		Enabled:      true,
	}
	mockSensor.On("ServiceGetSensorByExternalId", mock.Anything, "front-door").Return(friendly, nil)
	mockSensor.On("ServiceGetSensorByExternalId", mock.Anything, "0x00158d00018255df").Return(nil, fmt.Errorf("not found"))
	mockSensor.On("ServiceGetSensorByName", mock.Anything, "0x00158d00018255df").Return(nil, fmt.Errorf("not found"))
	mockSensor.On("ServiceProcessPushReadings", mock.Anything, *friendly, mock.AnythingOfType("[]gen.Reading")).Return(nil)

	cm.handleMessage(context.Background(), 1, "test-push-driver", "zigbee2mqtt/bridge/devices", []byte(`[]`))
	cm.handleMessage(context.Background(), 1, "test-push-driver", "zigbee2mqtt/0x00158d00018255df", []byte(`{}`))

	mockSensor.AssertExpectations(t)
}

func TestConnectionManager_HandleMessage_IEEECacheMissFallsThroughUnchanged(t *testing.T) {
	mockSensor := &MockSensorService{}
	mockSub := &MockSubRepo{}
	mockBroker := &MockBrokerRepo{}

	cm := NewConnectionManager(mockSensor, mockSub, mockBroker, slog.Default())

	ieeeName := "0x00158d00018255df"
	mockSensor.On("ServiceGetSensorByExternalId", mock.Anything, ieeeName).Return(nil, fmt.Errorf("not found"))
	mockSensor.On("ServiceGetSensorByName", mock.Anything, ieeeName).Return(nil, fmt.Errorf("not found"))
	mockSensor.On("ServiceSensorExistsByExternalId", mock.Anything, ieeeName).Return(false, nil)
	mockSensor.On("ServiceSensorExists", mock.Anything, ieeeName).Return(false, nil)
	mockSensor.On("ServiceAddSensor", mock.Anything, mock.MatchedBy(func(sensor gen.Sensor) bool {
		return sensor.Name == ieeeName &&
			sensor.Status == gen.SensorStatusPending &&
			sensor.ExternalId != nil &&
			*sensor.ExternalId == ieeeName
	})).Return(nil)

	cm.handleMessage(context.Background(), 1, "test-push-driver", "zigbee2mqtt/0x00158d00018255df", []byte(`{}`))

	mockSensor.AssertExpectations(t)
}

func TestConnectionManager_HandleMessage_RenamesPhantomIEEESensorInPlace(t *testing.T) {
	mockSensor := &MockSensorService{}
	mockSub := &MockSubRepo{}
	mockBroker := &MockBrokerRepo{}

	cm := NewConnectionManager(mockSensor, mockSub, mockBroker, slog.Default())

	mockSensor.On("ServiceGetSensorsByDriver", mock.Anything, "test-push-driver").Return([]gen.Sensor{}, nil).Once()

	ieeeName := "0x00158d00018255df"
	phantom := &gen.Sensor{
		Id:           77,
		Name:         ieeeName,
		ExternalId:   &ieeeName,
		SensorDriver: "test-push-driver",
		Status:       gen.SensorStatusPending,
		Enabled:      false,
		Config:       map[string]string{},
	}

	mockSensor.On("ServiceGetSensorByExternalId", mock.Anything, "front-door").Return(nil, fmt.Errorf("not found"))
	mockSensor.On("ServiceGetSensorByName", mock.Anything, "front-door").Return(nil, fmt.Errorf("not found"))
	mockSensor.On("ServiceGetSensorByExternalId", mock.Anything, ieeeName).Return(phantom, nil)
	mockSensor.On("ServiceUpdateSensorById", mock.Anything, mock.MatchedBy(func(sensor gen.Sensor) bool {
		if sensor.Id != phantom.Id || sensor.Name != "front-door" || sensor.ExternalId == nil || *sensor.ExternalId != ieeeName {
			return false
		}
		if sensor.Metadata == nil {
			return false
		}
		metadata := *sensor.Metadata
		return metadata["manufacturer"] == "Aqara" &&
			metadata["model"] == "MCCGQ11LM" &&
			metadata["description"] == "Door and window sensor" &&
			metadata["ieee_address"] == ieeeName
	}), false).Return(nil)

	cm.handleMessage(context.Background(), 1, "test-push-driver", "zigbee2mqtt/bridge/devices", []byte(`[]`))
	cm.handleMessage(context.Background(), 1, "test-push-driver", "zigbee2mqtt/0x00158d00018255df", []byte(`{}`))

	mockSensor.AssertExpectations(t)
	mockSensor.AssertNotCalled(t, "ServiceAddSensor")
	mockSensor.AssertNotCalled(t, "ServiceProcessPushReadings")
}

func TestConnectionManager_HandleMessage_IEEECollisionLogsAndKeepsOriginalSensor(t *testing.T) {
	mockSensor := &MockSensorService{}
	mockSub := &MockSubRepo{}
	mockBroker := &MockBrokerRepo{}

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	cm := NewConnectionManager(mockSensor, mockSub, mockBroker, logger)

	mockSensor.On("ServiceGetSensorsByDriver", mock.Anything, "test-push-driver").Return([]gen.Sensor{}, nil).Once()

	ieeeName := "0x00158d00018255df"
	friendly := &gen.Sensor{
		Id:           11,
		Name:         "front-door",
		SensorDriver: "test-push-driver",
		Status:       gen.SensorStatusActive,
		Enabled:      true,
		Config:       map[string]string{},
	}
	phantom := &gen.Sensor{
		Id:           77,
		Name:         ieeeName,
		ExternalId:   &ieeeName,
		SensorDriver: "test-push-driver",
		Status:       gen.SensorStatusActive,
		Enabled:      true,
		Config:       map[string]string{},
	}

	mockSensor.On("ServiceGetSensorByExternalId", mock.Anything, "front-door").Return(friendly, nil)
	mockSensor.On("ServiceGetSensorByExternalId", mock.Anything, ieeeName).Return(phantom, nil)
	mockSensor.On("ServiceProcessPushReadings", mock.Anything, *phantom, mock.AnythingOfType("[]gen.Reading")).Return(nil)

	cm.handleMessage(context.Background(), 1, "test-push-driver", "zigbee2mqtt/bridge/devices", []byte(`[]`))
	cm.handleMessage(context.Background(), 1, "test-push-driver", "zigbee2mqtt/0x00158d00018255df", []byte(`{}`))

	mockSensor.AssertExpectations(t)
	mockSensor.AssertNotCalled(t, "ServiceUpdateSensorById")
	assert.Contains(t, logs.String(), "collision")
	assert.Contains(t, logs.String(), ieeeName)
}

func TestConnectionManager_IsConnected_NoConnection(t *testing.T) {
	cm := NewConnectionManager(nil, nil, nil, slog.Default())
	assert.False(t, cm.IsConnected(999))
}

func TestConnectionManager_ConnectedBrokerIDs_Empty(t *testing.T) {
	cm := NewConnectionManager(nil, nil, nil, slog.Default())
	assert.Empty(t, cm.ConnectedBrokerIDs())
}
