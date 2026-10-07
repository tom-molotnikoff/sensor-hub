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
			"INSERT INTO automations (name, enabled, mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
			a.Name, a.Enabled, a.Mode, now, now)
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
			"UPDATE automations SET name = ?, enabled = ?, mode = ?, updated_at = ? WHERE id = ?",
			a.Name, a.Enabled, a.Mode, time.Now().UTC(), a.ID)
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
	_, err := r.inTx(ctx, func(tx *sql.Tx) (int, error) {
		active, err := activeRuns(ctx, tx, id)
		if err != nil {
			return 0, err
		}
		if len(active) > 0 {
			return 0, automation.ErrActiveRun
		}
		result, err := tx.ExecContext(ctx, "DELETE FROM automations WHERE id = ?", id)
		return 0, requireRow(result, err, "delete automation")
	})
	return err
}

func (r *AutomationRepository) SetBrokenReason(ctx context.Context, id int, reason string) (string, error) {
	var previous sql.NullString
	_, err := r.inTx(ctx, func(tx *sql.Tx) (int, error) {
		err := tx.QueryRowContext(ctx, "SELECT broken_reason FROM automations WHERE id = ?", id).Scan(&previous)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, automation.ErrNotFound
		}
		if err != nil {
			return 0, fmt.Errorf("query automation broken reason: %w", err)
		}
		if previous.String == reason {
			return 0, nil
		}
		stored := sql.NullString{String: reason, Valid: reason != ""}
		if _, err := tx.ExecContext(ctx, "UPDATE automations SET broken_reason = ? WHERE id = ?", stored, id); err != nil {
			return 0, fmt.Errorf("update automation broken reason: %w", err)
		}
		return 0, nil
	})
	return previous.String, err
}

func (r *AutomationRepository) RunStates(ctx context.Context) (map[int]automation.RunState, error) {
	rows, err := r.db.Reader.QueryContext(ctx, `SELECT a.id,
			EXISTS (SELECT 1 FROM automation_runs r WHERE r.automation_id = a.id AND r.status IN ('running', 'waiting')),
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
	return r.inTx(ctx, func(tx *sql.Tx) (int, error) { return insertRun(ctx, tx, run) })
}

func (r *AutomationRepository) AdmitRun(ctx context.Context, run automation.Run, mode automation.Mode) (automation.RunAdmission, error) {
	var admission automation.RunAdmission
	id, err := r.inTx(ctx, func(tx *sql.Tx) (int, error) {
		active, err := activeRuns(ctx, tx, run.AutomationID)
		if err != nil {
			return 0, err
		}
		run.Status = automation.RunRunning
		switch {
		case len(active) == 0:
		case mode == automation.ModeRestart:
			for _, id := range active {
				if err := cancelRun(ctx, tx, id, run.StartedAt); err != nil {
					return 0, err
				}
			}
			admission.Cancelled = active
		default:
			run.Status = automation.RunSkipped
			run.FinishedAt = &run.StartedAt
		}
		return insertRun(ctx, tx, run)
	})
	if err != nil {
		return automation.RunAdmission{}, err
	}
	admission.Run, err = r.getRun(ctx, id)
	return admission, err
}

func (r *AutomationRepository) CancelRun(ctx context.Context, automationID int, runID int, at time.Time) (automation.Run, error) {
	_, err := r.inTx(ctx, func(tx *sql.Tx) (int, error) {
		var status automation.RunStatus
		err := tx.QueryRowContext(ctx, "SELECT status FROM automation_runs WHERE id = ? AND automation_id = ?", runID, automationID).Scan(&status)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, automation.ErrRunNotFound
		}
		if err != nil {
			return 0, fmt.Errorf("query automation run status: %w", err)
		}
		if status != automation.RunRunning && status != automation.RunWaiting {
			return 0, automation.ErrRunNotActive
		}
		return 0, cancelRun(ctx, tx, runID, at)
	})
	if err != nil {
		return automation.Run{}, err
	}
	return r.getRun(ctx, runID)
}

func activeRuns(ctx context.Context, tx *sql.Tx, automationID int) ([]int, error) {
	rows, err := tx.QueryContext(ctx, "SELECT id FROM automation_runs WHERE automation_id = ? AND status IN (?, ?)",
		automationID, automation.RunRunning, automation.RunWaiting)
	if err != nil {
		return nil, fmt.Errorf("query active automation runs: %w", err)
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan active automation run: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func cancelRun(ctx context.Context, tx *sql.Tx, runID int, at time.Time) error {
	if _, err := tx.ExecContext(ctx, "UPDATE automation_runs SET status = ?, finished_at = ?, resume_at = NULL WHERE id = ?",
		automation.RunCancelled, at, runID); err != nil {
		return fmt.Errorf("cancel automation run: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE automation_run_steps SET outcome = ?, finished_at = ? WHERE run_id = ? AND outcome = ?",
		automation.StepCancelled, at, runID, automation.StepRunning); err != nil {
		return fmt.Errorf("cancel automation run step: %w", err)
	}
	return nil
}

func insertRun(ctx context.Context, tx *sql.Tx, run automation.Run) (int, error) {
	snapshot, err := json.Marshal(run.Steps)
	if err != nil {
		return 0, fmt.Errorf("encode run steps: %w", err)
	}
	var pastGrace *int64
	if run.PastGrace != nil {
		seconds := int64(run.PastGrace.Seconds())
		pastGrace = &seconds
	}
	var initiatedBy, causeRun *int
	if run.InitiatedBy != nil {
		initiatedBy = &run.InitiatedBy.ID
	}
	if run.CauseRun != nil {
		causeRun = &run.CauseRun.ID
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO automation_runs
		(automation_id, trigger_id, trigger_kind, initiated_by_user_id, cause_run_id, status, current_step, steps_snapshot, started_at, finished_at, due_at, past_grace_seconds, error)
		VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?, ?)`,
		run.AutomationID, run.TriggerID, run.TriggerKind, initiatedBy, causeRun, run.Status, string(snapshot), run.StartedAt, run.FinishedAt, run.DueAt, pastGrace, run.Error)
	if err != nil {
		return 0, fmt.Errorf("insert automation run: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read automation run id: %w", err)
	}
	return int(id), nil
}

func (r *AutomationRepository) CauseChain(ctx context.Context, runID int, limit int) ([]automation.CauseRun, error) {
	rows, err := r.db.Reader.QueryContext(ctx, `WITH RECURSIVE chain(id, automation_id, cause_run_id, depth) AS (
			SELECT id, automation_id, cause_run_id, 1 FROM automation_runs WHERE id = ?
			UNION ALL
			SELECT run.id, run.automation_id, run.cause_run_id, chain.depth + 1
			FROM automation_runs run JOIN chain ON run.id = chain.cause_run_id
			WHERE chain.depth < ?
		)
		SELECT chain.id, chain.automation_id, a.name FROM chain JOIN automations a ON a.id = chain.automation_id
		ORDER BY chain.depth`, runID, limit)
	if err != nil {
		return nil, fmt.Errorf("query cause chain of automation run %d: %w", runID, err)
	}
	defer rows.Close()
	var chain []automation.CauseRun
	for rows.Next() {
		var run automation.CauseRun
		if err := rows.Scan(&run.ID, &run.AutomationID, &run.AutomationName); err != nil {
			return nil, fmt.Errorf("scan cause chain of automation run %d: %w", runID, err)
		}
		chain = append(chain, run)
	}
	return chain, rows.Err()
}

func (r *AutomationRepository) StartRunStep(ctx context.Context, runID int, position int, kind automation.StepKind, at time.Time) (int, error) {
	return r.inTx(ctx, func(tx *sql.Tx) (int, error) {
		result, err := tx.ExecContext(ctx, "UPDATE automation_runs SET current_step = ? WHERE id = ? AND status = ?",
			position, runID, automation.RunRunning)
		if err := requireRun(result, err, "move automation run on"); err != nil {
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

// A step cancelled with its run while its command was in flight stays
// cancelled, but still records the command it sent.
func (r *AutomationRepository) FinishRunStep(ctx context.Context, stepID int, outcome automation.StepOutcome, commandID *int, at time.Time) error {
	_, err := r.db.Writer.ExecContext(ctx, `UPDATE automation_run_steps SET command_id = ?,
			outcome = CASE WHEN outcome = ? THEN ? ELSE outcome END,
			finished_at = CASE WHEN outcome = ? THEN ? ELSE finished_at END
		WHERE id = ?`,
		commandID, automation.StepRunning, outcome, automation.StepRunning, at, stepID)
	if err != nil {
		return fmt.Errorf("finish automation run step: %w", err)
	}
	return nil
}

func (r *AutomationRepository) SetTriggerDue(ctx context.Context, triggerID int, due time.Time) error {
	if _, err := r.db.Writer.ExecContext(ctx, "UPDATE automation_triggers SET next_due_at = ? WHERE id = ?", due, triggerID); err != nil {
		return fmt.Errorf("save automation trigger due time: %w", err)
	}
	return nil
}

func (r *AutomationRepository) SetMarginHint(ctx context.Context, triggerID int, hint *automation.MarginHint) error {
	var margin, checkedAt any
	if hint != nil {
		margin, checkedAt = hint.Margin, hint.CheckedAt
	}
	if _, err := r.db.Writer.ExecContext(ctx, "UPDATE automation_triggers SET margin_hint = ?, margin_hint_checked_at = ? WHERE id = ?",
		margin, checkedAt, triggerID); err != nil {
		return fmt.Errorf("save automation trigger margin hint: %w", err)
	}
	return nil
}

func (r *AutomationRepository) WaitRun(ctx context.Context, runID int, position int, at time.Time, resumeAt time.Time) error {
	_, err := r.inTx(ctx, func(tx *sql.Tx) (int, error) {
		result, err := tx.ExecContext(ctx, "UPDATE automation_runs SET status = ?, current_step = ?, resume_at = ? WHERE id = ? AND status = ?",
			automation.RunWaiting, position, resumeAt, runID, automation.RunRunning)
		if err := requireRun(result, err, "set automation run waiting"); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO automation_run_steps (run_id, position, kind, outcome, started_at)
			VALUES (?, ?, ?, ?, ?)`, runID, position, automation.StepWait, automation.StepRunning, at); err != nil {
			return 0, fmt.Errorf("insert automation wait step: %w", err)
		}
		return 0, nil
	})
	return err
}

func (r *AutomationRepository) ResumeRun(ctx context.Context, runID int, at time.Time) (automation.Run, error) {
	_, err := r.inTx(ctx, func(tx *sql.Tx) (int, error) {
		result, err := tx.ExecContext(ctx, "UPDATE automation_runs SET status = ?, resume_at = NULL WHERE id = ? AND status = ?",
			automation.RunRunning, runID, automation.RunWaiting)
		if err := requireRun(result, err, "resume automation run"); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE automation_run_steps SET outcome = ?, finished_at = ?
			WHERE run_id = ? AND position = (SELECT current_step FROM automation_runs WHERE id = ?)`,
			automation.StepSucceeded, at, runID, runID); err != nil {
			return 0, fmt.Errorf("finish automation wait step: %w", err)
		}
		return 0, nil
	})
	if err != nil {
		return automation.Run{}, err
	}
	return r.getRun(ctx, runID)
}

func (r *AutomationRepository) getRun(ctx context.Context, runID int) (automation.Run, error) {
	runs, err := r.queryRuns(ctx, "WHERE id = ?", runID)
	if err != nil {
		return automation.Run{}, err
	}
	if len(runs) == 0 {
		return automation.Run{}, automation.ErrRunGone
	}
	return runs[0], nil
}

func (r *AutomationRepository) ActiveRuns(ctx context.Context) ([]automation.Run, error) {
	return r.queryRuns(ctx, "WHERE status IN (?, ?)", automation.RunRunning, automation.RunWaiting)
}

func (r *AutomationRepository) LatestRunCommand(ctx context.Context, runID int, step automation.Step, since time.Time) (int, bool, error) {
	var id int
	err := r.db.Reader.QueryRowContext(ctx, `SELECT id FROM sensor_command_history
		WHERE automation_run_id = ? AND sensor_id = ? AND property = ? AND sent_at >= ?
		ORDER BY sent_at DESC, id DESC LIMIT 1`, runID, step.SensorID, step.Property, since).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("query command of automation run %d: %w", runID, err)
	}
	return id, true, nil
}

func (r *AutomationRepository) CancelledRunCommand(ctx context.Context, automationID int, step automation.Step) (int, bool, error) {
	var id int
	err := r.db.Reader.QueryRowContext(ctx, `SELECT command.id FROM sensor_command_history command
		JOIN automation_runs run ON run.id = command.automation_run_id
		WHERE run.automation_id = ? AND run.status = ? AND command.sensor_id = ? AND command.property = ? AND command.status = 'sent'
		ORDER BY command.sent_at DESC, command.id DESC LIMIT 1`,
		automationID, automation.RunCancelled, step.SensorID, step.Property).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("query in-flight command of a cancelled run of automation %d: %w", automationID, err)
	}
	return id, true, nil
}

func (r *AutomationRepository) FinishRun(ctx context.Context, runID int, status automation.RunStatus, message *string, at time.Time) error {
	result, err := r.db.Writer.ExecContext(ctx,
		"UPDATE automation_runs SET status = ?, error = ?, finished_at = ? WHERE id = ? AND status = ?",
		status, message, at, runID, automation.RunRunning)
	return requireRun(result, err, "finish automation run")
}

func (r *AutomationRepository) DeleteRunsFinishedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := r.db.Writer.ExecContext(ctx,
		"DELETE FROM automation_runs WHERE finished_at < ? AND status NOT IN (?, ?)",
		cutoff.UTC(), automation.RunRunning, automation.RunWaiting)
	if err != nil {
		return 0, fmt.Errorf("delete finished automation runs: %w", err)
	}
	return result.RowsAffected()
}

func (r *AutomationRepository) ListRuns(ctx context.Context, automationID int) ([]automation.Run, error) {
	return r.queryRuns(ctx, "WHERE automation_id = ?", automationID)
}

func (r *AutomationRepository) queryRuns(ctx context.Context, where string, args ...any) ([]automation.Run, error) {
	rows, err := r.db.Reader.QueryContext(ctx, `SELECT id, automation_id, trigger_id, trigger_kind,
			initiated_by_user_id, (SELECT username FROM users WHERE users.id = initiated_by_user_id),
			cause_run_id, (SELECT cause.automation_id FROM automation_runs cause WHERE cause.id = automation_runs.cause_run_id),
			(SELECT a.name FROM automation_runs cause JOIN automations a ON a.id = cause.automation_id WHERE cause.id = automation_runs.cause_run_id),
			status,
			current_step, steps_snapshot, started_at, finished_at, resume_at, due_at, past_grace_seconds, error
		FROM automation_runs `+where+`
		ORDER BY started_at DESC, id DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("query automation runs: %w", err)
	}
	defer rows.Close()

	runs := make([]automation.Run, 0)
	positions := make(map[int]int)
	for rows.Next() {
		var run automation.Run
		var triggerID, initiatedBy, causeRun, causeAutomation sql.NullInt64
		var initiatedByName, causeAutomationName sql.NullString
		var snapshot string
		var startedAt SQLiteTime
		var finishedAt, resumeAt, dueAt NullSQLiteTime
		var pastGrace sql.NullInt64
		var message sql.NullString
		if err := rows.Scan(&run.ID, &run.AutomationID, &triggerID, &run.TriggerKind, &initiatedBy, &initiatedByName,
			&causeRun, &causeAutomation, &causeAutomationName, &run.Status,
			&run.CurrentStep, &snapshot, &startedAt, &finishedAt, &resumeAt, &dueAt, &pastGrace, &message); err != nil {
			return nil, fmt.Errorf("scan automation run: %w", err)
		}
		if err := json.Unmarshal([]byte(snapshot), &run.Steps); err != nil {
			return nil, fmt.Errorf("decode steps of automation run %d: %w", run.ID, err)
		}
		run.TriggerID = nullableInt(triggerID)
		if initiatedBy.Valid {
			run.InitiatedBy = &automation.User{ID: int(initiatedBy.Int64), Username: initiatedByName.String}
		}
		if causeRun.Valid {
			run.CauseRun = &automation.CauseRun{ID: int(causeRun.Int64), AutomationID: int(causeAutomation.Int64), AutomationName: causeAutomationName.String}
		}
		run.StartedAt = startedAt.Time
		run.FinishedAt = nullableTime(finishedAt)
		run.ResumeAt = nullableTime(resumeAt)
		run.DueAt = nullableTime(dueAt)
		if pastGrace.Valid {
			duration := time.Duration(pastGrace.Int64) * time.Second
			run.PastGrace = &duration
		}
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
		WHERE s.run_id IN (SELECT id FROM automation_runs `+where+`)
		ORDER BY s.run_id, s.position`, args...)
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
		var minuteOfDay, weekdays, intervalSeconds any
		var sensorID, measurementTypeID, operator, threshold, binaryValue, margin, holdSeconds any
		switch trigger.Kind {
		case automation.TriggerSchedule:
			minuteOfDay, weekdays = trigger.Schedule.MinuteOfDay, int(trigger.Schedule.Days)
		case automation.TriggerInterval:
			intervalSeconds = int64(trigger.Interval / time.Second)
		case automation.TriggerReading:
			condition := trigger.Reading
			sensorID, measurementTypeID, operator = condition.SensorID, condition.MeasurementTypeID, condition.Operator
			holdSeconds = int64(condition.Hold / time.Second)
			if condition.Operator == automation.Becomes {
				binaryValue = condition.Value
			} else {
				threshold, margin = condition.Threshold, condition.Margin
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO automation_triggers
			(automation_id, position, kind, at_minute_of_day, weekdays, interval_seconds,
				sensor_id, measurement_type_id, operator, threshold, binary_value, rearm_margin, hold_seconds)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			automationID, i+1, trigger.Kind, minuteOfDay, weekdays, intervalSeconds,
			sensorID, measurementTypeID, operator, threshold, binaryValue, margin, holdSeconds); err != nil {
			return fmt.Errorf("insert automation trigger: %w", err)
		}
	}
	for i, step := range a.Steps {
		var sensorID, property, value, waitSeconds any
		switch step.Kind {
		case automation.StepSet:
			sensorID, property, value = step.SensorID, step.Property, step.Value
		case automation.StepWait:
			waitSeconds = step.Seconds
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO automation_steps
			(automation_id, position, kind, sensor_id, property, value, wait_seconds) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			automationID, i+1, step.Kind, sensorID, property, value, waitSeconds); err != nil {
			return fmt.Errorf("insert automation step: %w", err)
		}
	}
	return nil
}

func (r *AutomationRepository) queryAutomations(ctx context.Context, where string, args ...any) ([]automation.Automation, error) {
	rows, err := r.db.Reader.QueryContext(ctx,
		"SELECT id, name, enabled, mode, broken_reason, created_at, updated_at FROM automations "+where+" ORDER BY name COLLATE NOCASE, id", args...)
	if err != nil {
		return nil, fmt.Errorf("query automations: %w", err)
	}
	defer rows.Close()

	automations := make([]automation.Automation, 0)
	index := make(map[int]int)
	for rows.Next() {
		var a automation.Automation
		var brokenReason sql.NullString
		var createdAt, updatedAt SQLiteTime
		if err := rows.Scan(&a.ID, &a.Name, &a.Enabled, &a.Mode, &brokenReason, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan automation: %w", err)
		}
		a.BrokenReason = brokenReason.String
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
	triggerRows, err := r.db.Reader.QueryContext(ctx, `SELECT t.automation_id, t.id, t.kind, t.at_minute_of_day, t.weekdays,
			t.interval_seconds, t.next_due_at, t.sensor_id, t.measurement_type_id, mt.name, t.operator, t.threshold,
			t.binary_value, t.rearm_margin, t.hold_seconds, t.margin_hint, t.margin_hint_checked_at
		FROM automation_triggers t
		LEFT JOIN measurement_types mt ON mt.id = t.measurement_type_id
		`+ofMatching+` ORDER BY t.automation_id, t.position`, args...)
	if err != nil {
		return nil, fmt.Errorf("query automation triggers: %w", err)
	}
	defer triggerRows.Close()
	for triggerRows.Next() {
		var automationID int
		var trigger automation.Trigger
		var minuteOfDay, weekdays, intervalSeconds, sensorID, measurementTypeID, holdSeconds sql.NullInt64
		var measurementType, operator, binaryValue sql.NullString
		var threshold, margin, marginHint sql.NullFloat64
		var nextDueAt, marginHintCheckedAt NullSQLiteTime
		if err := triggerRows.Scan(&automationID, &trigger.ID, &trigger.Kind, &minuteOfDay, &weekdays, &intervalSeconds, &nextDueAt,
			&sensorID, &measurementTypeID, &measurementType, &operator, &threshold, &binaryValue, &margin, &holdSeconds,
			&marginHint, &marginHintCheckedAt); err != nil {
			return nil, fmt.Errorf("scan automation trigger: %w", err)
		}
		switch trigger.Kind {
		case automation.TriggerSchedule:
			trigger.Schedule = &automation.Schedule{MinuteOfDay: int(minuteOfDay.Int64), Days: automation.Weekdays(weekdays.Int64)}
		case automation.TriggerInterval:
			trigger.Interval = time.Duration(intervalSeconds.Int64) * time.Second
		case automation.TriggerReading:
			trigger.Reading = &automation.ReadingCondition{
				Series:            automation.Series{SensorID: int(sensorID.Int64), MeasurementType: measurementType.String},
				MeasurementTypeID: int(measurementTypeID.Int64),
				Operator:          automation.Operator(operator.String),
				Threshold:         threshold.Float64,
				Margin:            margin.Float64,
				Value:             binaryValue.String,
				Hold:              time.Duration(holdSeconds.Int64) * time.Second,
			}
			if marginHint.Valid {
				trigger.Reading.MarginHint = &automation.MarginHint{Margin: marginHint.Float64, CheckedAt: marginHintCheckedAt.Time}
			}
		}
		trigger.NextDueAt = nullableTime(nextDueAt)
		if i, ok := index[automationID]; ok {
			automations[i].Triggers = append(automations[i].Triggers, trigger)
		}
	}
	if err := triggerRows.Err(); err != nil {
		return nil, err
	}

	stepRows, err := r.db.Reader.QueryContext(ctx, `SELECT automation_id, kind, sensor_id, property, value, wait_seconds
		FROM automation_steps `+ofMatching+` ORDER BY automation_id, position`, args...)
	if err != nil {
		return nil, fmt.Errorf("query automation steps: %w", err)
	}
	defer stepRows.Close()
	for stepRows.Next() {
		var automationID int
		var step automation.Step
		var sensorID, waitSeconds sql.NullInt64
		var property, value sql.NullString
		if err := stepRows.Scan(&automationID, &step.Kind, &sensorID, &property, &value, &waitSeconds); err != nil {
			return nil, fmt.Errorf("scan automation step: %w", err)
		}
		step.SensorID, step.Property, step.Value = int(sensorID.Int64), property.String, value.String
		step.Seconds = int(waitSeconds.Int64)
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

func requireRun(result sql.Result, err error, action string) error {
	err = requireRow(result, err, action)
	if errors.Is(err, automation.ErrNotFound) {
		return automation.ErrRunGone
	}
	return err
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
