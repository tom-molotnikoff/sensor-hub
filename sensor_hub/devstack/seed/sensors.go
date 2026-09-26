package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"example/sensorHub/drivers"
	gen "example/sensorHub/gen"
	"example/sensorHub/testharness/fixtures"
)

const (
	zigbeeTopic        = "zigbee2mqtt/#"
	sensorAnswerWait   = 20 * time.Second
	sensorAnswerPoll   = 250 * time.Millisecond
	embeddedBrokerType = "embedded"
)

var (
	zigbee2mqttDriver     = (&drivers.Zigbee2MQTTDriver{}).Type()
	httpTemperatureDriver = (&drivers.SensorHubHTTPTemperature{}).Type()
)

type device struct {
	fixtures.Sensor
	signals []signal
}

var mqttDevices = []device{
	climateDevice("living-room-sensor", 21.0, 45.0, 95),
	climateDevice("bedroom-sensor", 18.5, 52.0, 81),
	climateDevice("kitchen-sensor", 23.0, 58.0, 67),
	contactDevice("front-door", true, 88),
	contactDevice("back-door", true, 74),
	plugDevice("office-plug", 40.0, 120.0, 1.2),
	plugDevice("fridge-plug", 5.0, 150.0, 31.5),
	motionDevice("hallway-motion", false, 120, 92),
}

func mqttDevice(name string, signals ...signal) device {
	return device{
		Sensor:  fixtures.Sensor{Name: name, Driver: zigbee2mqttDriver, ExternalID: name, Config: map[string]string{}},
		signals: signals,
	}
}

func climateDevice(name string, temperature, humidity, battery float64) device {
	return mqttDevice(name,
		walk{measurement: "temperature", low: 16.0, high: 28.0, step: 0.2, decimals: 2, start: temperature},
		walk{measurement: "humidity", low: 30.0, high: 70.0, step: 0.5, decimals: 2, start: humidity},
		batteryLevel(battery),
		uniform{measurement: "link_quality", low: 40, high: 255},
	)
}

func contactDevice(name string, closed bool, battery float64) device {
	return mqttDevice(name,
		toggle{measurement: "contact", chance: 0.10, start: closed},
		batteryLevel(battery),
	)
}

func plugDevice(name string, minPower, maxPower, energy float64) device {
	return mqttDevice(name, plugDraw{minPower: minPower, maxPower: maxPower, startEnergy: energy})
}

func motionDevice(name string, occupied bool, illuminance, battery float64) device {
	return mqttDevice(name,
		toggle{measurement: "occupancy", chance: 0.15, start: occupied},
		walk{measurement: "illuminance", low: 0, high: 800, step: 25, start: illuminance},
		batteryLevel(battery),
	)
}

func batteryLevel(level float64) signal {
	return steady{measurement: "battery", value: value{number: level}}
}

type httpMock struct {
	name string
	url  string
}

var composeHTTPMocks = []httpMock{
	{name: "downstairs", url: "http://mock-http-downstairs:5000"},
	{name: "upstairs", url: "http://mock-http-upstairs:5000"},
}

func (m httpMock) device() device {
	return device{
		Sensor:  fixtures.Sensor{Name: m.name, Driver: httpTemperatureDriver, Config: map[string]string{"url": m.url}},
		signals: []signal{uniform{measurement: "temperature", low: 18.0, high: 22.0, decimals: 2}},
	}
}

func (s *seeder) devices() []device {
	devices := append([]device(nil), mqttDevices...)
	for _, mock := range s.httpMocks {
		devices = append(devices, mock.device())
	}
	return devices
}

func (s *seeder) createMQTTSubscription(ctx context.Context) error {
	brokerID, err := s.embeddedBrokerID(ctx)
	if err != nil {
		return err
	}
	subscriptions, err := s.mqtt.GetSubscriptionsByBrokerID(ctx, brokerID)
	if err != nil {
		return err
	}
	for _, subscription := range subscriptions {
		if subscription.TopicPattern == zigbeeTopic {
			s.logger.Info("MQTT subscription exists", "topic", zigbeeTopic)
			return nil
		}
	}
	if _, err := s.mqtt.AddSubscription(ctx, gen.MQTTSubscription{
		BrokerId:     brokerID,
		TopicPattern: zigbeeTopic,
		DriverType:   zigbee2mqttDriver,
		Enabled:      true,
	}); err != nil {
		return err
	}
	s.logger.Info("created the MQTT subscription", "topic", zigbeeTopic, "driver", zigbee2mqttDriver)
	return nil
}

func (s *seeder) embeddedBrokerID(ctx context.Context) (int, error) {
	brokers, err := s.mqtt.GetAllBrokers(ctx)
	if err != nil {
		return 0, err
	}
	for _, broker := range brokers {
		if broker.Type == embeddedBrokerType && broker.Id != nil {
			return *broker.Id, nil
		}
	}
	return 0, errors.New("no embedded MQTT broker to subscribe on")
}

func (s *seeder) createSensors(ctx context.Context) (map[string]int, error) {
	answerBy := time.Now().Add(sensorAnswerWait)
	ids := make(map[string]int)
	for _, device := range s.devices() {
		id, err := s.createSensor(ctx, device.Sensor, answerBy)
		if err != nil {
			return nil, err
		}
		if err := recordSeededSensor(ctx, s.db.Writer, device.Name, id); err != nil {
			return nil, err
		}
		ids[device.Name] = id
	}
	return ids, nil
}

func (s *seeder) createSensor(ctx context.Context, sensor fixtures.Sensor, answerBy time.Time) (int, error) {
	existing, err := s.sensors.ServiceGetSensorByName(ctx, sensor.Name)
	if err != nil {
		return 0, err
	}
	if existing != nil {
		return existing.Id, s.finishSensor(ctx, *existing)
	}
	if err := s.waitUntilAnswering(ctx, sensor, answerBy); err != nil {
		return 0, fmt.Errorf("sensor %s did not answer: %w", sensor.Name, err)
	}
	id, err := fixtures.CreateApprovedSensor(ctx, s.sensors, sensor)
	if err != nil {
		return 0, err
	}
	s.logger.Info("created sensor", "name", sensor.Name, "driver", sensor.Driver)
	return id, nil
}

func (s *seeder) finishSensor(ctx context.Context, existing gen.Sensor) error {
	if existing.Status != gen.SensorStatusActive {
		if err := s.sensors.ServiceApproveSensor(ctx, existing.Id); err != nil {
			return fmt.Errorf("failed to approve sensor %s: %w", existing.Name, err)
		}
	}
	s.logger.Info("sensor exists, made sure it is approved", "name", existing.Name)
	return nil
}

func (s *seeder) waitUntilAnswering(ctx context.Context, sensor fixtures.Sensor, answerBy time.Time) error {
	candidate := gen.Sensor{Name: sensor.Name, SensorDriver: sensor.Driver, Config: sensor.Config}
	err := s.sensors.ServiceValidateSensorConfig(ctx, candidate)
	if err == nil {
		return nil
	}
	s.logger.Info("waiting for sensor to answer", "name", sensor.Name, "cause", err)
	for time.Now().Before(answerBy) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sensorAnswerPoll):
		}
		if err = s.sensors.ServiceValidateSensorConfig(ctx, candidate); err == nil {
			return nil
		}
	}
	return err
}
