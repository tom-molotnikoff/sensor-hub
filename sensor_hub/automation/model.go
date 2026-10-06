package automation

import (
	"errors"
	"time"
)

type TriggerKind string

const (
	TriggerSchedule TriggerKind = "schedule"
	TriggerInterval TriggerKind = "interval"
	TriggerManual   TriggerKind = "manual"
)

// Mode decides what a trigger does while the automation already has an
// active run.
type Mode string

const (
	ModeSingle  Mode = "single"
	ModeRestart Mode = "restart"
)

type StepKind string

const (
	StepSet  StepKind = "set"
	StepWait StepKind = "wait"
)

type RunStatus string

const (
	RunRunning   RunStatus = "running"
	RunWaiting   RunStatus = "waiting"
	RunSucceeded RunStatus = "succeeded"
	RunFailed    RunStatus = "failed"
	RunCancelled RunStatus = "cancelled"
	RunMissed    RunStatus = "missed"
	RunSkipped   RunStatus = "skipped"
)

type StepOutcome string

const (
	StepRunning   StepOutcome = "running"
	StepSucceeded StepOutcome = "succeeded"
	StepFailed    StepOutcome = "failed"
	StepCancelled StepOutcome = "cancelled"
)

// ErrRunGone means the run was cancelled, or deleted with its automation,
// while it was going.
var ErrRunGone = errors.New("automation run is no longer active")

var (
	ErrRunNotFound  = errors.New("automation run not found")
	ErrRunNotActive = errors.New("automation run is not active")
	ErrActiveRun    = errors.New("automation has an active run")
)

type Automation struct {
	ID        int
	Name      string
	Enabled   bool
	Mode      Mode
	Triggers  []Trigger
	Steps     []Step
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Trigger struct {
	ID        int
	Kind      TriggerKind
	Schedule  *Schedule
	Interval  time.Duration
	NextDueAt *time.Time
}

// A Step's JSON form is the API body's step shape, which is also how a run
// keeps a copy of the steps it started with.
type Step struct {
	Kind     StepKind `json:"type"`
	SensorID int      `json:"sensor_id,omitempty"`
	Property string   `json:"property,omitempty"`
	Value    string   `json:"value,omitempty"`
	Seconds  int      `json:"seconds,omitempty"`
}

func (s Step) Wait() time.Duration {
	return time.Duration(s.Seconds) * time.Second
}

type Run struct {
	ID           int
	AutomationID int
	TriggerID    *int
	TriggerKind  TriggerKind
	InitiatedBy  *User
	Status       RunStatus
	CurrentStep  int
	Steps        []Step
	StartedAt    time.Time
	FinishedAt   *time.Time
	ResumeAt     *time.Time
	DueAt        *time.Time
	PastGrace    *time.Duration
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

type User struct {
	ID       int
	Username string
}

// RunAdmission is what applying an automation's mode did when a run was
// asked for: Run is the started run, or the skipped one.
type RunAdmission struct {
	Run       Run
	Cancelled []int
}

type RunState struct {
	Running       bool
	LastRunFailed bool
}
