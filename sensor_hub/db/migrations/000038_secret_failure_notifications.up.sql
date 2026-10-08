-- A stored secret that no longer decrypts, such as after the secret-store key
-- was replaced, raises one secret_failure notification when the hub starts.
-- It goes in-app only by default: email may be what needs re-entry.
INSERT OR IGNORE INTO notification_channel_defaults (category, email_enabled, inapp_enabled) VALUES
    ('secret_failure', 0, 1);
