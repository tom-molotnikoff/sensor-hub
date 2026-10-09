// Package email sends the hub's alert and notification emails through an
// SMTP server whose settings are kept in the database and whose password is
// kept in the secret store.
package email

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"sync/atomic"
	"time"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/secrets"
	"example/sensorHub/telemetry"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// TestSubject is the subject of the email a test send delivers.
const TestSubject = "Sensor Hub test email"

var (
	// ErrInvalidSettings wraps the reason settings were refused.
	ErrInvalidSettings = errors.New("invalid email settings")
	// ErrNoRecipient means the user a test email is for has no address.
	ErrNoRecipient = errors.New("you have no email address to send a test email to; add one to your account first")
	// ErrNotConfigured means there is no SMTP host or no usable password, so
	// the hub sends nothing.
	ErrNotConfigured = errors.New("email is not configured")
)

// SendError is a send the SMTP server, or the way to it, failed. Its text is
// what the server said.
type SendError struct {
	Err error
}

func (e *SendError) Error() string { return e.Err.Error() }
func (e *SendError) Unwrap() error { return e.Err }

// SettingsRepository holds the email_settings row.
type SettingsRepository interface {
	Get(ctx context.Context) (database.EmailSettings, error)
	Save(ctx context.Context, s database.EmailSettings) error
	RecordSent(ctx context.Context, at time.Time) error
	RecordFailure(ctx context.Context, sendErr string) error
}

// Secrets is the part of the secret store the SMTP password lives in.
type Secrets interface {
	Get(ctx context.Context, owner, name string) (string, secrets.Status, error)
	Set(ctx context.Context, owner, name, value string) error
	Delete(ctx context.Context, owner, name string) error
	Status(owner, name string) secrets.Status
}

// Service reads and writes the email settings and sends through them.
type Service struct {
	repo      SettingsRepository
	secrets   Secrets
	transport transport
	logger    *slog.Logger
	sends     metric.Int64Counter
	now       func() time.Time

	// warnedNotConfigured is set once the hub has logged that it is sending
	// no email, so it says so once per start rather than once per message.
	warnedNotConfigured atomic.Bool
}

func NewService(repo SettingsRepository, store Secrets, logger *slog.Logger) *Service {
	return &Service{
		repo:    repo,
		secrets: store,
		logger:  logger.With("component", "email"),
		sends:   sendCounter(),
		now:     time.Now,
	}
}

// sendCounter counts sends by result, "sent" or "failed". Prometheus shows it
// as sensor_hub_email_send_total.
func sendCounter() metric.Int64Counter {
	counter, _ := telemetry.Meter("email").Int64Counter("sensor_hub.email.send",
		metric.WithDescription("Emails the hub tried to send through its SMTP server, by result"),
		metric.WithUnit("{message}"))
	return counter
}

// Settings returns the settings as the API shows them: the password's status
// and never the password.
func (s *Service) Settings(ctx context.Context) (gen.EmailSettings, error) {
	stored, err := s.repo.Get(ctx)
	if err != nil {
		return gen.EmailSettings{}, err
	}
	status := gen.EmailSettingsPasswordStatus(s.secrets.Status(database.SMTPSecretOwner, database.SMTPPasswordSecret))
	return gen.EmailSettings{
		Host:           stored.Host,
		Port:           stored.Port,
		Security:       gen.EmailSettingsSecurity(stored.Security),
		Username:       stored.Username,
		FromAddress:    stored.FromAddress,
		PasswordStatus: &status,
		LastError:      stored.LastError,
		LastSentAt:     stored.LastSentAt,
	}, nil
}

// Update replaces the settings and sets, keeps or clears the password as
// the password field asks, then returns the settings as Settings does.
func (s *Service) Update(ctx context.Context, settings gen.EmailSettings) (gen.EmailSettings, error) {
	stored, err := validate(settings)
	if err != nil {
		return gen.EmailSettings{}, err
	}
	if err := s.repo.Save(ctx, stored); err != nil {
		return gen.EmailSettings{}, err
	}
	if password, change := secrets.Requested(settings.Password); change {
		if password == "" {
			err = s.secrets.Delete(ctx, database.SMTPSecretOwner, database.SMTPPasswordSecret)
		} else {
			err = s.secrets.Set(ctx, database.SMTPSecretOwner, database.SMTPPasswordSecret, password)
		}
		if err != nil {
			return gen.EmailSettings{}, fmt.Errorf("failed to store the SMTP password: %w", err)
		}
	}
	return s.Settings(ctx)
}

func validate(settings gen.EmailSettings) (database.EmailSettings, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrInvalidSettings, fmt.Sprintf(format, args...))
	}
	host := strings.TrimSpace(settings.Host)
	if host == "" {
		return database.EmailSettings{}, invalid("host must not be empty")
	}
	if settings.Port < 1 || settings.Port > 65535 {
		return database.EmailSettings{}, invalid("port must be from 1 to 65535, got %d", settings.Port)
	}
	security := string(settings.Security)
	switch security {
	case SecuritySTARTTLS, SecurityImplicitTLS, SecurityNone:
	default:
		return database.EmailSettings{}, invalid("security must be starttls, implicit_tls or none, got %q", security)
	}
	from := strings.TrimSpace(settings.FromAddress)
	if parsed, err := mail.ParseAddress(from); err != nil || parsed.Address != from {
		return database.EmailSettings{}, invalid("from_address must be an email address such as alerts@example.com, got %q", settings.FromAddress)
	}
	username := strings.TrimSpace(settings.Username)
	if strings.ContainsAny(username, "\r\n") {
		return database.EmailSettings{}, invalid("username must be on one line")
	}
	return database.EmailSettings{Host: host, Port: settings.Port, Security: security, Username: username, FromAddress: from}, nil
}

// SendTest sends the test email to recipient, the caller's own address, and
// records the outcome as any send is recorded. It returns ErrNoRecipient,
// ErrNotConfigured or a *SendError.
func (s *Service) SendTest(ctx context.Context, recipient string) error {
	if strings.TrimSpace(recipient) == "" {
		return ErrNoRecipient
	}
	settings, srv, err := s.ready(ctx)
	if err != nil {
		return err
	}
	body := fmt.Sprintf("This is a test email from Sensor Hub.\n\n"+
		"It was sent through %s, so alert and notification emails can be sent too.\n", settings.Host)
	return s.send(ctx, settings, srv, recipient, TestSubject, body)
}

// SendNotification sends one alert or notification email. When email is not
// configured it sends nothing and says so once per hub start; the
// notification still reaches the user in the app.
func (s *Service) SendNotification(recipient, title, message, category string) error {
	ctx := context.Background()
	settings, srv, err := s.ready(ctx)
	if errors.Is(err, ErrNotConfigured) {
		if s.warnedNotConfigured.CompareAndSwap(false, true) {
			s.logger.Warn("email is not configured, so alert and notification emails are not sent; set the SMTP settings on the Alerts & Notifications page",
				"reason", err.Error())
		}
		return nil
	}
	if err != nil {
		return err
	}
	return s.send(ctx, settings, srv, recipient, fmt.Sprintf("[%s] %s", category, title), message)
}

// ready loads what a send needs, or says why there is nothing to send with.
func (s *Service) ready(ctx context.Context) (database.EmailSettings, server, error) {
	settings, err := s.repo.Get(ctx)
	if err != nil {
		return database.EmailSettings{}, server{}, err
	}
	if settings.Host == "" {
		return database.EmailSettings{}, server{}, fmt.Errorf("%w: no SMTP host is set", ErrNotConfigured)
	}
	password, status, err := s.secrets.Get(ctx, database.SMTPSecretOwner, database.SMTPPasswordSecret)
	if err != nil {
		return database.EmailSettings{}, server{}, err
	}
	switch status {
	case secrets.StatusSet:
	case secrets.StatusNeedsReentry:
		return database.EmailSettings{}, server{}, fmt.Errorf("%w: the stored SMTP password could not be decrypted and needs to be entered again", ErrNotConfigured)
	default:
		return database.EmailSettings{}, server{}, fmt.Errorf("%w: no SMTP password is set", ErrNotConfigured)
	}
	return settings, server{
		host:     settings.Host,
		port:     settings.Port,
		security: settings.Security,
		username: settings.Username,
		password: password,
	}, nil
}

func (s *Service) send(ctx context.Context, settings database.EmailSettings, srv server, recipient, subject, body string) error {
	now := s.now()
	message := buildMessage(settings.FromAddress, recipient, subject, body, now)
	err := s.transport.send(ctx, srv, settings.FromAddress, recipient, message)
	if err != nil {
		s.sends.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "failed")))
		if recordErr := s.repo.RecordFailure(ctx, err.Error()); recordErr != nil {
			s.logger.Error("could not record a failed email", "error", recordErr)
		}
		return &SendError{Err: err}
	}
	s.sends.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "sent")))
	if recordErr := s.repo.RecordSent(ctx, now); recordErr != nil {
		s.logger.Error("could not record a sent email", "error", recordErr)
	}
	s.logger.Info("email sent", "recipient", recipient, "subject", subject)
	return nil
}
