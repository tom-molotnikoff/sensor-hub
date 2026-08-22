package smtp

import (
	"fmt"
	"log/slog"
	"net/smtp"

	appProps "example/sensorHub/application_properties"
	"example/sensorHub/oauth"
)

type SMTPNotifier struct {
	logger *slog.Logger
}

func NewSMTPNotifier(logger *slog.Logger) *SMTPNotifier {
	return &SMTPNotifier{logger: logger.With("component", "smtp_notifier")}
}

func (n *SMTPNotifier) SendNotification(recipient, title, message, category string) error {
	if !oauth.OauthSet {
		n.logger.Warn("OAuth not configured, skipping notification email", "recipient", recipient)
		return nil
	}

	// Get current token from OAuth service (may have been refreshed)
	token := oauth.OauthToken
	if token == nil {
		n.logger.Warn("OAuth token is nil, skipping notification email", "recipient", recipient)
		return nil
	}

	smtpUser := appProps.AppConfig().SMTPUser
	auth := &oauth.XOauth2Auth{
		Username:    smtpUser,
		AccessToken: token.AccessToken,
	}

	subject := fmt.Sprintf("[%s] %s", category, title)
	msg := "From: " + smtpUser + "\n" +
		"To: " + recipient + "\n" +
		"Subject: " + subject + "\n\n" +
		message

	err := smtp.SendMail("smtp.gmail.com:587", auth, smtpUser, []string{recipient}, []byte(msg))
	if err != nil {
		return fmt.Errorf("failed to send notification email via SMTP: %w", err)
	}

	n.logger.Info("notification email sent", "recipient", recipient, "title", title)
	return nil
}
