-- The cleanup cycle prunes both tables by age, which would otherwise scan
-- each of them in full every cycle.
CREATE INDEX idx_automation_runs_finished ON automation_runs(finished_at) WHERE finished_at IS NOT NULL;
CREATE INDEX idx_sensor_command_history_sent_at ON sensor_command_history(sent_at);
