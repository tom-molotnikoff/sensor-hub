-- The previous schema cannot hold a wait step or a waiting or missed run.
DELETE FROM automations WHERE id IN (SELECT automation_id FROM automation_steps WHERE kind = 'wait');
DELETE FROM automation_runs WHERE status = 'missed';
UPDATE automation_runs
SET status = 'failed', error = 'the run was waiting when wait steps were removed', finished_at = CURRENT_TIMESTAMP
WHERE status = 'waiting';

ALTER TABLE automation_runs DROP COLUMN past_grace_seconds;
ALTER TABLE automation_runs DROP COLUMN due_at;
ALTER TABLE automation_runs DROP COLUMN resume_at;
ALTER TABLE automation_triggers DROP COLUMN next_due_at;
ALTER TABLE automation_steps DROP COLUMN wait_seconds;
