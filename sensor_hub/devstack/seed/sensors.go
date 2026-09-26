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

var mqttDevices = []fixtures.Sensor{
	mqttDevice("living-room-sensor"),
	mqttDevice("bedroom-sensor"),
	mqttDevice("kitchen-sensor"),
	mqttDevice("front-door"),
	mqttDevice("back-door"),
	mqttDevice("office-plug"),
	mqttDevice("fridge-plug"),
	mqttDevice("hallway-motion"),
}

func mqttDevice(name string) fixtures.Sensor {
	return fixtures.Sensor{Name: name, Driver: zigbee2mqttDriver, ExternalID: name, Config: map[string]string{}}
}

type httpMock struct {
	name string
	url  string
}

var composeHTTPMocks = []httpMock{
	{name: "downstairs", url: "http://mock-http-downstairs:5000"},
	{name: "upstairs", url: "http://mock-http-upstairs:5000"},
}

func (m httpMock) sensor() fixtures.Sensor {
	return fixtures.Sensor{Name: m.name, Driver: httpTemperatureDriver, Config: map[string]string{"url": m.url}}
}

func (s *seeder) devSensors() []fixtures.Sensor {
	sensors := append([]fixtures.Sensor(nil), mqttDevices...)
	for _, mock := range s.httpMocks {
		sensors = append(sensors, mock.sensor())
	}
	return sensors
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

func (s *seeder) createSensors(ctx context.Context) error {
	answerBy := time.Now().Add(sensorAnswerWait)
	for _, sensor := range s.devSensors() {
		if err := s.createSensor(ctx, sensor, answerBy); err != nil {
			return err
		}
	}
	return nil
}

func (s *seeder) createSensor(ctx context.Context, sensor fixtures.Sensor, answerBy time.Time) error {
	existing, err := s.sensors.ServiceGetSensorByName(ctx, sensor.Name)
	if err != nil {
		return err
	}
	if existing != nil {
		return s.finishSensor(ctx, *existing)
	}
	if err := s.waitUntilAnswering(ctx, sensor, answerBy); err != nil {
		return fmt.Errorf("sensor %s did not answer: %w", sensor.Name, err)
	}
	if _, err := fixtures.CreateApprovedSensor(ctx, s.sensors, sensor); err != nil {
		return err
	}
	s.logger.Info("created sensor", "name", sensor.Name, "driver", sensor.Driver)
	return nil
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
