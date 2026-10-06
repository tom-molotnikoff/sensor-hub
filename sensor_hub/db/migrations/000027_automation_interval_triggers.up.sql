ALTER TABLE automation_triggers ADD COLUMN interval_seconds INTEGER CHECK (kind <> 'interval' OR (interval_seconds IS NOT NULL AND interval_seconds >= 60));
