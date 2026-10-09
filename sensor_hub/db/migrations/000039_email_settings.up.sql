-- Alert and notification emails go through an SMTP server set from the UI,
-- in place of Gmail OAuth. The one row holds the settings; the password is in
-- the secrets table as smtp/password. last_error and last_sent_at record how
-- the last send went.
--
-- The row starts with Gmail's submission server, which is where 1.5.x sent
-- from. A startup step fills username and from_address from the old smtp.user
-- property, which the SQL cannot read, and sets updated_at, so a NULL
-- updated_at marks a row it has not run on yet.
CREATE TABLE email_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    host TEXT,
    port INTEGER,
    security TEXT NOT NULL CHECK (security IN ('starttls', 'implicit_tls', 'none')) DEFAULT 'starttls',
    username TEXT,
    from_address TEXT,
    last_error TEXT,
    last_sent_at TEXT,
    updated_at TEXT
);

INSERT INTO email_settings (id, host, port, security) VALUES (1, 'smtp.gmail.com', 587, 'starttls');

-- The OAuth permission now governs the email settings. Renaming it keeps
-- every role that held it.
UPDATE permissions SET name = 'manage_email', description = 'Manage the email (SMTP) settings and send a test email'
    WHERE name = 'manage_oauth';
