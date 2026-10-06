DELETE FROM notifications WHERE category = 'automation_failure';
DELETE FROM notification_channel_preferences WHERE category = 'automation_failure';
DELETE FROM notification_channel_defaults WHERE category = 'automation_failure';

DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE name IN ('view_automations', 'manage_automations')
);
DELETE FROM permissions WHERE name IN ('view_automations', 'manage_automations');

DROP INDEX IF EXISTS idx_sensor_command_history_automation_run;
ALTER TABLE sensor_command_history DROP COLUMN automation_run_id;

DROP TABLE automation_run_steps;
DROP TABLE automation_runs;
DROP TABLE automation_steps;
DROP TABLE automation_triggers;
DROP TABLE automations;
