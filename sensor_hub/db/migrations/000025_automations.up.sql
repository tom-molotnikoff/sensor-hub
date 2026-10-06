CREATE TABLE automations (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL,
    enabled    INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Each trigger kind owns its own nullable columns, and its CHECK only binds
-- rows of that kind, so a later kind can be added as new columns without
-- rebuilding the table.
CREATE TABLE automation_triggers (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    automation_id    INTEGER NOT NULL REFERENCES automations(id) ON DELETE CASCADE,
    position         INTEGER NOT NULL,
    kind             TEXT NOT NULL,
    at_minute_of_day INTEGER,
    weekdays         INTEGER,
    CHECK (kind <> 'schedule' OR (at_minute_of_day BETWEEN 0 AND 1439 AND weekdays BETWEEN 1 AND 127))
);

CREATE INDEX idx_automation_triggers_automation ON automation_triggers(automation_id, position);

CREATE TABLE automation_steps (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    automation_id INTEGER NOT NULL REFERENCES automations(id) ON DELETE CASCADE,
    position      INTEGER NOT NULL,
    kind          TEXT NOT NULL,
    sensor_id     INTEGER REFERENCES sensors(id) ON DELETE CASCADE,
    property      TEXT,
    value         TEXT,
    CHECK (kind <> 'set' OR (sensor_id IS NOT NULL AND property IS NOT NULL AND value IS NOT NULL))
);

CREATE INDEX idx_automation_steps_automation ON automation_steps(automation_id, position);
CREATE INDEX idx_automation_steps_sensor ON automation_steps(sensor_id);

CREATE TABLE automation_runs (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    automation_id  INTEGER NOT NULL REFERENCES automations(id) ON DELETE CASCADE,
    trigger_id     INTEGER REFERENCES automation_triggers(id) ON DELETE SET NULL,
    trigger_kind   TEXT NOT NULL,
    status         TEXT NOT NULL,
    current_step   INTEGER NOT NULL DEFAULT 0,
    steps_snapshot TEXT NOT NULL,
    started_at     DATETIME NOT NULL,
    finished_at    DATETIME,
    error          TEXT
);

CREATE INDEX idx_automation_runs_automation ON automation_runs(automation_id, started_at DESC);
CREATE INDEX idx_automation_runs_status ON automation_runs(status);
CREATE INDEX idx_automation_runs_trigger ON automation_runs(trigger_id);

CREATE TABLE automation_run_steps (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id      INTEGER NOT NULL REFERENCES automation_runs(id) ON DELETE CASCADE,
    position    INTEGER NOT NULL,
    kind        TEXT NOT NULL,
    outcome     TEXT NOT NULL,
    command_id  INTEGER REFERENCES sensor_command_history(id) ON DELETE SET NULL,
    started_at  DATETIME NOT NULL,
    finished_at DATETIME
);

CREATE INDEX idx_automation_run_steps_run ON automation_run_steps(run_id, position);
CREATE INDEX idx_automation_run_steps_command ON automation_run_steps(command_id) WHERE command_id IS NOT NULL;

ALTER TABLE sensor_command_history ADD COLUMN automation_run_id INTEGER REFERENCES automation_runs(id) ON DELETE SET NULL;

CREATE INDEX idx_sensor_command_history_automation_run
    ON sensor_command_history(automation_run_id)
    WHERE automation_run_id IS NOT NULL;

INSERT OR IGNORE INTO permissions (name, description) VALUES
    ('view_automations', 'View automations and their run history'),
    ('manage_automations', 'Create, update, enable and delete automations');

INSERT OR IGNORE INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.name = 'view_automations'
WHERE r.name IN ('admin', 'user', 'viewer');

INSERT OR IGNORE INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.name = 'manage_automations'
WHERE r.name IN ('admin', 'user');

INSERT OR IGNORE INTO notification_channel_defaults (category, email_enabled, inapp_enabled) VALUES
    ('automation_failure', 1, 1);
