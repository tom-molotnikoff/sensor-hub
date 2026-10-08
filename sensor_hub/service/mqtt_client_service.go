package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
)

// ErrInvalidMQTTClient wraps every validation failure for an MQTT client.
var ErrInvalidMQTTClient = errors.New("invalid MQTT client")

// mqttClientPasswordBytes is the entropy of a generated password, shown to the
// admin as twice as many hex characters.
const mqttClientPasswordBytes = 32

// MQTTConnectRefusal says why the embedded broker refused a CONNECT. It is
// empty when the CONNECT was accepted.
type MQTTConnectRefusal string

const (
	MQTTConnectRefusedAuth     MQTTConnectRefusal = "auth"
	MQTTConnectRefusedDisabled MQTTConnectRefusal = "disabled"
)

type MQTTClientRepository interface {
	Add(ctx context.Context, name, topicPrefix, passwordHash string, enabled bool) (int, error)
	GetByID(ctx context.Context, id int) (database.MQTTClient, error)
	GetByName(ctx context.Context, name string) (database.MQTTClient, error)
	GetAll(ctx context.Context) ([]database.MQTTClient, error)
	Update(ctx context.Context, id int, name, topicPrefix string, enabled bool) error
	SetPasswordHash(ctx context.Context, id int, passwordHash string) error
	RecordConnected(ctx context.Context, id int) error
	Delete(ctx context.Context, id int) error
}

// MQTTClientSessions is the embedded broker's live client list.
type MQTTClientSessions interface {
	ConnectedUsernames() map[string]bool
	// Disconnect closes every live connection made with the username.
	Disconnect(username string)
}

type MQTTClientServiceInterface interface {
	List(ctx context.Context) ([]gen.MQTTClient, error)
	Get(ctx context.Context, id int) (gen.MQTTClient, error)
	Create(ctx context.Context, input gen.MQTTClientInput) (gen.MQTTClientCreated, error)
	Update(ctx context.Context, id int, input gen.MQTTClientInput) (gen.MQTTClient, error)
	RotatePassword(ctx context.Context, id int) (gen.MQTTClientCreated, error)
	Delete(ctx context.Context, id int) error
}

// MQTTClientService owns the credentials devices use to dial in to the
// embedded broker: it generates passwords, keeps only their SHA-256 hashes,
// checks CONNECTs against them, and disconnects a client whose access changes.
type MQTTClientService struct {
	repo     MQTTClientRepository
	sessions MQTTClientSessions
	logger   *slog.Logger
}

func NewMQTTClientService(repo MQTTClientRepository, logger *slog.Logger) *MQTTClientService {
	return &MQTTClientService{repo: repo, logger: logger.With("component", "mqtt_client_service")}
}

// SetSessions connects the service to the embedded broker it authenticates
// for. Set after construction because the broker is built with this service.
func (s *MQTTClientService) SetSessions(sessions MQTTClientSessions) {
	s.sessions = sessions
}

func (s *MQTTClientService) List(ctx context.Context) ([]gen.MQTTClient, error) {
	clients, err := s.repo.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	connected := s.connectedUsernames()
	result := make([]gen.MQTTClient, len(clients))
	for i, client := range clients {
		result[i] = toGenMQTTClient(client, connected[client.Name])
	}
	return result, nil
}

func (s *MQTTClientService) Get(ctx context.Context, id int) (gen.MQTTClient, error) {
	client, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return gen.MQTTClient{}, err
	}
	return toGenMQTTClient(client, s.connectedUsernames()[client.Name]), nil
}

func (s *MQTTClientService) Create(ctx context.Context, input gen.MQTTClientInput) (gen.MQTTClientCreated, error) {
	name, err := validateMQTTClientInput(input)
	if err != nil {
		return gen.MQTTClientCreated{}, err
	}
	password, err := generateMQTTClientPassword()
	if err != nil {
		return gen.MQTTClientCreated{}, err
	}
	enabled := input.Enabled == nil || *input.Enabled
	id, err := s.repo.Add(ctx, name, input.TopicPrefix, HashMQTTClientPassword(password), enabled)
	if err != nil {
		return gen.MQTTClientCreated{}, err
	}
	s.logger.Info("created MQTT client", "client_id", id, "name", name, "topic_prefix", input.TopicPrefix)
	return s.withPassword(ctx, id, password)
}

func (s *MQTTClientService) Update(ctx context.Context, id int, input gen.MQTTClientInput) (gen.MQTTClient, error) {
	name, err := validateMQTTClientInput(input)
	if err != nil {
		return gen.MQTTClient{}, err
	}
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return gen.MQTTClient{}, err
	}
	enabled := current.Enabled
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	if err := s.repo.Update(ctx, id, name, input.TopicPrefix, enabled); err != nil {
		return gen.MQTTClient{}, err
	}
	// A live connection keeps the name and prefix it connected with, so any
	// change ends it and the next CONNECT is checked against the new settings.
	if name != current.Name || input.TopicPrefix != current.TopicPrefix || enabled != current.Enabled {
		s.disconnect(current.Name)
	}
	s.logger.Info("updated MQTT client", "client_id", id, "name", name, "topic_prefix", input.TopicPrefix, "enabled", enabled)
	return s.Get(ctx, id)
}

// RotatePassword replaces the client's password. A live connection is left
// alone; the old password stops working on the next CONNECT.
func (s *MQTTClientService) RotatePassword(ctx context.Context, id int) (gen.MQTTClientCreated, error) {
	password, err := generateMQTTClientPassword()
	if err != nil {
		return gen.MQTTClientCreated{}, err
	}
	if err := s.repo.SetPasswordHash(ctx, id, HashMQTTClientPassword(password)); err != nil {
		return gen.MQTTClientCreated{}, err
	}
	s.logger.Info("rotated MQTT client password", "client_id", id)
	return s.withPassword(ctx, id, password)
}

func (s *MQTTClientService) Delete(ctx context.Context, id int) error {
	client, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.disconnect(client.Name)
	s.logger.Info("deleted MQTT client", "client_id", id, "name", client.Name)
	return nil
}

// Authenticate checks a CONNECT's credentials. It returns the topic prefix the
// connection is limited to, or why the CONNECT is refused.
func (s *MQTTClientService) Authenticate(ctx context.Context, username string, password []byte) (string, MQTTConnectRefusal) {
	if username == "" {
		return "", MQTTConnectRefusedAuth
	}
	client, err := s.repo.GetByName(ctx, username)
	if err != nil {
		if !errors.Is(err, database.ErrMQTTClientNotFound) {
			s.logger.Error("failed to look up MQTT client", "name", username, "error", err)
		}
		return "", MQTTConnectRefusedAuth
	}
	if !mqttClientPasswordMatches(client.PasswordHash, password) {
		return "", MQTTConnectRefusedAuth
	}
	if !client.Enabled {
		return "", MQTTConnectRefusedDisabled
	}
	if err := s.repo.RecordConnected(ctx, client.ID); err != nil {
		s.logger.Warn("failed to record MQTT client connection", "name", username, "error", err)
	}
	return client.TopicPrefix, ""
}

// IsEnabled reports whether the client still exists and is enabled.
func (s *MQTTClientService) IsEnabled(ctx context.Context, username string) bool {
	client, err := s.repo.GetByName(ctx, username)
	if err != nil {
		if !errors.Is(err, database.ErrMQTTClientNotFound) {
			s.logger.Error("failed to look up MQTT client", "name", username, "error", err)
		}
		return false
	}
	return client.Enabled
}

func (s *MQTTClientService) withPassword(ctx context.Context, id int, password string) (gen.MQTTClientCreated, error) {
	client, err := s.Get(ctx, id)
	if err != nil {
		return gen.MQTTClientCreated{}, err
	}
	return gen.MQTTClientCreated{
		Id:              client.Id,
		Name:            client.Name,
		TopicPrefix:     client.TopicPrefix,
		Enabled:         client.Enabled,
		Connected:       client.Connected,
		LastConnectedAt: client.LastConnectedAt,
		CreatedAt:       client.CreatedAt,
		UpdatedAt:       client.UpdatedAt,
		Password:        &password,
	}, nil
}

func (s *MQTTClientService) connectedUsernames() map[string]bool {
	if s.sessions == nil {
		return nil
	}
	return s.sessions.ConnectedUsernames()
}

func (s *MQTTClientService) disconnect(username string) {
	if s.sessions != nil {
		s.sessions.Disconnect(username)
	}
}

func toGenMQTTClient(client database.MQTTClient, connected bool) gen.MQTTClient {
	return gen.MQTTClient{
		Id:              client.ID,
		Name:            client.Name,
		TopicPrefix:     client.TopicPrefix,
		Enabled:         client.Enabled,
		Connected:       connected,
		LastConnectedAt: client.LastConnectedAt,
		CreatedAt:       client.CreatedAt,
		UpdatedAt:       client.UpdatedAt,
	}
}

// validateMQTTClientInput returns the trimmed name, or an error wrapping
// ErrInvalidMQTTClient.
func validateMQTTClientInput(input gen.MQTTClientInput) (string, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return "", fmt.Errorf("%w: name cannot be empty", ErrInvalidMQTTClient)
	}
	if err := validateMQTTTopicPrefix(input.TopicPrefix); err != nil {
		return "", err
	}
	return name, nil
}

// validateMQTTTopicPrefix accepts a literal topic tree a client can be held
// to with a plain prefix match: no wildcards, no $SYS-style topics, and a
// trailing "/" so "p/" cannot also match "pa/".
func validateMQTTTopicPrefix(prefix string) error {
	switch {
	case prefix == "":
		return fmt.Errorf("%w: topic prefix cannot be empty", ErrInvalidMQTTClient)
	case !strings.HasSuffix(prefix, "/"):
		return fmt.Errorf("%w: topic prefix must end with \"/\"", ErrInvalidMQTTClient)
	case strings.HasPrefix(prefix, "$"):
		return fmt.Errorf("%w: topic prefix must not start with \"$\"", ErrInvalidMQTTClient)
	case strings.ContainsAny(prefix, "+#\x00"):
		return fmt.Errorf("%w: topic prefix must not contain \"+\", \"#\" or NUL", ErrInvalidMQTTClient)
	}
	return nil
}

func generateMQTTClientPassword() (string, error) {
	buf := make([]byte, mqttClientPasswordBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate MQTT client password: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// HashMQTTClientPassword returns the hex SHA-256 digest stored for a password.
func HashMQTTClientPassword(password string) string {
	digest := sha256.Sum256([]byte(password))
	return hex.EncodeToString(digest[:])
}

func mqttClientPasswordMatches(storedHash string, password []byte) bool {
	stored, err := hex.DecodeString(storedHash)
	if err != nil {
		return false
	}
	given := sha256.Sum256(password)
	return subtle.ConstantTimeCompare(stored, given[:]) == 1
}
