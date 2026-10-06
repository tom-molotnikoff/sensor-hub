package automation

import (
	"errors"
	"time"
)

type TriggerKind string

const TriggerSchedule TriggerKind = "schedule"

type StepKind string

const StepSet StepKind = "set"

type RunStatus string

const (
	RunRunning   RunStatus = "running"
	RunSucceeded RunStatus = "succeeded"
	RunFailed    RunStatus = "failed"
)

type StepOutcome string

const (
	StepRunning   StepOutcome = "running"
	StepSucceeded StepOutcome = "succeeded"
	StepFailed    StepOutcome = "failed"
)

// ErrRunGone means the run was deleted, with its automation, while it was going.
var ErrRunGone = errors.New("automation run no longer exists")

type Automation struct {
	ID        int
	Name      string
	Enabled   bool
	Triggers  []Trigger
	Steps     []Step
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Trigger struct {
	ID       int
	Kind     TriggerKind
	Schedule *Schedule
}

// A Step's JSON form is the API body's step shape, which is also how a run
// keeps a copy of the steps it started with.
type Step struct {
	Kind     StepKind `json:"type"`
	SensorID int      `json:"sensor_id,omitempty"`
	Property string   `json:"property,omitempty"`
	Value    string   `json:"value,omitempty"`
}

type Run struct {
	ID           int
	AutomationID int
	TriggerID    *int
	TriggerKind  TriggerKind
	Status       RunStatus
	CurrentStep  int
	Steps        []Step
	StartedAt    time.Time
	FinishedAt   *time.Time
	Error        *string
	StepOutcomes []RunStep
}

// RunStep positions count from 1.
type RunStep struct {
	ID         int
	Position   int
	Kind       StepKind
	Outcome    StepOutcome
	CommandID  *int
	StartedAt  time.Time
	FinishedAt *time.Time
}

type RunState struct {
	Running       bool
	LastRunFailed bool
}
