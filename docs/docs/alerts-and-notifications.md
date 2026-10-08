---
id: alerts-and-notifications
title: Alerts and Notifications
sidebar_position: 7
---

# Alerts and Notifications

Sensor Hub provides an alerting system that monitors sensor readings against configurable rules.

## Alert rules

Alert rules are configured per sensor. Each sensor can have multiple alert rules.

There are two types of alert rules:

- Numeric range: triggers when a reading exceeds a high threshold or falls below a low threshold. Use this for temperature alerts (e.g., alert when temperature drops below 15 degrees or exceeds 30 degrees).
- Status-based: triggers when a sensor reports a specific status value.

## Notifications

Notifications are the delivery mechanism for alerts and system events. The current implementation supports two channels:

- In-app: notifications appear in the notification bell in the UI header. New notifications are pushed to connected clients in real time via WebSocket.
- Email: notifications are sent to the user's email address through an SMTP server you configure (see [Email](#email)).

## Notification preferences

Each user can configure which categories of notifications they receive and through which channels.

The **Stored Secrets** category (`secret_failure`) tells users with `view_notifications_config` when the hub started with stored secrets, such as broker passwords, that it could not decrypt and that need entering again (see [Secrets that need re-entry](configuration#secrets-that-need-re-entry)). It is in-app only by default.

Email notifications need the SMTP settings below. Until a host is set and a password is stored, the hub sends no email: it logs one warning after each start and still delivers every notification in the app.

## Email

The hub sends email through any SMTP server that takes a username and password. Set it up on the Alerts & Notifications page, in the Email card, which needs the `manage_email` permission. From the CLI, `sensor-hub email show`, `email set` and `email test` do the same (see [Email](cli-tool#email)).

| Setting | What it is |
|---|---|
| Host and port | The SMTP server, such as `email-smtp.eu-west-1.amazonaws.com` and `587` |
| Security | `starttls` (the default, usually port 587): the hub starts unencrypted, then requires STARTTLS and stops if the server does not offer it. `implicit_tls` (usually port 465): TLS from the first byte. `none`: nothing is encrypted, the password included, so use it only for a relay on the same machine or a network you trust |
| Username | The SMTP login. Leave it empty for a server that takes mail without one |
| From address | The address the emails come from. Most providers only send from an address or domain you have verified with them |
| Password | Write-only. The hub stores it encrypted in the secret store and never shows it again. Leave the field empty to keep the stored password, or use Clear to remove it |

With either TLS mode the hub checks the server's certificate against the system's trusted roots and that it was issued for the host, before it sends the password.

**Send test email** sends "Sensor Hub test email" to your own account's email address with the saved settings. A failure shows the SMTP server's error, such as `535 5.7.8 Authentication credentials invalid`. The card also shows when an email was last sent and the error the last send failed with. Alerts and notifications update both too.

If the stored password cannot be decrypted, for example after the secret-store key was replaced, the card shows **Needs re-entry** and no email is sent until the password is entered again (see [Secrets that need re-entry](configuration#secrets-that-need-re-entry)).

### Getting a send-only credential

The SMTP password sits on the hub, so give it a credential that can only send mail, and that you can revoke without touching anything else:

- **Amazon SES**: create SMTP credentials in the SES console. They are an IAM user limited to sending through SES, separate from your AWS sign-in.
- **A transactional email provider** such as Postmark, Mailgun, Brevo or Resend: each gives SMTP credentials, or an API key used as the SMTP password, scoped to sending from a domain you verify.
- **To stay on Gmail**: create a separate Google account used only for sending alerts, turn on 2-Step Verification for it, and create an app password for the hub. Use `smtp.gmail.com`, port `587`, `starttls`, the account's address as username and from address, and the app password as the password.

Never use your main mailbox. A Gmail app password is not limited to sending: it gives whole-account access, so anyone who obtains it can read, send and delete that account's mail. That is why it belongs on an account that holds nothing else.
