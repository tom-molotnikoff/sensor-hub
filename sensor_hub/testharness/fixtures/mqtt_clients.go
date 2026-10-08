package fixtures

import (
	"context"
	"errors"
	"fmt"

	database "example/sensorHub/db"
	"example/sensorHub/service"
)

// MQTTClient is a device's credential for the embedded broker with a password
// known in advance, so a test or the devstack can hand it to a device.
type MQTTClient struct {
	Name        string
	TopicPrefix string
	Password    string
}

// EnsureMQTTClient stores the client unless one with its name exists.
func EnsureMQTTClient(ctx context.Context, clients service.MQTTClientRepository, client MQTTClient) error {
	_, err := clients.GetByName(ctx, client.Name)
	if err == nil {
		return nil
	}
	if !errors.Is(err, database.ErrMQTTClientNotFound) {
		return fmt.Errorf("failed to look up MQTT client %s: %w", client.Name, err)
	}
	if _, err := clients.Add(ctx, client.Name, client.TopicPrefix, service.HashMQTTClientPassword(client.Password), true); err != nil {
		return fmt.Errorf("failed to create MQTT client %s: %w", client.Name, err)
	}
	return nil
}
