ALTER TABLE automation_runs ADD COLUMN cause_run_id INTEGER REFERENCES automation_runs(id) ON DELETE SET NULL;

-- Without it, deleting a run scans every run to clear the ones it caused.
CREATE INDEX idx_automation_runs_cause ON automation_runs(cause_run_id) WHERE cause_run_id IS NOT NULL;
