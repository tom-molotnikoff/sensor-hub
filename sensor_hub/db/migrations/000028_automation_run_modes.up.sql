ALTER TABLE automations ADD COLUMN mode TEXT NOT NULL DEFAULT 'single' CHECK (mode IN ('single', 'restart'));

ALTER TABLE automation_runs ADD COLUMN initiated_by_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL;
