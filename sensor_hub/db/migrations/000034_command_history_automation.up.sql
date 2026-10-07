-- Runs are pruned sooner than commands by default, so a command records its
-- automation directly to keep showing who sent it once the run is gone.
ALTER TABLE sensor_command_history ADD COLUMN automation_id INTEGER REFERENCES automations(id) ON DELETE SET NULL;

UPDATE sensor_command_history
SET automation_id = (SELECT run.automation_id FROM automation_runs run WHERE run.id = sensor_command_history.automation_run_id)
WHERE automation_run_id IS NOT NULL;

CREATE INDEX idx_sensor_command_history_automation
    ON sensor_command_history(automation_id)
    WHERE automation_id IS NOT NULL;
