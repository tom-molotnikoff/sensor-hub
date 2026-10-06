ALTER TABLE automation_steps ADD COLUMN wait_seconds INTEGER CHECK (kind <> 'wait' OR (wait_seconds IS NOT NULL AND wait_seconds >= 1));

-- Persisted so that a trigger that came due while the hub was down can be
-- found on startup.
ALTER TABLE automation_triggers ADD COLUMN next_due_at DATETIME;

ALTER TABLE automation_runs ADD COLUMN resume_at DATETIME;
ALTER TABLE automation_runs ADD COLUMN due_at DATETIME;
ALTER TABLE automation_runs ADD COLUMN past_grace_seconds INTEGER;
