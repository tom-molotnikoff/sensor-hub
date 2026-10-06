-- The previous schema cannot hold an interval trigger.
DELETE FROM automations WHERE id IN (SELECT automation_id FROM automation_triggers WHERE kind = 'interval');

ALTER TABLE automation_triggers DROP COLUMN interval_seconds;
