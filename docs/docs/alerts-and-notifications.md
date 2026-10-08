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
- Email: notifications are sent to the user's email address via Gmail SMTP. This requires OAuth 2.0 configuration (see [Installation](installation#set-up-oauth-for-email-notifications-optional)).

## Notification preferences

Each user can configure which categories of notifications they receive and through which channels.

The **Stored Secrets** category (`secret_failure`) tells users with `view_notifications_config` when the hub started with stored secrets, such as broker passwords, that it could not decrypt and that need entering again (see [Secrets that need re-entry](configuration#secrets-that-need-re-entry)). It is in-app only by default.

Email notifications require a working OAuth configuration. If OAuth is not configured, email delivery is silently skipped.

## Email notification setup

To enable email notifications:

1. Complete the OAuth setup as described in the [Installation guide](installation#set-up-oauth-for-email-notifications-optional)
2. Ensure `smtp.user` is set in `configuration/smtp.properties` to the Gmail address that will send notifications
3. The OAuth token is refreshed automatically in the background (default: every 30 minutes)

Once configured, the OAuth status is displayed on the Alerts and Notifications page under the OAuth Configuration card.