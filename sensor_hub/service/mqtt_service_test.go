package service

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"example/sensorHub/drivers"
	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ============================================================================
// Mock repositories
// ============================================================================

type MockMQTTBrokerRepo struct{ mock.Mock }

func (m *MockMQTTBrokerRepo) Add(ctx context.Context, broker gen.MQTTBroker) (int, error) {
	args := m.Called(ctx, broker)
	return args.Int(0), args.Error(1)
}
func (m *MockMQTTBrokerRepo) GetByID(ctx context.Context, id int) (*gen.MQTTBroker, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.MQTTBroker), args.Error(1)
}
func (m *MockMQTTBrokerRepo) GetByName(ctx context.Context, name string) (*gen.MQTTBroker, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.MQTTBroker), args.Error(1)
}
func (m *MockMQTTBrokerRepo) GetAll(ctx context.Context) ([]gen.MQTTBroker, error) {
	args := m.Called(ctx)
	return args.Get(0).([]gen.MQTTBroker), args.Error(1)
}
func (m *MockMQTTBrokerRepo) GetEnabled(ctx context.Context) ([]gen.MQTTBroker, error) {
	args := m.Called(ctx)
	return args.Get(0).([]gen.MQTTBroker), args.Error(1)
}
func (m *MockMQTTBrokerRepo) Update(ctx context.Context, broker gen.MQTTBroker) error {
	return m.Called(ctx, broker).Error(0)
}
func (m *MockMQTTBrokerRepo) Delete(ctx context.Context, id int) error {
	return m.Called(ctx, id).Error(0)
}

type MockMQTTSubRepo struct{ mock.Mock }

func (m *MockMQTTSubRepo) Add(ctx context.Context, sub gen.MQTTSubscription) (int, error) {
	args := m.Called(ctx, sub)
	return args.Int(0), args.Error(1)
}
func (m *MockMQTTSubRepo) GetByID(ctx context.Context, id int) (*gen.MQTTSubscription, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.MQTTSubscription), args.Error(1)
}
func (m *MockMQTTSubRepo) GetAll(ctx context.Context) ([]gen.MQTTSubscription, error) {
	args := m.Called(ctx)
	return args.Get(0).([]gen.MQTTSubscription), args.Error(1)
}
func (m *MockMQTTSubRepo) GetByBrokerID(ctx context.Context, brokerID int) ([]gen.MQTTSubscription, error) {
	args := m.Called(ctx, brokerID)
	return args.Get(0).([]gen.MQTTSubscription), args.Error(1)
}
func (m *MockMQTTSubRepo) GetEnabledByBrokerID(ctx context.Context, brokerID int) ([]gen.MQTTSubscription, error) {
	args := m.Called(ctx, brokerID)
	return args.Get(0).([]gen.MQTTSubscription), args.Error(1)
}
func (m *MockMQTTSubRepo) GetEnabledByDriverType(ctx context.Context, driverType string) (*gen.MQTTSubscription, error) {
	args := m.Called(ctx, driverType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.MQTTSubscription), args.Error(1)
}
func (m *MockMQTTSubRepo) Update(ctx context.Context, sub gen.MQTTSubscription) error {
	return m.Called(ctx, sub).Error(0)
}
func (m *MockMQTTSubRepo) Delete(ctx context.Context, id int) error {
	return m.Called(ctx, id).Error(0)
}

// MockSubscriptionNotifier tracks calls to OnSubscriptionAdded/Removed.
type MockSubscriptionNotifier struct{ mock.Mock }

func (m *MockSubscriptionNotifier) OnSubscriptionAdded(sub gen.MQTTSubscription) {
	m.Called(sub)
}
func (m *MockSubscriptionNotifier) OnSubscriptionRemoved(sub gen.MQTTSubscription) {
	m.Called(sub)
}

// ============================================================================
// Stub PushDriver for subscription validation tests
// ============================================================================

func init() {
	drivers.Register(&stubPushDriver{})
	drivers.Register(&stubNonPushDriver{})
}

type stubPushDriver struct{}

func (d *stubPushDriver) Type() string                                           { return "mqtt-test-driver" }
func (d *stubPushDriver) DisplayName() string                                    { return "Test MQTT Driver" }
func (d *stubPushDriver) Description() string                                    { return "A stub MQTT push driver" }
func (d *stubPushDriver) ConfigFields() []drivers.ConfigFieldSpec                { return nil }
func (d *stubPushDriver) SupportedMeasurementTypes() []gen.MeasurementType       { return nil }
func (d *stubPushDriver) ValidateSensor(_ context.Context, _ gen.Sensor) error   { return nil }
func (d *stubPushDriver) ParseMessage(_ string, _ []byte) ([]gen.Reading, error) { return nil, nil }
func (d *stubPushDriver) IdentifyDevice(_ string, _ []byte) (string, error)      { return "", nil }

func setupMQTTService() (*MQTTService, *MockMQTTBrokerRepo, *MockMQTTSubRepo) {
	brokerRepo := new(MockMQTTBrokerRepo)
	subRepo := new(MockMQTTSubRepo)
	svc := NewMQTTService(brokerRepo, subRepo, slog.Default())
	return svc, brokerRepo, subRepo
}

// ============================================================================
// Broker tests
// ============================================================================

func TestMQTTService_AddBroker_Success(t *testing.T) {
	svc, brokerRepo, _ := setupMQTTService()

	broker := gen.MQTTBroker{Name: "test", Type: "external", Host: "mqtt.example.com", Port: 1883, Enabled: true}
	brokerRepo.On("GetByName", mock.Anything, "test").Return(nil, nil)
	brokerRepo.On("GetAll", mock.Anything).Return([]gen.MQTTBroker{}, nil)
	brokerRepo.On("Add", mock.Anything, broker).Return(1, nil)

	id, err := svc.AddBroker(context.Background(), broker)
	assert.NoError(t, err)
	assert.Equal(t, 1, id)
	brokerRepo.AssertExpectations(t)
}

func TestMQTTService_AddBroker_ValidationFails(t *testing.T) {
	svc, _, _ := setupMQTTService()

	tests := []struct {
		name   string
		broker gen.MQTTBroker
		errMsg string
	}{
		{"empty name", gen.MQTTBroker{Type: "external", Host: "h", Port: 1883}, "broker name cannot be empty"},
		{"empty host", gen.MQTTBroker{Name: "n", Type: "external", Port: 1883}, "broker host cannot be empty"},
		{"invalid port", gen.MQTTBroker{Name: "n", Type: "external", Host: "h", Port: 0}, "broker port must be between"},
		{"invalid type", gen.MQTTBroker{Name: "n", Type: "invalid", Host: "h", Port: 1883}, "broker type must be"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.AddBroker(context.Background(), tt.broker)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestMQTTService_AddBroker_EmbeddedSuccess(t *testing.T) {
	svc, brokerRepo, _ := setupMQTTService()

	brokerRepo.On("GetAll", mock.Anything).Return([]gen.MQTTBroker{}, nil)
	brokerRepo.On("GetByName", mock.Anything, "emb").Return(nil, nil)
	// normaliseEmbeddedBroker sets host to "localhost"
	expected := gen.MQTTBroker{Name: "emb", Type: "embedded", Host: "localhost", Port: 1883, Enabled: true}
	brokerRepo.On("Add", mock.Anything, expected).Return(1, nil)

	id, err := svc.AddBroker(context.Background(), gen.MQTTBroker{Name: "emb", Type: "embedded", Port: 1883, Enabled: true})
	assert.NoError(t, err)
	assert.Equal(t, 1, id)
	brokerRepo.AssertExpectations(t)
}

func TestMQTTService_AddBroker_DuplicateEmbedded(t *testing.T) {
	svc, brokerRepo, _ := setupMQTTService()

	existing := []gen.MQTTBroker{{Id: ptrInt(1), Name: "Embedded Broker", Type: "embedded"}}
	brokerRepo.On("GetAll", mock.Anything).Return(existing, nil)

	_, err := svc.AddBroker(context.Background(), gen.MQTTBroker{Name: "emb2", Type: "embedded", Port: 1883})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "an embedded broker already exists")
}

func TestMQTTService_GetAllBrokers(t *testing.T) {
	svc, brokerRepo, _ := setupMQTTService()

	expected := []gen.MQTTBroker{{Id: ptrInt(1), Name: "b1"}, {Id: ptrInt(2), Name: "b2"}}
	brokerRepo.On("GetAll", mock.Anything).Return(expected, nil)

	brokers, err := svc.GetAllBrokers(context.Background())
	assert.NoError(t, err)
	assert.Len(t, brokers, 2)
}

func TestMQTTService_UpdateBroker_Success(t *testing.T) {
	svc, brokerRepo, _ := setupMQTTService()

	broker := gen.MQTTBroker{Id: ptrInt(1), Name: "updated", Type: "external", Host: "h", Port: 1883}
	brokerRepo.On("GetByName", mock.Anything, "updated").Return(nil, nil)
	brokerRepo.On("GetAll", mock.Anything).Return([]gen.MQTTBroker{}, nil)
	brokerRepo.On("Update", mock.Anything, broker).Return(nil)

	err := svc.UpdateBroker(context.Background(), broker)
	assert.NoError(t, err)
}

func TestMQTTService_UpdateBroker_InvalidID(t *testing.T) {
	svc, _, _ := setupMQTTService()

	err := svc.UpdateBroker(context.Background(), gen.MQTTBroker{Name: "x", Type: "external", Host: "h", Port: 1883})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "broker id must be positive")
}

func TestMQTTService_DeleteBroker(t *testing.T) {
	svc, brokerRepo, _ := setupMQTTService()

	brokerRepo.On("Delete", mock.Anything, 1).Return(nil)

	err := svc.DeleteBroker(context.Background(), 1)
	assert.NoError(t, err)
}

// ============================================================================
// Subscription tests
// ============================================================================

func TestMQTTService_AddSubscription_Success(t *testing.T) {
	svc, brokerRepo, subRepo := setupMQTTService()

	sub := gen.MQTTSubscription{BrokerId: 1, TopicPattern: "zigbee2mqtt/+", DriverType: "mqtt-test-driver", Enabled: true}
	brokerRepo.On("GetByID", mock.Anything, 1).Return(&gen.MQTTBroker{Id: ptrInt(1)}, nil)
	subRepo.On("GetByBrokerID", mock.Anything, 1).Return([]gen.MQTTSubscription{}, nil)
	subRepo.On("Add", mock.Anything, sub).Return(1, nil)

	id, err := svc.AddSubscription(context.Background(), sub)
	assert.NoError(t, err)
	assert.Equal(t, 1, id)
	subRepo.AssertExpectations(t)
}

func TestMQTTService_AddSubscription_UnknownDriver(t *testing.T) {
	svc, _, _ := setupMQTTService()

	sub := gen.MQTTSubscription{BrokerId: 1, TopicPattern: "test/+", DriverType: "nonexistent", Enabled: true}

	_, err := svc.AddSubscription(context.Background(), sub)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown driver type")
}

func TestMQTTService_AddSubscription_NotPushDriver(t *testing.T) {
	svc, _, _ := setupMQTTService()

	sub := gen.MQTTSubscription{BrokerId: 1, TopicPattern: "test/+", DriverType: "non-push-driver", Enabled: true}

	_, err := svc.AddSubscription(context.Background(), sub)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not an MQTT push driver")
}

func TestMQTTService_AddSubscription_BrokerNotFound(t *testing.T) {
	svc, brokerRepo, _ := setupMQTTService()

	sub := gen.MQTTSubscription{BrokerId: 99, TopicPattern: "test/+", DriverType: "mqtt-test-driver", Enabled: true}
	brokerRepo.On("GetByID", mock.Anything, 99).Return(nil, nil)

	_, err := svc.AddSubscription(context.Background(), sub)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "broker not found")
}

func TestMQTTService_AddSubscription_InvalidTopic(t *testing.T) {
	svc, brokerRepo, _ := setupMQTTService()
	brokerRepo.On("GetByID", mock.Anything, 1).Return(&gen.MQTTBroker{Id: ptrInt(1)}, nil)

	tests := []struct {
		name    string
		pattern string
		errMsg  string
	}{
		{"spaces", "test topic/+", "must not contain spaces"},
		{"mid hash", "test/#/more", "multi-level wildcard (#) must be the last segment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := gen.MQTTSubscription{BrokerId: 1, TopicPattern: tt.pattern, DriverType: "mqtt-test-driver", Enabled: true}
			_, err := svc.AddSubscription(context.Background(), sub)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestMQTTService_GetAllSubscriptions(t *testing.T) {
	svc, _, subRepo := setupMQTTService()

	expected := []gen.MQTTSubscription{{Id: ptrInt(1)}, {Id: ptrInt(2)}}
	subRepo.On("GetAll", mock.Anything).Return(expected, nil)

	subs, err := svc.GetAllSubscriptions(context.Background())
	assert.NoError(t, err)
	assert.Len(t, subs, 2)
}

func TestMQTTService_DeleteSubscription(t *testing.T) {
	svc, _, subRepo := setupMQTTService()

	sub := &gen.MQTTSubscription{Id: ptrInt(1), BrokerId: 1, TopicPattern: "test/+"}
	subRepo.On("GetByID", mock.Anything, 1).Return(sub, nil)
	subRepo.On("Delete", mock.Anything, 1).Return(nil)

	err := svc.DeleteSubscription(context.Background(), 1)
	assert.NoError(t, err)
}

// ============================================================================
// Topic validation tests
// ============================================================================

func TestValidateTopicPattern_Valid(t *testing.T) {
	valid := []string{
		"zigbee2mqtt/+",
		"rtl_433/+/+",
		"tele/+/SENSOR",
		"home/#",
		"+/+/temperature",
	}
	for _, p := range valid {
		assert.NoError(t, validateTopicPattern(p), "expected valid: %s", p)
	}
}

func TestValidateTopicPattern_Invalid(t *testing.T) {
	assert.Error(t, validateTopicPattern("test topic"))
	assert.Error(t, validateTopicPattern("test/#/more"))
}

func TestValidateTopicPattern_WhitespaceOnly(t *testing.T) {
	svc, brokerRepo, _ := setupMQTTService()
	brokerRepo.On("GetByID", mock.Anything, 1).Return(&gen.MQTTBroker{Id: ptrInt(1)}, nil)

	sub := gen.MQTTSubscription{BrokerId: 1, TopicPattern: "   ", DriverType: "mqtt-test-driver", Enabled: true}
	_, err := svc.AddSubscription(context.Background(), sub)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "topic pattern cannot be empty")
}

func TestValidateTopicPattern_ExceedsMaxLength(t *testing.T) {
	longTopic := strings.Repeat("a/", 32768) + "b"
	err := validateTopicPattern(longTopic)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum length")
}

// ============================================================================
// Broker duplicate host:port tests
// ============================================================================

func TestMQTTService_AddBroker_DuplicateHostPort(t *testing.T) {
	svc, brokerRepo, _ := setupMQTTService()

	existing := []gen.MQTTBroker{{Id: ptrInt(1), Name: "Broker A", Type: "external", Host: "mqtt.local", Port: 1883}}
	brokerRepo.On("GetAll", mock.Anything).Return(existing, nil)
	brokerRepo.On("GetByName", mock.Anything, "Broker B").Return(nil, nil)

	_, err := svc.AddBroker(context.Background(), gen.MQTTBroker{
		Name: "Broker B", Type: "external", Host: "mqtt.local", Port: 1883, Enabled: true,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "broker host:port mqtt.local:1883 is already in use")
}

func TestMQTTService_AddBroker_DuplicateHostPort_CaseInsensitive(t *testing.T) {
	svc, brokerRepo, _ := setupMQTTService()

	existing := []gen.MQTTBroker{{Id: ptrInt(1), Name: "Broker A", Type: "external", Host: "MQTT.LOCAL", Port: 1883}}
	brokerRepo.On("GetAll", mock.Anything).Return(existing, nil)
	brokerRepo.On("GetByName", mock.Anything, "Broker B").Return(nil, nil)

	_, err := svc.AddBroker(context.Background(), gen.MQTTBroker{
		Name: "Broker B", Type: "external", Host: "mqtt.local", Port: 1883, Enabled: true,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already in use")
}

func TestMQTTService_AddBroker_DifferentPortOK(t *testing.T) {
	svc, brokerRepo, _ := setupMQTTService()

	existing := []gen.MQTTBroker{{Id: ptrInt(1), Name: "Broker A", Type: "external", Host: "mqtt.local", Port: 1883}}
	brokerRepo.On("GetAll", mock.Anything).Return(existing, nil)
	brokerRepo.On("GetByName", mock.Anything, "Broker B").Return(nil, nil)

	broker := gen.MQTTBroker{Name: "Broker B", Type: "external", Host: "mqtt.local", Port: 8883, Enabled: true}
	brokerRepo.On("Add", mock.Anything, broker).Return(2, nil)

	id, err := svc.AddBroker(context.Background(), broker)
	assert.NoError(t, err)
	assert.Equal(t, 2, id)
}

func TestMQTTService_UpdateBroker_DuplicateHostPort(t *testing.T) {
	svc, brokerRepo, _ := setupMQTTService()

	existing := []gen.MQTTBroker{
		{Id: ptrInt(1), Name: "Broker A", Type: "external", Host: "mqtt.local", Port: 1883},
		{Id: ptrInt(2), Name: "Broker B", Type: "external", Host: "other.local", Port: 1883},
	}
	brokerRepo.On("GetAll", mock.Anything).Return(existing, nil)
	brokerRepo.On("GetByName", mock.Anything, "Broker B").Return(&existing[1], nil)

	// Try to update broker 2 to use same host:port as broker 1
	err := svc.UpdateBroker(context.Background(), gen.MQTTBroker{
		Id: ptrInt(2), Name: "Broker B", Type: "external", Host: "mqtt.local", Port: 1883,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already in use")
}

func TestMQTTService_UpdateBroker_SameHostPortSelf(t *testing.T) {
	svc, brokerRepo, _ := setupMQTTService()

	existing := []gen.MQTTBroker{{Id: ptrInt(1), Name: "Broker A", Type: "external", Host: "mqtt.local", Port: 1883}}
	brokerRepo.On("GetAll", mock.Anything).Return(existing, nil)
	brokerRepo.On("GetByName", mock.Anything, "Broker A").Return(&existing[0], nil)
	brokerRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	// Updating self with same host:port is fine
	err := svc.UpdateBroker(context.Background(), gen.MQTTBroker{
		Id: ptrInt(1), Name: "Broker A", Type: "external", Host: "mqtt.local", Port: 1883,
	})
	assert.NoError(t, err)
}

// ============================================================================
// Broker name case-insensitive uniqueness tests
// ============================================================================

func TestMQTTService_AddBroker_DuplicateNameCaseInsensitive(t *testing.T) {
	svc, brokerRepo, _ := setupMQTTService()

	// GetByName uses LOWER, so it finds the existing broker
	brokerRepo.On("GetByName", mock.Anything, "mybroker").Return(&gen.MQTTBroker{Id: ptrInt(1), Name: "MyBroker"}, nil)

	_, err := svc.AddBroker(context.Background(), gen.MQTTBroker{
		Name: "mybroker", Type: "external", Host: "other.local", Port: 1883, Enabled: true,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "broker name")
	assert.Contains(t, err.Error(), "already in use")
}

// ============================================================================
// Topic overlap tests
// ============================================================================

func TestTopicsOverlap(t *testing.T) {
	tests := []struct {
		a, b    string
		overlap bool
	}{
		// Exact duplicate
		{"zigbee2mqtt/+", "zigbee2mqtt/+", true},
		// # subsumes everything beneath
		{"home/#", "home/kitchen/temperature", true},
		{"home/kitchen/temperature", "home/#", true},
		// + vs literal at same level
		{"home/+/temperature", "home/kitchen/temperature", true},
		// Disjoint topics
		{"home/kitchen/temperature", "office/lobby/humidity", false},
		// + vs + at same level
		{"home/+/temperature", "home/+/humidity", false},
		// # vs + deeper
		{"zigbee2mqtt/#", "zigbee2mqtt/+/+", true},
		// Different prefix, same suffix
		{"sensors/outdoor/temp", "sensors/indoor/temp", false},
		// Single segment
		{"test", "test", true},
		{"test", "other", false},
		// # alone matches everything
		{"#", "any/topic/here", true},
		// Different depths no wildcards
		{"a/b", "a/b/c", false},
		{"a/b/c", "a/b", false},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			assert.Equal(t, tt.overlap, topicsOverlap(tt.a, tt.b),
				"topicsOverlap(%q, %q)", tt.a, tt.b)
		})
	}
}

func TestMQTTService_AddSubscription_OverlappingTopic(t *testing.T) {
	svc, brokerRepo, subRepo := setupMQTTService()

	brokerRepo.On("GetByID", mock.Anything, 1).Return(&gen.MQTTBroker{Id: ptrInt(1)}, nil)
	subRepo.On("GetByBrokerID", mock.Anything, 1).Return([]gen.MQTTSubscription{
		{Id: ptrInt(10), BrokerId: 1, TopicPattern: "zigbee2mqtt/#", DriverType: "mqtt-test-driver", Enabled: true},
	}, nil)

	_, err := svc.AddSubscription(context.Background(), gen.MQTTSubscription{
		BrokerId: 1, TopicPattern: "zigbee2mqtt/+/+", DriverType: "mqtt-test-driver", Enabled: true,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "overlaps with existing subscription")
}

func TestMQTTService_AddSubscription_NonOverlappingTopicOK(t *testing.T) {
	svc, brokerRepo, subRepo := setupMQTTService()

	brokerRepo.On("GetByID", mock.Anything, 1).Return(&gen.MQTTBroker{Id: ptrInt(1)}, nil)
	subRepo.On("GetByBrokerID", mock.Anything, 1).Return([]gen.MQTTSubscription{
		{Id: ptrInt(10), BrokerId: 1, TopicPattern: "zigbee2mqtt/#", DriverType: "mqtt-test-driver", Enabled: true},
	}, nil)

	sub := gen.MQTTSubscription{BrokerId: 1, TopicPattern: "rtl_433/+", DriverType: "mqtt-test-driver", Enabled: true}
	subRepo.On("Add", mock.Anything, sub).Return(2, nil)

	id, err := svc.AddSubscription(context.Background(), sub)
	assert.NoError(t, err)
	assert.Equal(t, 2, id)
}

func TestMQTTService_AddSubscription_DifferentBrokerOK(t *testing.T) {
	svc, brokerRepo, subRepo := setupMQTTService()

	brokerRepo.On("GetByID", mock.Anything, 2).Return(&gen.MQTTBroker{Id: ptrInt(2)}, nil)
	// Broker 2 has no subscriptions
	subRepo.On("GetByBrokerID", mock.Anything, 2).Return([]gen.MQTTSubscription{}, nil)

	sub := gen.MQTTSubscription{BrokerId: 2, TopicPattern: "zigbee2mqtt/#", DriverType: "mqtt-test-driver", Enabled: true}
	subRepo.On("Add", mock.Anything, sub).Return(1, nil)

	id, err := svc.AddSubscription(context.Background(), sub)
	assert.NoError(t, err)
	assert.Equal(t, 1, id)
}

func TestMQTTService_UpdateSubscription_OverlapSkipsSelf(t *testing.T) {
	svc, brokerRepo, subRepo := setupMQTTService()

	brokerRepo.On("GetByID", mock.Anything, 1).Return(&gen.MQTTBroker{Id: ptrInt(1)}, nil)
	subRepo.On("GetByBrokerID", mock.Anything, 1).Return([]gen.MQTTSubscription{
		{Id: ptrInt(5), BrokerId: 1, TopicPattern: "zigbee2mqtt/#", DriverType: "mqtt-test-driver", Enabled: true},
	}, nil)
	subRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	// Updating the same subscription (id=5) with same topic is fine
	err := svc.UpdateSubscription(context.Background(), gen.MQTTSubscription{
		Id: ptrInt(5), BrokerId: 1, TopicPattern: "zigbee2mqtt/#", DriverType: "mqtt-test-driver", Enabled: true,
	})
	assert.NoError(t, err)
}

// ============================================================================
// Stub non-push driver (only implements SensorDriver, not PushDriver)
// ============================================================================

type stubNonPushDriver struct{}

func (d *stubNonPushDriver) Type() string                                         { return "non-push-driver" }
func (d *stubNonPushDriver) DisplayName() string                                  { return "Non Push" }
func (d *stubNonPushDriver) Description() string                                  { return "Not a push driver" }
func (d *stubNonPushDriver) ConfigFields() []drivers.ConfigFieldSpec              { return nil }
func (d *stubNonPushDriver) SupportedMeasurementTypes() []gen.MeasurementType     { return nil }
func (d *stubNonPushDriver) ValidateSensor(_ context.Context, _ gen.Sensor) error { return nil }

// ============================================================================
// Subscription notifier tests
// ============================================================================

func TestMQTTService_AddSubscription_NotifiesOnSuccess(t *testing.T) {
	svc, brokerRepo, subRepo := setupMQTTService()
	notifier := new(MockSubscriptionNotifier)
	svc.SetSubscriptionNotifier(notifier)

	sub := gen.MQTTSubscription{BrokerId: 1, TopicPattern: "zigbee2mqtt/+", DriverType: "mqtt-test-driver", Enabled: true}
	brokerRepo.On("GetByID", mock.Anything, 1).Return(&gen.MQTTBroker{Id: ptrInt(1)}, nil)
	subRepo.On("GetByBrokerID", mock.Anything, 1).Return([]gen.MQTTSubscription{}, nil)
	subRepo.On("Add", mock.Anything, sub).Return(42, nil)

	expected := sub
	expected.Id = ptrInt(42)
	notifier.On("OnSubscriptionAdded", expected).Return()

	id, err := svc.AddSubscription(context.Background(), sub)
	assert.NoError(t, err)
	assert.Equal(t, 42, id)
	notifier.AssertExpectations(t)
}

func TestMQTTService_DeleteSubscription_NotifiesOnSuccess(t *testing.T) {
	svc, _, subRepo := setupMQTTService()
	notifier := new(MockSubscriptionNotifier)
	svc.SetSubscriptionNotifier(notifier)

	sub := &gen.MQTTSubscription{Id: ptrInt(5), BrokerId: 1, TopicPattern: "test/#", DriverType: "mqtt-test-driver"}
	subRepo.On("GetByID", mock.Anything, 5).Return(sub, nil)
	subRepo.On("Delete", mock.Anything, 5).Return(nil)
	notifier.On("OnSubscriptionRemoved", *sub).Return()

	err := svc.DeleteSubscription(context.Background(), 5)
	assert.NoError(t, err)
	notifier.AssertExpectations(t)
}

func TestMQTTService_AddSubscription_NoNotifyWithoutNotifier(t *testing.T) {
	svc, brokerRepo, subRepo := setupMQTTService()
	// No notifier set — should not panic

	sub := gen.MQTTSubscription{BrokerId: 1, TopicPattern: "zigbee2mqtt/+", DriverType: "mqtt-test-driver", Enabled: true}
	brokerRepo.On("GetByID", mock.Anything, 1).Return(&gen.MQTTBroker{Id: ptrInt(1)}, nil)
	subRepo.On("GetByBrokerID", mock.Anything, 1).Return([]gen.MQTTSubscription{}, nil)
	subRepo.On("Add", mock.Anything, sub).Return(1, nil)

	id, err := svc.AddSubscription(context.Background(), sub)
	assert.NoError(t, err)
	assert.Equal(t, 1, id)
}

func ptrInt(i int) *int { return &i }
