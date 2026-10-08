package service

import (
	"context"
	database "example/sensorHub/db"
	"example/sensorHub/drivers"
	gen "example/sensorHub/gen"
	"example/sensorHub/secrets"
	"fmt"
	"log/slog"
	"strings"
)

// maxTopicLength is the MQTT specification limit for topic filters (UTF-8 encoded).
const maxTopicLength = 65535

// keepPassword is the placeholder a client may send back for a password it
// was never shown. Like an omitted password, it leaves the stored one alone.
const keepPassword = "****"

// BrokerSecrets is the part of the secret store the MQTT service writes broker
// passwords through. It never reads a password back.
type BrokerSecrets interface {
	Set(ctx context.Context, owner, name, value string) error
	Delete(ctx context.Context, owner, name string) error
	Status(owner, name string) secrets.Status
	Forget(owner string)
}

type MQTTService struct {
	brokerRepo database.MQTTBrokerRepositoryInterface
	subRepo    database.MQTTSubscriptionRepositoryInterface
	secrets    BrokerSecrets
	logger     *slog.Logger
	notifier   SubscriptionNotifier
	brokers    BrokerNotifier
}

func NewMQTTService(
	brokerRepo database.MQTTBrokerRepositoryInterface,
	subRepo database.MQTTSubscriptionRepositoryInterface,
	brokerSecrets BrokerSecrets,
	logger *slog.Logger,
) *MQTTService {
	return &MQTTService{
		brokerRepo: brokerRepo,
		subRepo:    subRepo,
		secrets:    brokerSecrets,
		logger:     logger.With("component", "mqtt_service"),
	}
}

// ============================================================================
// Broker operations
// ============================================================================

func (s *MQTTService) AddBroker(ctx context.Context, broker gen.MQTTBroker) (int, error) {
	normaliseBroker(&broker)
	if broker.Type == "embedded" {
		existing, err := s.brokerRepo.GetAll(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to check existing brokers: %w", err)
		}
		for _, b := range existing {
			if b.Type == "embedded" {
				return 0, fmt.Errorf("an embedded broker already exists (id=%d, name=%q)", b.Id, b.Name)
			}
		}
	}
	if err := validateBroker(broker); err != nil {
		return 0, err
	}
	if err := s.checkBrokerNameUnique(ctx, broker.Name, 0); err != nil {
		return 0, err
	}
	if err := s.checkBrokerHostPortUnique(ctx, broker, 0); err != nil {
		return 0, err
	}
	id, err := s.brokerRepo.Add(ctx, broker)
	if err != nil {
		return 0, err
	}
	if password, ok := newPassword(broker.Password); ok && password != "" {
		if err := s.secrets.Set(ctx, database.BrokerSecretOwner(id), database.BrokerPasswordSecret, password); err != nil {
			// Without its password the broker is not what was asked for.
			if deleteErr := s.brokerRepo.Delete(ctx, id); deleteErr != nil {
				s.logger.Error("failed to remove a broker whose password could not be stored", "broker_id", id, "error", deleteErr)
			}
			return 0, fmt.Errorf("failed to store the broker password: %w", err)
		}
	}
	s.brokerChanged(id)
	return id, nil
}

func (s *MQTTService) GetBrokerByID(ctx context.Context, id int) (*gen.MQTTBroker, error) {
	broker, err := s.brokerRepo.GetByID(ctx, id)
	if broker != nil {
		s.describePassword(broker)
	}
	return broker, err
}

func (s *MQTTService) GetBrokerByName(ctx context.Context, name string) (*gen.MQTTBroker, error) {
	broker, err := s.brokerRepo.GetByName(ctx, name)
	if broker != nil {
		s.describePassword(broker)
	}
	return broker, err
}

func (s *MQTTService) GetAllBrokers(ctx context.Context) ([]gen.MQTTBroker, error) {
	brokers, err := s.brokerRepo.GetAll(ctx)
	for i := range brokers {
		s.describePassword(&brokers[i])
	}
	return brokers, err
}

func (s *MQTTService) GetEnabledBrokers(ctx context.Context) ([]gen.MQTTBroker, error) {
	brokers, err := s.brokerRepo.GetEnabled(ctx)
	for i := range brokers {
		s.describePassword(&brokers[i])
	}
	return brokers, err
}

// describePassword gives the broker its password status. The password itself
// never leaves the secret store this way.
func (s *MQTTService) describePassword(broker *gen.MQTTBroker) {
	broker.Password = nil
	if broker.Id == nil {
		return
	}
	status := gen.MQTTBrokerPasswordStatus(s.secrets.Status(database.BrokerSecretOwner(*broker.Id), database.BrokerPasswordSecret))
	broker.PasswordStatus = &status
}

// newPassword reads the password a create or update asks for. ok is false
// when the stored password is to be left as it is: the field is omitted or
// holds the "****" placeholder. An empty password asks for none.
func newPassword(password *string) (string, bool) {
	if password == nil || *password == keepPassword {
		return "", false
	}
	return *password, true
}

func (s *MQTTService) UpdateBroker(ctx context.Context, broker gen.MQTTBroker) error {
	if broker.Id == nil || *broker.Id <= 0 {
		return fmt.Errorf("broker id must be positive")
	}
	normaliseBroker(&broker)
	if err := validateBroker(broker); err != nil {
		return err
	}
	if err := s.checkBrokerNameUnique(ctx, broker.Name, *broker.Id); err != nil {
		return err
	}
	if err := s.checkBrokerHostPortUnique(ctx, broker, *broker.Id); err != nil {
		return err
	}
	if err := s.brokerRepo.Update(ctx, broker); err != nil {
		return err
	}
	// The row has changed even if the password write below fails.
	defer s.brokerChanged(*broker.Id)
	password, ok := newPassword(broker.Password)
	if !ok {
		return nil
	}
	owner := database.BrokerSecretOwner(*broker.Id)
	if password == "" {
		if err := s.secrets.Delete(ctx, owner, database.BrokerPasswordSecret); err != nil {
			return fmt.Errorf("failed to clear the broker password: %w", err)
		}
		return nil
	}
	if err := s.secrets.Set(ctx, owner, database.BrokerPasswordSecret, password); err != nil {
		return fmt.Errorf("failed to store the broker password: %w", err)
	}
	return nil
}

// DeleteBroker removes the broker and, in the same transaction, its password.
func (s *MQTTService) DeleteBroker(ctx context.Context, id int) error {
	if err := s.brokerRepo.Delete(ctx, id); err != nil {
		return err
	}
	s.secrets.Forget(database.BrokerSecretOwner(id))
	if s.brokers != nil {
		s.brokers.OnBrokerDeleted(id)
	}
	return nil
}

// SetBrokerNotifier registers the notifier told about every broker write.
func (s *MQTTService) SetBrokerNotifier(notifier BrokerNotifier) {
	s.brokers = notifier
}

func (s *MQTTService) brokerChanged(id int) {
	if s.brokers != nil {
		s.brokers.OnBrokerChanged(id)
	}
}

// ============================================================================
// Subscription operations
// ============================================================================

func (s *MQTTService) SetSubscriptionNotifier(notifier SubscriptionNotifier) {
	s.notifier = notifier
}

func (s *MQTTService) AddSubscription(ctx context.Context, sub gen.MQTTSubscription) (int, error) {
	if err := s.validateSubscription(ctx, sub); err != nil {
		return 0, err
	}
	id, err := s.subRepo.Add(ctx, sub)
	if err != nil {
		return 0, err
	}
	sub.Id = &id
	if s.notifier != nil {
		s.notifier.OnSubscriptionAdded(sub)
	}
	return id, nil
}

func (s *MQTTService) GetSubscriptionByID(ctx context.Context, id int) (*gen.MQTTSubscription, error) {
	return s.subRepo.GetByID(ctx, id)
}

func (s *MQTTService) GetAllSubscriptions(ctx context.Context) ([]gen.MQTTSubscription, error) {
	return s.subRepo.GetAll(ctx)
}

func (s *MQTTService) GetSubscriptionsByBrokerID(ctx context.Context, brokerID int) ([]gen.MQTTSubscription, error) {
	return s.subRepo.GetByBrokerID(ctx, brokerID)
}

func (s *MQTTService) GetEnabledSubscriptionsByBrokerID(ctx context.Context, brokerID int) ([]gen.MQTTSubscription, error) {
	return s.subRepo.GetEnabledByBrokerID(ctx, brokerID)
}

func (s *MQTTService) UpdateSubscription(ctx context.Context, sub gen.MQTTSubscription) error {
	if sub.Id == nil || *sub.Id <= 0 {
		return fmt.Errorf("subscription id must be positive")
	}
	if err := s.validateSubscription(ctx, sub); err != nil {
		return err
	}
	return s.subRepo.Update(ctx, sub)
}

func (s *MQTTService) DeleteSubscription(ctx context.Context, id int) error {
	// Fetch before deleting so we can notify the connection manager
	sub, err := s.subRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.subRepo.Delete(ctx, id); err != nil {
		return err
	}
	if s.notifier != nil && sub != nil {
		s.notifier.OnSubscriptionRemoved(*sub)
	}
	return nil
}

// ============================================================================
// Validation
// ============================================================================

// normaliseBroker drops any host, port and TLS settings given for the
// embedded broker. The hub reaches it in-process, so it has no address and no
// connection to secure. An external broker's blank CA means none.
func normaliseBroker(broker *gen.MQTTBroker) {
	if broker.Type == "embedded" {
		broker.Host = nil
		broker.Port = nil
		broker.Tls = nil
		broker.CaCertPem = nil
	}
	if !brokerCAGiven(broker.CaCertPem) {
		broker.CaCertPem = nil
	}
}

func validateBroker(broker gen.MQTTBroker) error {
	if strings.TrimSpace(broker.Name) == "" {
		return fmt.Errorf("broker name cannot be empty")
	}
	if broker.Type != "embedded" && broker.Type != "external" {
		return fmt.Errorf("broker type must be 'embedded' or 'external'")
	}
	if broker.Type == "embedded" {
		return nil
	}
	if broker.Host == nil || strings.TrimSpace(*broker.Host) == "" {
		return fmt.Errorf("broker host cannot be empty")
	}
	if broker.Port == nil || *broker.Port <= 0 || *broker.Port > 65535 {
		return fmt.Errorf("broker port must be between 1 and 65535")
	}
	if broker.CaCertPem != nil {
		if _, err := ParseBrokerCA(*broker.CaCertPem); err != nil {
			return err
		}
	}
	return nil
}

// checkBrokerNameUnique ensures no other broker has the same name (case-insensitive).
// excludeID is the broker being updated (0 for new brokers).
func (s *MQTTService) checkBrokerNameUnique(ctx context.Context, name string, excludeID int) error {
	existing, err := s.brokerRepo.GetByName(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to check broker name uniqueness: %w", err)
	}
	if existing != nil && (existing.Id == nil || *existing.Id != excludeID) {
		return fmt.Errorf("broker name %q is already in use (id=%d)", existing.Name, *existing.Id)
	}
	return nil
}

// checkBrokerHostPortUnique ensures no other external broker targets the same
// host:port. excludeID is the broker being updated (0 for new brokers).
func (s *MQTTService) checkBrokerHostPortUnique(ctx context.Context, broker gen.MQTTBroker, excludeID int) error {
	if broker.Type == "embedded" {
		return nil
	}
	all, err := s.brokerRepo.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to check broker host:port uniqueness: %w", err)
	}
	host, port := *broker.Host, *broker.Port
	for _, b := range all {
		if b.Type == "embedded" || (b.Id != nil && *b.Id == excludeID) {
			continue
		}
		if strings.EqualFold(*b.Host, host) && *b.Port == port {
			return fmt.Errorf("broker host:port %s:%d is already in use by broker %q (id=%d)", host, port, b.Name, *b.Id)
		}
	}
	return nil
}

func (s *MQTTService) validateSubscription(ctx context.Context, sub gen.MQTTSubscription) error {
	if strings.TrimSpace(sub.TopicPattern) == "" {
		return fmt.Errorf("topic pattern cannot be empty")
	}
	if sub.DriverType == "" {
		return fmt.Errorf("driver type cannot be empty")
	}
	if sub.BrokerId <= 0 {
		return fmt.Errorf("broker id must be positive")
	}

	// Validate the driver exists and is a PushDriver
	driver, ok := drivers.Get(sub.DriverType)
	if !ok {
		return fmt.Errorf("unknown driver type: %s", sub.DriverType)
	}
	if _, isPush := driver.(drivers.PushDriver); !isPush {
		return fmt.Errorf("driver %s is not an MQTT push driver", sub.DriverType)
	}

	// Validate the broker exists
	broker, err := s.brokerRepo.GetByID(ctx, sub.BrokerId)
	if err != nil {
		return fmt.Errorf("broker not found: %w", err)
	}
	if broker == nil {
		return fmt.Errorf("broker not found: no broker with id %d", sub.BrokerId)
	}

	// Basic MQTT topic pattern validation
	if err := validateTopicPattern(sub.TopicPattern); err != nil {
		return err
	}

	// Check for overlapping subscriptions on the same broker
	excludeSubID := 0
	if sub.Id != nil {
		excludeSubID = *sub.Id
	}
	if err := s.checkTopicOverlap(ctx, sub.BrokerId, sub.TopicPattern, excludeSubID); err != nil {
		return err
	}

	return nil
}

func validateTopicPattern(pattern string) error {
	if strings.Contains(pattern, " ") {
		return fmt.Errorf("topic pattern must not contain spaces")
	}
	if len(pattern) > maxTopicLength {
		return fmt.Errorf("topic pattern exceeds maximum length of %d bytes", maxTopicLength)
	}
	parts := strings.Split(pattern, "/")
	for i, part := range parts {
		if part == "#" && i != len(parts)-1 {
			return fmt.Errorf("multi-level wildcard (#) must be the last segment in topic pattern")
		}
	}
	return nil
}

// checkTopicOverlap verifies that a new or updated subscription does not overlap
// with existing subscriptions on the same broker, which would cause duplicate
// message processing. excludeID is the subscription being updated (0 for new).
func (s *MQTTService) checkTopicOverlap(ctx context.Context, brokerID int, newTopic string, excludeID int) error {
	existing, err := s.subRepo.GetByBrokerID(ctx, brokerID)
	if err != nil {
		return fmt.Errorf("failed to check topic overlap: %w", err)
	}
	for _, sub := range existing {
		if sub.Id != nil && *sub.Id == excludeID {
			continue
		}
		if topicsOverlap(sub.TopicPattern, newTopic) {
			subID := 0
			if sub.Id != nil {
				subID = *sub.Id
			}
			return fmt.Errorf("topic pattern %q overlaps with existing subscription %q (id=%d) on this broker; overlapping topics cause duplicate message processing", newTopic, sub.TopicPattern, subID)
		}
	}
	return nil
}

// topicsOverlap returns true if two MQTT topic filters could match any of the
// same concrete topics. It checks both directions: whether pattern A could
// match topics that B also matches, and vice versa.
func topicsOverlap(a, b string) bool {
	return topicCouldMatch(a, b) || topicCouldMatch(b, a)
}

// topicCouldMatch returns true if a message matching concrete segments of
// pattern `sub` could also be delivered to `filter`. This handles MQTT's `+`
// (single-level) and `#` (multi-level) wildcards.
func topicCouldMatch(filter, sub string) bool {
	filterParts := strings.Split(filter, "/")
	subParts := strings.Split(sub, "/")

	for i := 0; i < len(filterParts); i++ {
		if filterParts[i] == "#" {
			return true // # matches everything from here on
		}
		if i >= len(subParts) {
			return false // filter has more segments than sub, no overlap
		}
		if filterParts[i] == "+" || subParts[i] == "+" || subParts[i] == "#" {
			if subParts[i] == "#" {
				return true
			}
			continue // single-level wildcard matches any single segment
		}
		if filterParts[i] != subParts[i] {
			return false // literal segments don't match
		}
	}

	return len(filterParts) == len(subParts)
}
