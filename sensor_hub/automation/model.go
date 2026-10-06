// Package automation runs user-defined automations: triggers that start a
// run, and an ordered list of steps the hub carries out as the system actor.
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

// ErrRunGone reports that a run's row no longer exists, because its
// automation was deleted while the run was going.
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

// Trigger starts a run. Schedule is set for schedule triggers.
type Trigger struct {
	ID       int
	Kind     TriggerKind
	Schedule *Schedule
}

// Step is one action of a run. Its JSON form is the API body's step shape,
// which is also how a run keeps a copy of the steps it started with.
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

// RunStep is the outcome of one step of a run. Position counts from 1.
type RunStep struct {
	ID         int
	Position   int
	Kind       StepKind
	Outcome    StepOutcome
	CommandID  *int
	StartedAt  time.Time
	FinishedAt *time.Time
}

// RunState is what an automation's runs say about its status.
type RunState struct {
	Running       bool
	LastRunFailed bool
}
