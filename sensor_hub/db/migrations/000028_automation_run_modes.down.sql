-- The previous schema has no Run now and no cancelled or skipped runs.
DELETE FROM automation_runs WHERE trigger_kind = 'manual' OR status IN ('cancelled', 'skipped');

ALTER TABLE automation_runs DROP COLUMN initiated_by_user_id;
ALTER TABLE automations DROP COLUMN mode;
