package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"example/sensorHub/automation"
)

var _ automation.Store = (*AutomationRepository)(nil)

type AutomationRepository struct {
	db     *Handles
	logger *slog.Logger
}

func NewAutomationRepository(db *Handles, logger *slog.Logger) *AutomationRepository {
	return &AutomationRepository{db: db, logger: logger.With("component", "automation_repository")}
}

func (r *AutomationRepository) ListAutomations(ctx context.Context) ([]automation.Automation, error) {
	return r.queryAutomations(ctx, "")
}

func (r *AutomationRepository) GetAutomation(ctx context.Context, id int) (automation.Automation, error) {
	automations, err := r.queryAutomations(ctx, "WHERE id = ?", id)
	if err != nil {
		return automation.Automation{}, err
	}
	if len(automations) == 0 {
		return automation.Automation{}, automation.ErrNotFound
	}
	return automations[0], nil
}

func (r *AutomationRepository) CreateAutomation(ctx context.Context, a automation.Automation) (automation.Automation, error) {
	now := time.Now().UTC()
	id, err := r.inTx(ctx, func(tx *sql.Tx) (int, error) {
		result, err := tx.ExecContext(ctx,
			"INSERT INTO automations (name, enabled, created_at, updated_at) VALUES (?, ?, ?, ?)",
			a.Name, a.Enabled, now, now)
		if err != nil {
			return 0, fmt.Errorf("insert automation: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("read automation id: %w", err)
		}
		return int(id), insertDefinition(ctx, tx, int(id), a)
	})
	if err != nil {
		return automation.Automation{}, err
	}
	return r.GetAutomation(ctx, id)
}

// UpdateAutomation replaces the automation's triggers and steps, so they get
// new IDs. Runs keep their copy of the steps they started with.
func (r *AutomationRepository) UpdateAutomation(ctx context.Context, a automation.Automation) (automation.Automation, error) {
	_, err := r.inTx(ctx, func(tx *sql.Tx) (int, error) {
		result, err := tx.ExecContext(ctx,
			"UPDATE automations SET name = ?, enabled = ?, updated_at = ? WHERE id = ?",
			a.Name, a.Enabled, time.Now().UTC(), a.ID)
		if err := requireRow(result, err, "update automation"); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM automation_triggers WHERE automation_id = ?", a.ID); err != nil {
			return 0, fmt.Errorf("delete automation triggers: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM automation_steps WHERE automation_id = ?", a.ID); err != nil {
			return 0, fmt.Errorf("delete automation steps: %w", err)
		}
		return a.ID, insertDefinition(ctx, tx, a.ID, a)
	})
	if err != nil {
		return automation.Automation{}, err
	}
	return r.GetAutomation(ctx, a.ID)
}

func (r *AutomationRepository) SetAutomationEnabled(ctx context.Context, id int, enabled bool) (automation.Automation, error) {
	result, err := r.db.Writer.ExecContext(ctx,
		"UPDATE automations SET enabled = ?, updated_at = ? WHERE id = ?", enabled, time.Now().UTC(), id)
	if err := requireRow(result, err, "update automation enabled"); err != nil {
		return automation.Automation{}, err
	}
	return r.GetAutomation(ctx, id)
}

// The schema cascades the delete to triggers, steps, runs and run steps, and
// clears automation_run_id on the commands its runs sent.
func (r *AutomationRepository) DeleteAutomation(ctx context.Context, id int) error {
	result, err := r.db.Writer.ExecContext(ctx, "DELETE FROM automations WHERE id = ?", id)
	return requireRow(result, err, "delete automation")
}

func (r *AutomationRepository) RunStates(ctx context.Context) (map[int]automation.RunState, error) {
	rows, err := r.db.Reader.QueryContext(ctx, `SELECT a.id,
			EXISTS (SELECT 1 FROM automation_runs r WHERE r.automation_id = a.id AND r.status = 'running'),
			COALESCE((SELECT r.status = 'failed' FROM automation_runs r
				WHERE r.automation_id = a.id AND r.status IN ('succeeded', 'failed')
				ORDER BY r.started_at DESC, r.id DESC LIMIT 1), 0)
		FROM automations a`)
	if err != nil {
		return nil, fmt.Errorf("query automation run states: %w", err)
	}
	defer rows.Close()

	states := make(map[int]automation.RunState)
	for rows.Next() {
		var id int
		var state automation.RunState
		if err := rows.Scan(&id, &state.Running, &state.LastRunFailed); err != nil {
			return nil, fmt.Errorf("scan automation run state: %w", err)
		}
		states[id] = state
	}
	return states, rows.Err()
}

func (r *AutomationRepository) CreateRun(ctx context.Context, run automation.Run) (int, error) {
	snapshot, err := json.Marshal(run.Steps)
	if err != nil {
		return 0, fmt.Errorf("encode run steps: %w", err)
	}
	result, err := r.db.Writer.ExecContext(ctx, `INSERT INTO automation_runs
		(automation_id, trigger_id, trigger_kind, status, current_step, steps_snapshot, started_at)
		VALUES (?, ?, ?, ?, 0, ?, ?)`,
		run.AutomationID, run.TriggerID, run.TriggerKind, run.Status, string(snapshot), run.StartedAt)
	if err != nil {
		return 0, fmt.Errorf("insert automation run: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read automation run id: %w", err)
	}
	return int(id), nil
}

func (r *AutomationRepository) StartRunStep(ctx context.Context, runID int, position int, kind automation.StepKind, at time.Time) (int, error) {
	return r.inTx(ctx, func(tx *sql.Tx) (int, error) {
		result, err := tx.ExecContext(ctx, "UPDATE automation_runs SET current_step = ? WHERE id = ?", position, runID)
		if err := requireRow(result, err, "move automation run on"); err != nil {
			if errors.Is(err, automation.ErrNotFound) {
				return 0, automation.ErrRunGone
			}
			return 0, err
		}
		result, err = tx.ExecContext(ctx, `INSERT INTO automation_run_steps (run_id, position, kind, outcome, started_at)
			VALUES (?, ?, ?, ?, ?)`, runID, position, kind, automation.StepRunning, at)
		if err != nil {
			return 0, fmt.Errorf("insert automation run step: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("read automation run step id: %w", err)
		}
		return int(id), nil
	})
}

func (r *AutomationRepository) FinishRunStep(ctx context.Context, stepID int, outcome automation.StepOutcome, commandID *int, at time.Time) error {
	_, err := r.db.Writer.ExecContext(ctx,
		"UPDATE automation_run_steps SET outcome = ?, command_id = ?, finished_at = ? WHERE id = ?",
		outcome, commandID, at, stepID)
	if err != nil {
		return fmt.Errorf("finish automation run step: %w", err)
	}
	return nil
}

func (r *AutomationRepository) FinishRun(ctx context.Context, runID int, status automation.RunStatus, message *string, at time.Time) error {
	_, err := r.db.Writer.ExecContext(ctx,
		"UPDATE automation_runs SET status = ?, error = ?, finished_at = ? WHERE id = ?",
		status, message, at, runID)
	if err != nil {
		return fmt.Errorf("finish automation run: %w", err)
	}
	return nil
}

func (r *AutomationRepository) FailRunningRuns(ctx context.Context, message string, at time.Time) (int, error) {
	result, err := r.db.Writer.ExecContext(ctx,
		"UPDATE automation_runs SET status = ?, error = ?, finished_at = ? WHERE status = ?",
		automation.RunFailed, message, at, automation.RunRunning)
	if err != nil {
		return 0, fmt.Errorf("fail running automation runs: %w", err)
	}
	count, err := result.RowsAffected()
	return int(count), err
}

func (r *AutomationRepository) ListRuns(ctx context.Context, automationID int) ([]automation.Run, error) {
	rows, err := r.db.Reader.QueryContext(ctx, `SELECT id, automation_id, trigger_id, trigger_kind, status,
			current_step, steps_snapshot, started_at, finished_at, error
		FROM automation_runs WHERE automation_id = ?
		ORDER BY started_at DESC, id DESC`, automationID)
	if err != nil {
		return nil, fmt.Errorf("query automation runs: %w", err)
	}
	defer rows.Close()

	runs := make([]automation.Run, 0)
	positions := make(map[int]int)
	for rows.Next() {
		var run automation.Run
		var triggerID sql.NullInt64
		var snapshot string
		var startedAt SQLiteTime
		var finishedAt NullSQLiteTime
		var message sql.NullString
		if err := rows.Scan(&run.ID, &run.AutomationID, &triggerID, &run.TriggerKind, &run.Status,
			&run.CurrentStep, &snapshot, &startedAt, &finishedAt, &message); err != nil {
			return nil, fmt.Errorf("scan automation run: %w", err)
		}
		if err := json.Unmarshal([]byte(snapshot), &run.Steps); err != nil {
			return nil, fmt.Errorf("decode steps of automation run %d: %w", run.ID, err)
		}
		run.TriggerID = nullableInt(triggerID)
		run.StartedAt = startedAt.Time
		run.FinishedAt = nullableTime(finishedAt)
		if message.Valid {
			run.Error = &message.String
		}
		run.StepOutcomes = []automation.RunStep{}
		positions[run.ID] = len(runs)
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	stepRows, err := r.db.Reader.QueryContext(ctx, `SELECT s.run_id, s.id, s.position, s.kind, s.outcome,
			s.command_id, s.started_at, s.finished_at
		FROM automation_run_steps s
		JOIN automation_runs r ON r.id = s.run_id
		WHERE r.automation_id = ?
		ORDER BY s.run_id, s.position`, automationID)
	if err != nil {
		return nil, fmt.Errorf("query automation run steps: %w", err)
	}
	defer stepRows.Close()
	for stepRows.Next() {
		var runID int
		var step automation.RunStep
		var commandID sql.NullInt64
		var startedAt SQLiteTime
		var finishedAt NullSQLiteTime
		if err := stepRows.Scan(&runID, &step.ID, &step.Position, &step.Kind, &step.Outcome,
			&commandID, &startedAt, &finishedAt); err != nil {
			return nil, fmt.Errorf("scan automation run step: %w", err)
		}
		step.CommandID = nullableInt(commandID)
		step.StartedAt = startedAt.Time
		step.FinishedAt = nullableTime(finishedAt)
		if index, ok := positions[runID]; ok {
			runs[index].StepOutcomes = append(runs[index].StepOutcomes, step)
		}
	}
	return runs, stepRows.Err()
}

func insertDefinition(ctx context.Context, tx *sql.Tx, automationID int, a automation.Automation) error {
	for i, trigger := range a.Triggers {
		var minuteOfDay, weekdays any
		if trigger.Schedule != nil {
			minuteOfDay, weekdays = trigger.Schedule.MinuteOfDay, int(trigger.Schedule.Days)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO automation_triggers
			(automation_id, position, kind, at_minute_of_day, weekdays) VALUES (?, ?, ?, ?, ?)`,
			automationID, i+1, trigger.Kind, minuteOfDay, weekdays); err != nil {
			return fmt.Errorf("insert automation trigger: %w", err)
		}
	}
	for i, step := range a.Steps {
		if _, err := tx.ExecContext(ctx, `INSERT INTO automation_steps
			(automation_id, position, kind, sensor_id, property, value) VALUES (?, ?, ?, ?, ?, ?)`,
			automationID, i+1, step.Kind, step.SensorID, step.Property, step.Value); err != nil {
			return fmt.Errorf("insert automation step: %w", err)
		}
	}
	return nil
}

func (r *AutomationRepository) queryAutomations(ctx context.Context, where string, args ...any) ([]automation.Automation, error) {
	rows, err := r.db.Reader.QueryContext(ctx,
		"SELECT id, name, enabled, created_at, updated_at FROM automations "+where+" ORDER BY name COLLATE NOCASE, id", args...)
	if err != nil {
		return nil, fmt.Errorf("query automations: %w", err)
	}
	defer rows.Close()

	automations := make([]automation.Automation, 0)
	index := make(map[int]int)
	for rows.Next() {
		var a automation.Automation
		var createdAt, updatedAt SQLiteTime
		if err := rows.Scan(&a.ID, &a.Name, &a.Enabled, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan automation: %w", err)
		}
		a.CreatedAt, a.UpdatedAt = createdAt.Time, updatedAt.Time
		index[a.ID] = len(automations)
		automations = append(automations, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(automations) == 0 {
		return automations, nil
	}

	ofMatching := "WHERE automation_id IN (SELECT id FROM automations " + where + ")"
	triggerRows, err := r.db.Reader.QueryContext(ctx, `SELECT automation_id, id, kind, at_minute_of_day, weekdays
		FROM automation_triggers `+ofMatching+` ORDER BY automation_id, position`, args...)
	if err != nil {
		return nil, fmt.Errorf("query automation triggers: %w", err)
	}
	defer triggerRows.Close()
	for triggerRows.Next() {
		var automationID int
		var trigger automation.Trigger
		var minuteOfDay, weekdays sql.NullInt64
		if err := triggerRows.Scan(&automationID, &trigger.ID, &trigger.Kind, &minuteOfDay, &weekdays); err != nil {
			return nil, fmt.Errorf("scan automation trigger: %w", err)
		}
		if trigger.Kind == automation.TriggerSchedule {
			trigger.Schedule = &automation.Schedule{MinuteOfDay: int(minuteOfDay.Int64), Days: automation.Weekdays(weekdays.Int64)}
		}
		if i, ok := index[automationID]; ok {
			automations[i].Triggers = append(automations[i].Triggers, trigger)
		}
	}
	if err := triggerRows.Err(); err != nil {
		return nil, err
	}

	stepRows, err := r.db.Reader.QueryContext(ctx, `SELECT automation_id, kind, sensor_id, property, value
		FROM automation_steps `+ofMatching+` ORDER BY automation_id, position`, args...)
	if err != nil {
		return nil, fmt.Errorf("query automation steps: %w", err)
	}
	defer stepRows.Close()
	for stepRows.Next() {
		var automationID int
		var step automation.Step
		var sensorID sql.NullInt64
		var property, value sql.NullString
		if err := stepRows.Scan(&automationID, &step.Kind, &sensorID, &property, &value); err != nil {
			return nil, fmt.Errorf("scan automation step: %w", err)
		}
		step.SensorID, step.Property, step.Value = int(sensorID.Int64), property.String, value.String
		if i, ok := index[automationID]; ok {
			automations[i].Steps = append(automations[i].Steps, step)
		}
	}
	return automations, stepRows.Err()
}

func (r *AutomationRepository) inTx(ctx context.Context, work func(tx *sql.Tx) (int, error)) (int, error) {
	tx, err := r.db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	id, err := work(tx)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}
	return id, nil
}

func requireRow(result sql.Result, err error, action string) error {
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	if affected == 0 {
		return automation.ErrNotFound
	}
	return nil
}

func nullableInt(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int64)
	return &v
}

func nullableTime(value NullSQLiteTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}
