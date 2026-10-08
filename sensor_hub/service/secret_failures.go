package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/notifications"
	"example/sensorHub/secrets"
)

// secretFailurePermission is who is told about stored secrets needing
// re-entry: whoever looks after the hub's notification settings.
const secretFailurePermission = "view_notifications_config"

// SecretFailureNotifier creates the secret_failure notification.
type SecretFailureNotifier interface {
	CreateNotification(ctx context.Context, notif notifications.Notification, targetPermission string) (int, error)
}

// SecretOwnerBrokers looks up the broker a secret belongs to, for its name.
type SecretOwnerBrokers interface {
	GetByID(ctx context.Context, id int) (*gen.MQTTBroker, error)
}

// NotifySecretFailures raises one secret_failure notification listing, by
// display name, every owner of a stored secret that did not decrypt at
// startup. When the key check replaced the sealed key this boot, which is why
// they no longer decrypt, the notification says so. It raises nothing when
// every secret decrypted.
func NotifySecretFailures(ctx context.Context, failed []secrets.Ref, replaced *secrets.KeyReplacement, brokers SecretOwnerBrokers, notifier SecretFailureNotifier, logger *slog.Logger) error {
	if len(failed) == 0 {
		return nil
	}
	var names []string
	seen := map[string]bool{}
	for _, ref := range failed {
		if seen[ref.Owner] {
			continue
		}
		seen[ref.Owner] = true
		names = append(names, secretOwnerDisplayName(ctx, ref.Owner, brokers, logger))
	}
	_, err := notifier.CreateNotification(ctx, notifications.Notification{
		Category: notifications.CategorySecretFailure,
		Severity: notifications.SeverityError,
		Title:    "Stored secrets need re-entry",
		Message: fmt.Sprintf("These stored secrets could not be decrypted with the current secret-store key, so they are not used "+
			"until they are entered again: %s.", strings.Join(names, ", ")) + keyReplacementNote(replaced),
	}, secretFailurePermission)
	if err != nil {
		return fmt.Errorf("failed to notify that stored secrets need re-entry: %w", err)
	}
	return nil
}

// secretOwnerDisplayName names a secret's owner as the UI does. An owner
// that cannot be looked up is shown as stored.
func secretOwnerDisplayName(ctx context.Context, owner string, brokers SecretOwnerBrokers, logger *slog.Logger) string {
	if owner == database.SMTPSecretOwner {
		return "the email (SMTP) settings"
	}
	brokerID, ok := database.BrokerIDOfSecretOwner(owner)
	if !ok {
		return owner
	}
	broker, err := brokers.GetByID(ctx, brokerID)
	if err != nil || broker == nil {
		if err != nil {
			logger.Warn("cannot name the broker whose stored secret needs re-entry", "owner", owner, "error", err)
		}
		return owner
	}
	return fmt.Sprintf("MQTT broker %q", broker.Name)
}

// keyReplacementNote explains a sealed key the key check replaced, most of all
// one that is now protected by the host's credential secret, not the TPM.
func keyReplacementNote(replaced *secrets.KeyReplacement) string {
	if replaced == nil {
		return ""
	}
	note := fmt.Sprintf(" The TPM-sealed secret-store key could not be unsealed, so a new key was sealed in its place; "+
		"the old one is kept as %s.", replaced.SetAside)
	if replaced.SealedWith == "host" {
		note += " The TPM could not be used, so the new key is protected by this host's credential secret, not the TPM. " +
			"Seal it with the TPM again once the TPM works."
	}
	return note
}
