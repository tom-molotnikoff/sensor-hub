UPDATE permissions SET name = 'manage_oauth', description = 'Manage OAuth credentials and re-authorize'
    WHERE name = 'manage_email';

DROP TABLE email_settings;
