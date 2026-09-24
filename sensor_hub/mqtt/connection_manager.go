// Package mqtt provides the MQTT connection manager that maintains per-broker
// Paho MQTT client connections, manages subscriptions, and routes incoming
// messages to the appropriate PushDriver for parsing.
//
// The ConnectionManager is the bridge between external MQTT brokers and the
// Sensor Hub's driver/service layer. It handles:
//   - Per-broker client lifecycle (connect, reconnect, disconnect)
//   - Subscription management (subscribe/unsubscribe based on DB config)
//   - Message routing: topic → subscription → driver → readings
//   - Auto-discovery: unknown devices become pending sensors

package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	database "example/sensorHub/db"
	"example/sensorHub/drivers"
	gen "example/sensorHub/gen"
	"example/sensorHub/service"
	"example/sensorHub/telemetry"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// MessageHandler is called when an MQTT message is received. It is responsible
// for routing the message through the correct driver and storing results.
type MessageHandler func(ctx context.Context, brokerID int, topic string, payload []byte)

// BrokerConnection holds the Paho client and metadata for a single broker.
type BrokerConnection struct {
	Broker gen.MQTTBroker
	Client pahomqtt.Client
}

type bridgeCacheKey struct {
	brokerID int
	key      string
}

type bridgeDevicesCache struct {
	mu             sync.RWMutex
	ieeeToFriendly map[bridgeCacheKey]string
	deviceMetadata map[bridgeCacheKey]drivers.DeviceMetadata
}

// ConnectionManager manages MQTT client connections for all configured brokers.
type ConnectionManager struct {
	sensorService service.SensorServiceInterface
	subRepo       database.MQTTSubscriptionRepositoryInterface
	brokerRepo    database.MQTTBrokerRepositoryInterface
	logger        *slog.Logger

	connections map[int]*BrokerConnection // keyed by broker ID
	mu          sync.RWMutex

	instruments *mqttInstruments
	tracer      trace.Tracer
	stats       *StatsTracker

	bridgeDevices *bridgeDevicesCache
}

// NewConnectionManager creates a new connection manager.
func NewConnectionManager(
	sensorService service.SensorServiceInterface,
	subRepo database.MQTTSubscriptionRepositoryInterface,
	brokerRepo database.MQTTBrokerRepositoryInterface,
	logger *slog.Logger,
) *ConnectionManager {
	return &ConnectionManager{
		sensorService: sensorService,
		subRepo:       subRepo,
		brokerRepo:    brokerRepo,
		logger:        logger.With("component", "mqtt_connection_manager"),
		connections:   make(map[int]*BrokerConnection),
		instruments:   newMQTTInstruments(),
		tracer:        telemetry.Tracer("mqtt"),
		stats:         NewStatsTracker(),
		bridgeDevices: &bridgeDevicesCache{
			ieeeToFriendly: make(map[bridgeCacheKey]string),
			deviceMetadata: make(map[bridgeCacheKey]drivers.DeviceMetadata),
		},
	}
}

// Start loads all enabled brokers and subscriptions from the database,
// connects to each broker, and subscribes to the configured topics.
func (cm *ConnectionManager) Start(ctx context.Context) error {
	brokers, err := cm.brokerRepo.GetEnabled(ctx)
	if err != nil {
		return fmt.Errorf("failed to load MQTT brokers: %w", err)
	}

	for _, broker := range brokers {
		if !broker.Enabled {
			cm.logger.Debug("skipping disabled broker", "broker", broker.Name)
			continue
		}
		if broker.Type == "embedded" {
			// Embedded broker connections use localhost
			broker.Host = "localhost"
		}
		if err := cm.ConnectBroker(ctx, broker); err != nil {
			cm.logger.Error("failed to connect to broker", "broker", broker.Name, "error", err)
			continue
		}
	}

	return nil
}

// Stop disconnects all broker clients gracefully.
func (cm *ConnectionManager) Stop() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for id, conn := range cm.connections {
		cm.logger.Info("disconnecting from broker", "broker", conn.Broker.Name)
		conn.Client.Disconnect(250)
		delete(cm.connections, id)
	}
}

// ConnectBroker establishes a connection to the given broker and subscribes
// to all enabled subscriptions for that broker.
func (cm *ConnectionManager) ConnectBroker(ctx context.Context, broker gen.MQTTBroker) error {
	brokerID := 0
	if broker.Id != nil {
		brokerID = *broker.Id
	}
	ctx, span := cm.tracer.Start(ctx, "mqtt.connect_broker",
		trace.WithAttributes(
			attribute.String("broker.name", broker.Name),
			attribute.Int("broker.id", brokerID),
			attribute.String("broker.host", broker.Host),
			attribute.Int("broker.port", broker.Port),
		))
	defer span.End()

	brokerURL := fmt.Sprintf("tcp://%s:%d", broker.Host, broker.Port)

	clientID := fmt.Sprintf("sensor-hub-%d", brokerID)
	if broker.ClientId != nil && *broker.ClientId != "" {
		clientID = *broker.ClientId
	}

	cm.stats.SetBrokerName(brokerID, broker.Name)

	opts := pahomqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(clientID).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetMaxReconnectInterval(2 * time.Minute).
		SetConnectionLostHandler(func(client pahomqtt.Client, err error) {
			cm.logger.Warn("MQTT connection lost", "broker", broker.Name, "error", err)
			cm.instruments.connectionsActive.Add(context.Background(), -1)
			cm.stats.RecordDisconnected(brokerID)
		}).
		SetOnConnectHandler(func(client pahomqtt.Client) {
			cm.logger.Info("MQTT connected", "broker", broker.Name)
			cm.instruments.connectionsActive.Add(context.Background(), 1)
			cm.stats.RecordConnected(brokerID)
			// Re-subscribe on reconnect
			go func() {
				if err := cm.subscribeAll(context.Background(), brokerID, client); err != nil {
					cm.logger.Error("failed to re-subscribe after reconnect", "broker", broker.Name, "error", err)
				}
			}()
		})

	if broker.Username != nil && *broker.Username != "" {
		opts.SetUsername(*broker.Username)
	}
	if broker.Password != nil && *broker.Password != "" {
		opts.SetPassword(*broker.Password)
	}

	client := pahomqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(10 * time.Second) {
		span.RecordError(fmt.Errorf("connection timed out"))
		return fmt.Errorf("connection to broker %s timed out", broker.Name)
	}
	if token.Error() != nil {
		span.RecordError(token.Error())
		return fmt.Errorf("failed to connect to broker %s: %w", broker.Name, token.Error())
	}

	cm.mu.Lock()
	cm.connections[brokerID] = &BrokerConnection{
		Broker: broker,
		Client: client,
	}
	cm.mu.Unlock()

	if err := cm.subscribeAll(ctx, brokerID, client); err != nil {
		cm.logger.Error("failed to subscribe to topics", "broker", broker.Name, "error", err)
	}

	cm.logger.Info("connected to MQTT broker", "broker", broker.Name, "url", brokerURL)
	return nil
}

// DisconnectBroker disconnects from a specific broker by ID.
func (cm *ConnectionManager) DisconnectBroker(brokerID int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	conn, ok := cm.connections[brokerID]
	if !ok {
		return
	}

	conn.Client.Disconnect(250)
	delete(cm.connections, brokerID)
	cm.instruments.connectionsActive.Add(context.Background(), -1)
	cm.stats.RecordDisconnected(brokerID)
	cm.logger.Info("disconnected from broker", "broker_id", brokerID)
}

// subscribeAll loads enabled subscriptions for a broker and subscribes to each topic.
func (cm *ConnectionManager) subscribeAll(ctx context.Context, brokerID int, client pahomqtt.Client) error {
	subs, err := cm.subRepo.GetEnabledByBrokerID(ctx, brokerID)
	if err != nil {
		return fmt.Errorf("failed to load subscriptions for broker %d: %w", brokerID, err)
	}

	for _, sub := range subs {
		if !sub.Enabled {
			continue
		}
		cm.subscribeTopic(client, brokerID, sub)
	}

	return nil
}

// subscribeTopic subscribes to a single MQTT topic and routes messages.
func (cm *ConnectionManager) subscribeTopic(client pahomqtt.Client, brokerID int, sub gen.MQTTSubscription) {
	handler := func(client pahomqtt.Client, msg pahomqtt.Message) {
		cm.handleMessage(context.Background(), brokerID, sub.DriverType, msg.Topic(), msg.Payload())
	}

	token := client.Subscribe(sub.TopicPattern, 0, handler)
	if token.WaitTimeout(5*time.Second) && token.Error() != nil {
		cm.logger.Error("failed to subscribe", "topic", sub.TopicPattern, "error", token.Error())
		return
	}

	cm.logger.Info("subscribed to MQTT topic", "topic", sub.TopicPattern, "driver", sub.DriverType)
}

// handleMessage processes an incoming MQTT message by routing it through the
// appropriate PushDriver. It handles both known sensors and auto-discovery.
func (cm *ConnectionManager) handleMessage(ctx context.Context, brokerID int, driverType string, topic string, payload []byte) {
	start := time.Now()

	ctx, span := cm.tracer.Start(ctx, "mqtt.handle_message",
		trace.WithAttributes(
			attribute.Int("broker.id", brokerID),
			attribute.String("driver", driverType),
			attribute.String("topic", topic),
		))
	defer span.End()

	attrs := attribute.NewSet(
		attribute.Int("broker_id", brokerID),
		attribute.String("driver", driverType),
	)

	cm.stats.RecordMessageReceived(brokerID)
	cm.instruments.messagesReceived.Add(ctx, 1, metric.WithAttributeSet(attrs))

	drv, ok := drivers.Get(driverType)
	if !ok {
		cm.logger.Warn("no driver registered for type", "driver", driverType, "topic", topic)
		cm.instruments.messageErrors.Add(ctx, 1, metric.WithAttributeSet(attrs))
		cm.stats.RecordParseError(brokerID)
		return
	}

	pushDriver, ok := drv.(drivers.PushDriver)
	if !ok {
		cm.logger.Warn("driver is not a PushDriver", "driver", driverType)
		cm.instruments.messageErrors.Add(ctx, 1, metric.WithAttributeSet(attrs))
		cm.stats.RecordParseError(brokerID)
		return
	}

	if systemHandler, ok := pushDriver.(drivers.SystemMessageHandler); ok {
		if metadata := systemHandler.ParseSystemMessage(topic, payload); metadata != nil {
			cm.updateBridgeDevicesCache(brokerID, metadata)
			cm.backfillSensorMetadata(ctx, brokerID, driverType, metadata)
			return
		}
	}

	// Identify the device from the message
	deviceName, err := pushDriver.IdentifyDevice(topic, payload)
	if err != nil {
		cm.logger.Debug("could not identify device", "topic", topic, "error", err)
		return
	}

	originalDeviceName := deviceName
	deviceName = cm.resolveDeviceName(brokerID, deviceName)

	span.SetAttributes(attribute.String("sensor", deviceName))

	sensor, err := cm.lookupSensorByIdentity(ctx, deviceName)
	if deviceName != originalDeviceName {
		if err == nil {
			originalSensor, originalErr := cm.lookupSensorByIdentity(ctx, originalDeviceName)
			if originalErr == nil {
				cm.logger.Warn("IEEE resolution collision, keeping original sensor",
					"broker_id", brokerID,
					"ieee_name", originalDeviceName,
					"friendly_name", deviceName)
				sensor = originalSensor
				deviceName = originalDeviceName
			}
		} else {
			sensor, err = cm.renameResolvedIEEESensor(ctx, brokerID, deviceName, originalDeviceName)
		}
	}
	if err != nil {
		// Sensor not found → auto-discovery: create as pending
		cm.autoDiscoverSensor(ctx, brokerID, deviceName, driverType, pushDriver)
		return
	}

	// Only process readings for active, enabled sensors
	if sensor.Status != gen.SensorStatusActive || !sensor.Enabled {
		return
	}

	readings, err := pushDriver.ParseMessage(topic, payload)
	if err != nil {
		cm.logger.Error("failed to parse MQTT message", "sensor", deviceName, "topic", topic, "error", err)
		cm.instruments.messageErrors.Add(ctx, 1, metric.WithAttributeSet(attrs))
		cm.stats.RecordParseError(brokerID)
		span.RecordError(err)
		cm.sensorService.ServiceUpdateSensorHealthById(ctx, sensor.Id, gen.Bad,
			fmt.Sprintf("parse error: %v", err))
		return
	}

	if len(readings) == 0 {
		return
	}

	if err := cm.sensorService.ServiceProcessPushReadings(ctx, *sensor, readings); err != nil {
		cm.logger.Error("failed to process MQTT readings", "sensor", deviceName, "error", err)
		cm.instruments.messageErrors.Add(ctx, 1, metric.WithAttributeSet(attrs))
		cm.stats.RecordProcessingError(brokerID)
		span.RecordError(err)
		return
	}

	elapsed := float64(time.Since(start).Microseconds()) / 1000.0
	cm.instruments.processingTime.Record(ctx, elapsed, metric.WithAttributeSet(attrs))

	cm.logger.Debug("processed MQTT message", "sensor", deviceName, "readings", len(readings))
}

func (cm *ConnectionManager) resolveDeviceName(brokerID int, deviceName string) string {
	if !isIEEEAddress(deviceName) {
		return deviceName
	}

	cm.bridgeDevices.mu.RLock()
	defer cm.bridgeDevices.mu.RUnlock()

	friendlyName, ok := cm.bridgeDevices.ieeeToFriendly[bridgeCacheKey{brokerID: brokerID, key: deviceName}]
	if !ok || friendlyName == "" {
		return deviceName
	}

	return friendlyName
}

func (cm *ConnectionManager) lookupSensorByIdentity(ctx context.Context, deviceName string) (*gen.Sensor, error) {
	sensor, err := cm.sensorService.ServiceGetSensorByExternalId(ctx, deviceName)
	if err == nil {
		return sensor, nil
	}
	sensor, err = cm.sensorService.ServiceGetSensorByName(ctx, deviceName)
	if err != nil {
		return nil, err
	}
	if sensor == nil {
		return nil, fmt.Errorf("no sensor found with external id or name %s", deviceName)
	}
	return sensor, nil
}

func (cm *ConnectionManager) renameResolvedIEEESensor(ctx context.Context, brokerID int, resolvedName string, ieeeName string) (*gen.Sensor, error) {
	sensor, err := cm.lookupSensorByIdentity(ctx, ieeeName)
	if err != nil {
		return nil, err
	}

	renamed := *sensor
	renamed.Name = resolvedName

	if device, ok := cm.cachedDeviceMetadata(brokerID, resolvedName); ok {
		renamed.Metadata = mergedSensorMetadata(renamed.Metadata, device)
	}

	if err := cm.sensorService.ServiceUpdateSensorById(ctx, renamed, false); err != nil {
		return nil, err
	}

	return &renamed, nil
}

func (cm *ConnectionManager) cachedDeviceMetadata(brokerID int, deviceName string) (drivers.DeviceMetadata, bool) {
	cm.bridgeDevices.mu.RLock()
	defer cm.bridgeDevices.mu.RUnlock()

	device, ok := cm.bridgeDevices.deviceMetadata[bridgeCacheKey{brokerID: brokerID, key: deviceName}]
	return device, ok
}

func isIEEEAddress(name string) bool {
	if len(name) != 18 || !strings.HasPrefix(name, "0x") {
		return false
	}

	for _, char := range name[2:] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') && (char < 'A' || char > 'F') {
			return false
		}
	}

	return true
}

func (cm *ConnectionManager) updateBridgeDevicesCache(brokerID int, devices []drivers.DeviceMetadata) {
	cm.bridgeDevices.mu.Lock()
	defer cm.bridgeDevices.mu.Unlock()

	for _, device := range devices {
		if device.IEEEAddress != "" {
			cm.bridgeDevices.ieeeToFriendly[bridgeCacheKey{brokerID: brokerID, key: device.IEEEAddress}] = device.FriendlyName
		}
		cm.bridgeDevices.deviceMetadata[bridgeCacheKey{brokerID: brokerID, key: device.FriendlyName}] = device
	}
}

func (cm *ConnectionManager) backfillSensorMetadata(ctx context.Context, brokerID int, driverType string, devices []drivers.DeviceMetadata) {
	sensors, err := cm.sensorService.ServiceGetSensorsByDriver(ctx, driverType)
	if err != nil {
		cm.logger.Error("failed to load sensors for metadata backfill", "driver", driverType, "broker_id", brokerID, "error", err)
		return
	}

	devicesByName := make(map[string]drivers.DeviceMetadata, len(devices))
	for _, device := range devices {
		devicesByName[device.FriendlyName] = device
	}

	for _, sensor := range sensors {
		device, ok := devicesByName[sensor.Name]
		if !ok && sensor.ExternalId != nil {
			device, ok = devicesByName[*sensor.ExternalId]
		}
		if !ok {
			continue
		}

		sensor.Metadata = mergedSensorMetadata(sensor.Metadata, device)
		if err := cm.sensorService.ServiceUpdateSensorById(ctx, sensor, false); err != nil {
			cm.logger.Error("failed to backfill sensor metadata", "sensor_id", sensor.Id, "sensor", sensor.Name, "broker_id", brokerID, "error", err)
		}
	}
}

func mergedSensorMetadata(existing *map[string]interface{}, device drivers.DeviceMetadata) *map[string]interface{} {
	metadata := make(map[string]interface{})
	if existing != nil {
		for key, value := range *existing {
			metadata[key] = value
		}
	}
	for key, value := range device.Metadata {
		metadata[key] = value
	}
	if device.IEEEAddress != "" {
		metadata["ieee_address"] = device.IEEEAddress
	}
	if len(device.Exposes) > 0 {
		metadata["exposes"] = json.RawMessage(device.Exposes)
	}
	return &metadata
}

// autoDiscoverSensor creates a new sensor in pending status for user approval.
func (cm *ConnectionManager) autoDiscoverSensor(ctx context.Context, brokerID int, deviceName, driverType string, pushDriver drivers.PushDriver) {
	// Check if already exists by external_id (race condition guard)
	exists, _ := cm.sensorService.ServiceSensorExistsByExternalId(ctx, deviceName)
	if exists {
		return
	}
	// Also check by name for backward compat
	exists, _ = cm.sensorService.ServiceSensorExists(ctx, deviceName)
	if exists {
		return
	}

	sensor := gen.Sensor{
		Name:         deviceName,
		ExternalId:   &deviceName,
		SensorDriver: driverType,
		Config:       map[string]string{},
		Enabled:      false,
		Status:       gen.SensorStatusPending,
	}
	if device, ok := cm.cachedDeviceMetadata(brokerID, deviceName); ok {
		sensor.Metadata = mergedSensorMetadata(sensor.Metadata, device)
	}

	if err := cm.sensorService.ServiceAddSensor(ctx, sensor); err != nil {
		cm.logger.Error("failed to auto-discover sensor", "name", deviceName, "error", err)
		return
	}

	cm.instruments.devicesDiscovered.Add(ctx, 1,
		metric.WithAttributes(attribute.Int("broker_id", brokerID), attribute.String("driver", driverType)))
	cm.stats.RecordDeviceDiscovered(brokerID)

	cm.logger.Info("auto-discovered MQTT sensor", "name", deviceName, "driver", driverType)
}

// OnSubscriptionAdded subscribes to a new topic on the live broker client.
// If the broker is not connected, the subscription will activate on next connect.
func (cm *ConnectionManager) OnSubscriptionAdded(sub gen.MQTTSubscription) {
	cm.mu.RLock()
	conn, ok := cm.connections[sub.BrokerId]
	cm.mu.RUnlock()
	if !ok || !conn.Client.IsConnected() {
		cm.logger.Warn("broker not connected, subscription will activate on next connect",
			"broker_id", sub.BrokerId, "topic", sub.TopicPattern)
		return
	}
	cm.subscribeTopic(conn.Client, sub.BrokerId, sub)
}

// OnSubscriptionRemoved unsubscribes from a topic on the live broker client.
func (cm *ConnectionManager) OnSubscriptionRemoved(sub gen.MQTTSubscription) {
	cm.mu.RLock()
	conn, ok := cm.connections[sub.BrokerId]
	cm.mu.RUnlock()
	if !ok || !conn.Client.IsConnected() {
		return
	}
	token := conn.Client.Unsubscribe(sub.TopicPattern)
	if token.WaitTimeout(5*time.Second) && token.Error() != nil {
		cm.logger.Error("failed to unsubscribe", "topic", sub.TopicPattern, "error", token.Error())
		return
	}
	cm.logger.Info("unsubscribed from MQTT topic", "topic", sub.TopicPattern)
}

func (cm *ConnectionManager) Publish(brokerID int, topic string, payload []byte, qos byte) error {
	cm.mu.RLock()
	conn, ok := cm.connections[brokerID]
	cm.mu.RUnlock()
	if !ok || !conn.Client.IsConnected() {
		return fmt.Errorf("broker %d is not connected", brokerID)
	}

	token := conn.Client.Publish(topic, qos, false, payload)
	if token.WaitTimeout(5*time.Second) && token.Error() != nil {
		return fmt.Errorf("publish to broker %d failed: %w", brokerID, token.Error())
	}

	return nil
}

// IsConnected returns whether a broker connection is currently active.
func (cm *ConnectionManager) IsConnected(brokerID int) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	conn, ok := cm.connections[brokerID]
	if !ok {
		return false
	}
	return conn.Client.IsConnected()
}

// ConnectedBrokerIDs returns the IDs of all currently connected brokers.
func (cm *ConnectionManager) ConnectedBrokerIDs() []int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	ids := make([]int, 0, len(cm.connections))
	for id := range cm.connections {
		ids = append(ids, id)
	}
	return ids
}

// Stats returns a snapshot of per-broker runtime statistics, enriched with
// live connection status from the Paho clients.
func (cm *ConnectionManager) Stats() map[int]BrokerStats {
	snapshot := cm.stats.Snapshot()

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for id, conn := range cm.connections {
		bs, ok := snapshot[id]
		if !ok {
			bs = BrokerStats{BrokerID: id, BrokerName: conn.Broker.Name}
		}
		bs.Connected = conn.Client.IsConnected()
		snapshot[id] = bs
	}
	return snapshot
}
