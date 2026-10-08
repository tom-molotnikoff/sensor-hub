package automation_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	appProps "example/sensorHub/application_properties"
	"example/sensorHub/automation"
	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/notifications"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixture struct {
	service  *automation.Service
	readings *automation.ReadingConsumer
	stop     context.CancelFunc
	store    *database.AutomationRepository
	history  database.ReadingsRepository
	db       *database.Handles
	sensors  *fakeSensors
	commands *fakeCommands
	notifier *fakeNotifier
	logger   *slog.Logger
	lampID   int
	climate  gen.Sensor
	door     gen.Sensor
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handles, err := database.Open(&appProps.ApplicationConfiguration{
		DatabasePath:              filepath.Join(t.TempDir(), "automations.db"),
		DatabaseReaderConnections: 2,
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { handles.Close() })

	result, err := handles.Writer.Exec("INSERT INTO sensors (name, sensor_driver, config) VALUES ('hallway-lamp', 'mqtt-zigbee2mqtt', '{}')")
	require.NoError(t, err)
	lampID, err := result.LastInsertId()
	require.NoError(t, err)

	climate := insertSensor(t, handles, "lounge-climate")
	door := insertSensor(t, handles, "front-door")

	stopCommands := make(chan struct{})
	t.Cleanup(func() { close(stopCommands) })
	f := &fixture{
		store: database.NewAutomationRepository(handles, logger),
		history: database.NewReadingsRepository(handles, database.NewSensorRepository(handles, logger),
			database.NewMeasurementTypeRepository(handles, logger), logger),
		db: handles,
		sensors: &fakeSensors{
			sensors: map[int]gen.Sensor{int(lampID): lamp(int(lampID)), climate.Id: climate, door.Id: door},
			types: map[int][]gen.MeasurementType{
				climate.Id: {measurementType(t, handles, "temperature"), measurementType(t, handles, "humidity")},
				door.Id:    {measurementType(t, handles, "contact")},
			},
		},
		commands: &fakeCommands{history: database.NewSensorCommandHistoryRepository(handles, logger),
			stop: stopCommands, awaited: make(map[int]chan string)},
		notifier: &fakeNotifier{},
		logger:   logger,
		lampID:   int(lampID),
		climate:  climate,
		door:     door,
	}
	f.start(t)
	return f
}

func (f *fixture) start(t *testing.T) {
	t.Helper()
	f.readings = automation.NewReadingConsumer()
	f.service = automation.NewService(f.store, f.sensors, f.commands, f.notifier, f.readings, f.history, f.logger)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	f.stop = cancel
	require.NoError(t, f.service.Start(ctx))
}

func (f *fixture) restart(t *testing.T) {
	t.Helper()
	f.stop()
	f.start(t)
}

func lamp(id int) gen.Sensor {
	minBrightness, maxBrightness := 0.0, 254.0
	on, off := "ON", "OFF"
	effects := []string{"blink", "breathe"}
	return gen.Sensor{
		Id:           id,
		Name:         "hallway-lamp",
		SensorDriver: "mqtt-zigbee2mqtt",
		Enabled:      true,
		Status:       gen.SensorStatusActive,
		Capabilities: &[]gen.Capability{
			{Property: "state", Type: gen.CapabilityTypeBinary, ValueOn: &on, ValueOff: &off},
			{Property: "brightness", Type: gen.CapabilityTypeNumeric, Min: &minBrightness, Max: &maxBrightness},
			{Property: "effect", Type: gen.CapabilityTypeEnum, Values: &effects},
		},
	}
}

// laterToday is a schedule time twelve hours away, so the real clock never
// fires it while a test runs.
func laterToday() string {
	return time.Now().UTC().Add(12 * time.Hour).Format("15:04")
}

func scheduleTrigger(at string, days ...gen.AutomationTriggerDays) gen.AutomationTrigger {
	if len(days) == 0 {
		days = []gen.AutomationTriggerDays{gen.AutomationDayMon, gen.AutomationDayTue, gen.AutomationDayWed,
			gen.AutomationDayThu, gen.AutomationDayFri, gen.AutomationDaySat, gen.AutomationDaySun}
	}
	return gen.AutomationTrigger{Type: gen.AutomationTriggerTypeSchedule, At: &at, Days: &days}
}

func intervalTrigger(seconds int) gen.AutomationTrigger {
	return gen.AutomationTrigger{Type: gen.AutomationTriggerTypeInterval, Seconds: &seconds}
}

func setStep(sensorID int, property, value string) gen.AutomationStep {
	return gen.AutomationStep{Type: gen.AutomationStepTypeSet, SensorId: &sensorID, Property: &property, Value: &value}
}

func waitStep(seconds int) gen.AutomationStep {
	return gen.AutomationStep{Type: gen.AutomationStepTypeWait, Seconds: &seconds}
}

func (f *fixture) create(t *testing.T, triggers []gen.AutomationTrigger, steps ...gen.AutomationStep) gen.Automation {
	t.Helper()
	created, err := f.service.Create(context.Background(), gen.AutomationInput{Name: "Evening lights", Triggers: triggers, Steps: steps})
	require.NoError(t, err)
	return created
}

func (f *fixture) fire(automation gen.Automation) {
	f.service.Fire(*automation.Triggers[0].Id, time.Now().UTC())
}

func (f *fixture) latestRun(t *testing.T, automationID int, status gen.AutomationRunStatus) gen.AutomationRun {
	t.Helper()
	var run gen.AutomationRun
	require.Eventually(t, func() bool {
		runs, err := f.service.Runs(context.Background(), automationID)
		require.NoError(t, err)
		if len(runs) == 0 || runs[0].Status != status {
			return false
		}
		run = runs[0]
		return true
	}, 5*time.Second, 10*time.Millisecond, "no run reached %s", status)
	return run
}

func (f *fixture) commandsInHistory(t *testing.T) int {
	t.Helper()
	var count int
	assert.NoError(t, f.db.Reader.QueryRow("SELECT COUNT(*) FROM sensor_command_history WHERE sensor_id = ?", f.lampID).Scan(&count))
	return count
}

// Not assert.Never, which checks on a goroutine that can still be reading the
// database after the test has closed it.
func (f *fixture) neverMoreCommandsThan(t *testing.T, limit int, message string) {
	t.Helper()
	for deadline := time.Now().Add(200 * time.Millisecond); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if f.commandsInHistory(t) > limit {
			assert.Fail(t, message)
			return
		}
	}
}

func insertSensor(t *testing.T, handles *database.Handles, name string) gen.Sensor {
	t.Helper()
	result, err := handles.Writer.Exec("INSERT INTO sensors (name, sensor_driver, config) VALUES (?, 'mqtt-zigbee2mqtt', '{}')", name)
	require.NoError(t, err)
	id, err := result.LastInsertId()
	require.NoError(t, err)
	return gen.Sensor{Id: int(id), Name: name, SensorDriver: "mqtt-zigbee2mqtt", Enabled: true, Status: gen.SensorStatusActive}
}

func measurementType(t *testing.T, handles *database.Handles, name string) gen.MeasurementType {
	t.Helper()
	mt := gen.MeasurementType{Name: name}
	require.NoError(t, handles.Reader.QueryRow("SELECT id, category, default_unit FROM measurement_types WHERE name = ?", name).
		Scan(&mt.Id, &mt.Category, &mt.Unit))
	return mt
}

type fakeSensors struct {
	mu      sync.Mutex
	sensors map[int]gen.Sensor
	types   map[int][]gen.MeasurementType
}

func (f *fakeSensors) ServiceGetMeasurementTypesForSensor(_ context.Context, id int) ([]gen.MeasurementType, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.types[id], nil
}

func (f *fakeSensors) ServiceGetSensorById(_ context.Context, id int) (*gen.Sensor, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	sensor, ok := f.sensors[id]
	if !ok {
		return nil, fmt.Errorf("no sensor found with id %d: %w", id, sql.ErrNoRows)
	}
	return &sensor, nil
}

func (f *fakeSensors) change(id int, edit func(sensor *gen.Sensor)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	sensor := f.sensors[id]
	edit(&sensor)
	f.sensors[id] = sensor
}

type sentCommand struct {
	id       int
	sensorID int
	property string
	value    string
	runID    int
	at       time.Time
	outcome  chan string
}

// fakeCommands writes real command history because a run step's
// command_id has to reference a real row, and records each outcome in it as
// the command tracker does.
type fakeCommands struct {
	history *database.SensorCommandHistoryRepository
	stop    chan struct{}
	mu      sync.Mutex
	sent    []sentCommand
	awaited map[int]chan string
	refuse  error
}

func (f *fakeCommands) SendAsSystem(ctx context.Context, sensorID int, property, value string, runID int) (int, <-chan string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.refuse != nil {
		return 0, nil, f.refuse
	}
	id, err := f.history.AddSentCommand(ctx, database.NewCommand{
		SensorID: sensorID, AutomationRunID: &runID, Property: property, Value: value,
		MQTTTopic: "zigbee2mqtt/hallway-lamp/set", MQTTPayload: "{}", TimeoutSeconds: 10, SentAt: time.Now().UTC(),
	})
	if err != nil {
		return 0, nil, err
	}
	command := sentCommand{id: id, sensorID: sensorID, property: property, value: value, runID: runID, at: time.Now(), outcome: make(chan string, 1)}
	f.sent = append(f.sent, command)
	return id, f.follow(id, command.outcome), nil
}

// A command sent before a restart gets a fresh channel, so that the stopped
// run cannot take its outcome.
func (f *fakeCommands) AwaitOutcome(_ context.Context, commandID int) (<-chan string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	outcome := make(chan string, 1)
	f.awaited[commandID] = outcome
	return f.follow(commandID, outcome), nil
}

// follow passes on the outcome a test gives a command once it is in command
// history, so that the command is no longer in flight there either.
func (f *fakeCommands) follow(commandID int, given <-chan string) <-chan string {
	outcome := make(chan string, 1)
	go func() {
		select {
		case status := <-given:
			ctx := context.Background()
			switch status {
			case "acknowledged":
				_, _ = f.history.MarkAcknowledged(ctx, commandID, "", time.Now().UTC())
			case "timed_out":
				_, _ = f.history.MarkTimedOut(ctx, commandID)
			default:
				_, _ = f.history.MarkFailed(ctx, commandID)
			}
			outcome <- status
		case <-f.stop:
		}
	}()
	return outcome
}

func (f *fakeCommands) awaitedOutcome(t *testing.T, commandID int) chan string {
	t.Helper()
	var outcome chan string
	require.Eventually(t, func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		outcome = f.awaited[commandID]
		return outcome != nil
	}, 5*time.Second, 10*time.Millisecond, "command %d was never awaited", commandID)
	return outcome
}

func (f *fakeCommands) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

func (f *fakeCommands) await(t *testing.T, index int) sentCommand {
	t.Helper()
	require.Eventually(t, func() bool { return f.count() > index }, 5*time.Second, 10*time.Millisecond, "command %d was never sent", index)
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sent[index]
}

type sentNotification struct {
	notification notifications.Notification
	permission   string
}

type fakeNotifier struct {
	mu   sync.Mutex
	sent []sentNotification
}

func (f *fakeNotifier) CreateNotification(_ context.Context, notification notifications.Notification, permission string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sentNotification{notification: notification, permission: permission})
	return len(f.sent), nil
}

func (f *fakeNotifier) all() []sentNotification {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentNotification(nil), f.sent...)
}

func TestRun_SendsEachSetStepOnlyAfterThePreviousIsAcknowledged(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())},
		setStep(f.lampID, "state", "ON"),
		setStep(f.lampID, "brightness", "150"),
	)

	f.fire(created)

	first := f.commands.await(t, 0)
	assert.Equal(t, "state", first.property)
	assert.Equal(t, "ON", first.value)
	f.neverMoreCommandsThan(t, 1, "the second step went out before the first was acknowledged")

	first.outcome <- "acknowledged"
	second := f.commands.await(t, 1)
	assert.Equal(t, "brightness", second.property)
	assert.Equal(t, "150", second.value)
	second.outcome <- "acknowledged"

	run := f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)
	assert.Equal(t, first.runID, run.Id)
	assert.Equal(t, gen.AutomationRunTriggerKindSchedule, run.TriggerKind)
	assert.Equal(t, created.Triggers[0].Id, run.TriggerId)
	assert.Equal(t, 2, run.CurrentStep)
	assert.Len(t, run.Steps, 2)
	require.Len(t, run.StepOutcomes, 2)
	for i, outcome := range run.StepOutcomes {
		assert.Equal(t, i+1, outcome.Position)
		assert.Equal(t, gen.AutomationRunStepOutcomeSucceeded, outcome.Outcome)
		require.NotNil(t, outcome.FinishedAt)
	}
	assert.Equal(t, &first.id, run.StepOutcomes[0].CommandId)
	assert.Equal(t, &second.id, run.StepOutcomes[1].CommandId)
	assert.NotNil(t, run.FinishedAt)
	assert.Empty(t, f.notifier.all())

	history, err := f.commands.history.ListBySensorID(context.Background(), f.lampID, 10)
	require.NoError(t, err)
	require.Len(t, history, 2)
	for _, entry := range history {
		assert.Nil(t, entry.User, "an automation's command has no user")
		assert.Equal(t, &run.Id, entry.AutomationRunId)
		assert.Equal(t, &gen.CommandHistoryAutomation{Id: created.Id, Name: "Evening lights"}, entry.Automation)
	}
}

func TestRun_AFailedStepEndsTheRunAndNotifiesAutomationManagers(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())},
		setStep(f.lampID, "state", "ON"),
		setStep(f.lampID, "state", "OFF"),
	)

	f.fire(created)
	f.commands.await(t, 0).outcome <- "timed_out"

	run := f.latestRun(t, created.Id, gen.AutomationRunStatusFailed)
	require.NotNil(t, run.Error)
	assert.Contains(t, *run.Error, "step 1 (set hallway-lamp state to ON)")
	assert.Contains(t, *run.Error, "did not acknowledge")
	require.Len(t, run.StepOutcomes, 1, "no step after the failed one runs")
	assert.Equal(t, gen.AutomationRunStepOutcomeFailed, run.StepOutcomes[0].Outcome)
	assert.Equal(t, 1, f.commandsInHistory(t))

	sent := f.notifier.all()
	require.Len(t, sent, 1)
	assert.Equal(t, "manage_automations", sent[0].permission)
	assert.Equal(t, notifications.CategoryAutomationFailure, sent[0].notification.Category)
	assert.Equal(t, notifications.SeverityError, sent[0].notification.Severity)
	assert.Contains(t, sent[0].notification.Title, "Evening lights")
	assert.Contains(t, sent[0].notification.Message, "step 1 (set hallway-lamp state to ON)")
	assert.Contains(t, sent[0].notification.Message, "did not acknowledge")

	view, err := f.service.Get(context.Background(), created.Id)
	require.NoError(t, err)
	assert.True(t, view.LastRunFailed)
}

func TestRun_AStepThatCannotBeSentFailsWithoutRetrying(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(f *fixture)
		reason  string
		sends   int
	}{
		{
			name: "a command already in flight for the property",
			arrange: func(f *fixture) {
				f.commands.refuse = errors.New(`sensor 1 already has a pending command for property "state"`)
			},
			reason: "already has a pending command",
		},
		{
			name:    "a disabled sensor",
			arrange: func(f *fixture) { f.sensors.change(f.lampID, func(s *gen.Sensor) { s.Enabled = false }) },
			reason:  "hallway-lamp is disabled",
		},
		{
			name: "a sensor that is not active",
			arrange: func(f *fixture) {
				f.sensors.change(f.lampID, func(s *gen.Sensor) { s.Status = gen.SensorStatusPending })
			},
			reason: "hallway-lamp is pending, not active",
		},
		{
			name: "a value the capability no longer allows",
			arrange: func(f *fixture) {
				f.sensors.change(f.lampID, func(s *gen.Sensor) {
					(*s.Capabilities)[0].ValueOn = ptr("open")
					(*s.Capabilities)[0].ValueOff = ptr("closed")
				})
			},
			reason: `"ON" is not "open" or "closed"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, setStep(f.lampID, "state", "ON"))
			tt.arrange(f)

			f.fire(created)

			run := f.latestRun(t, created.Id, gen.AutomationRunStatusFailed)
			require.NotNil(t, run.Error)
			assert.Contains(t, *run.Error, tt.reason)
			assert.Equal(t, 0, f.commandsInHistory(t))
			assert.Len(t, f.notifier.all(), 1)
		})
	}
}

func ptr[T any](value T) *T { return &value }

func TestSave_AcceptsAnyWritableCapability(t *testing.T) {
	f := newFixture(t)

	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger("19:00", gen.AutomationDayMon, gen.AutomationDayFri)},
		setStep(f.lampID, "state", "on"),
		setStep(f.lampID, "brightness", "254"),
		setStep(f.lampID, "effect", "breathe"),
	)

	assert.Len(t, created.Steps, 3)
	assert.Equal(t, "19:00", *created.Triggers[0].At)
	assert.Equal(t, []gen.AutomationTriggerDays{gen.AutomationDayMon, gen.AutomationDayFri}, *created.Triggers[0].Days)
	assert.True(t, created.Enabled, "an automation is on unless the body says otherwise")
}

func TestSave_RejectsAnInvalidAutomationNamingTheField(t *testing.T) {
	tests := []struct {
		name     string
		triggers func(lampID int) []gen.AutomationTrigger
		steps    func(lampID int) []gen.AutomationStep
		field    string
	}{
		{"no triggers", nil, nil, "triggers"},
		{"no steps", nil, func(int) []gen.AutomationStep { return nil }, "steps"},
		{"a time past 23:59", func(int) []gen.AutomationTrigger { return []gen.AutomationTrigger{scheduleTrigger("24:00")} }, nil, "triggers[0].at"},
		{"a time not as HH:MM", func(int) []gen.AutomationTrigger { return []gen.AutomationTrigger{scheduleTrigger("7pm")} }, nil, "triggers[0].at"},
		{"no weekdays", func(int) []gen.AutomationTrigger {
			trigger := scheduleTrigger("19:00")
			trigger.Days = &[]gen.AutomationTriggerDays{}
			return []gen.AutomationTrigger{trigger}
		}, nil, "triggers[0].days"},
		{"an interval under a minute", func(int) []gen.AutomationTrigger { return []gen.AutomationTrigger{intervalTrigger(59)} }, nil, "triggers[0].seconds"},
		{"an interval with no length", func(int) []gen.AutomationTrigger {
			return []gen.AutomationTrigger{{Type: gen.AutomationTriggerTypeInterval}}
		}, nil, "triggers[0].seconds"},
		{"a cron trigger", func(int) []gen.AutomationTrigger {
			return []gen.AutomationTrigger{{Type: "cron", At: ptr("0 19 * * 1-5")}}
		}, nil, "triggers[0].type"},
		{"a sensor that does not exist", nil, func(int) []gen.AutomationStep { return []gen.AutomationStep{setStep(999, "state", "ON")} }, "steps[0].sensor_id"},
		{"a property that is not writable", nil, func(id int) []gen.AutomationStep { return []gen.AutomationStep{setStep(id, "colour", "red")} }, "steps[0].property"},
		{"a binary value that is neither on nor off", nil, func(id int) []gen.AutomationStep { return []gen.AutomationStep{setStep(id, "state", "MAYBE")} }, "steps[0].value"},
		{"a numeric value above the maximum", nil, func(id int) []gen.AutomationStep { return []gen.AutomationStep{setStep(id, "brightness", "300")} }, "steps[0].value"},
		{"an enum value outside the list", nil, func(id int) []gen.AutomationStep { return []gen.AutomationStep{setStep(id, "effect", "spin")} }, "steps[0].value"},
		{"a wait under a second", nil, func(int) []gen.AutomationStep { return []gen.AutomationStep{waitStep(0)} }, "steps[0].seconds"},
		{"a wait with no duration", nil, func(int) []gen.AutomationStep { return []gen.AutomationStep{{Type: gen.AutomationStepTypeWait}} }, "steps[0].seconds"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			input := gen.AutomationInput{
				Name:  "Evening lights",
				Steps: []gen.AutomationStep{setStep(f.lampID, "state", "ON")},
			}
			if tt.field != "triggers" {
				input.Triggers = []gen.AutomationTrigger{scheduleTrigger("19:00")}
			}
			if tt.triggers != nil {
				input.Triggers = tt.triggers(f.lampID)
			}
			if tt.steps != nil {
				input.Steps = tt.steps(f.lampID)
			}

			_, err := f.service.Create(context.Background(), input)

			var invalid *automation.ValidationError
			require.ErrorAs(t, err, &invalid)
			assert.Contains(t, invalid.Message, tt.field)
		})
	}
}

func TestStatus_FollowsTheEnabledSwitchAndTheRuns(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, setStep(f.lampID, "state", "ON"))
	assert.Equal(t, gen.AutomationStatusArmed, created.Status)
	assert.Equal(t, "UTC", created.HubTimezone)
	require.NotNil(t, created.NextFireAt)
	assert.WithinDuration(t, time.Now().Add(12*time.Hour), *created.NextFireAt, time.Minute)

	f.fire(created)
	first := f.commands.await(t, 0)
	running, err := f.service.Get(ctx, created.Id)
	require.NoError(t, err)
	assert.Equal(t, gen.AutomationStatusRunning, running.Status)

	first.outcome <- "failed"
	f.latestRun(t, created.Id, gen.AutomationRunStatusFailed)
	failed, err := f.service.Get(ctx, created.Id)
	require.NoError(t, err)
	assert.Equal(t, gen.AutomationStatusArmed, failed.Status)
	assert.True(t, failed.LastRunFailed)

	f.service.Fire(*created.Triggers[0].Id, time.Now().UTC().Add(time.Second))
	f.commands.await(t, 1).outcome <- "acknowledged"
	f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)
	recovered, err := f.service.Get(ctx, created.Id)
	require.NoError(t, err)
	assert.False(t, recovered.LastRunFailed, "a succeeded run clears the flag")

	off, err := f.service.SetEnabled(ctx, created.Id, false)
	require.NoError(t, err)
	assert.Equal(t, gen.AutomationStatusOff, off.Status)
	assert.Nil(t, off.NextFireAt)

	f.fire(created)
	f.neverMoreCommandsThan(t, 2, "an automation that is off started a run")
}

func TestTriggers_EitherStartsARunAndTwoDueTogetherStartOne(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday()), scheduleTrigger(laterToday())},
		setStep(f.lampID, "state", "ON"))
	due := time.Now().UTC()

	f.service.Fire(*created.Triggers[0].Id, due)
	f.service.Fire(*created.Triggers[1].Id, due)
	f.commands.await(t, 0).outcome <- "acknowledged"
	f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)

	f.service.Fire(*created.Triggers[1].Id, due.Add(time.Minute))
	f.commands.await(t, 1).outcome <- "acknowledged"
	require.Eventually(t, func() bool {
		runs, err := f.service.Runs(context.Background(), created.Id)
		require.NoError(t, err)
		return len(runs) == 2 && runs[0].Status == gen.AutomationRunStatusSucceeded
	}, 5*time.Second, 10*time.Millisecond)
}

func TestHubTimezoneChange_RecomputesNextFireTimesStraightAway(t *testing.T) {
	f := newFixture(t)
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger("19:00")}, setStep(f.lampID, "state", "ON"))
	require.NotNil(t, created.NextFireAt)
	assert.Equal(t, 19, created.NextFireAt.Hour(), "19:00 in UTC")

	original := appProps.AppConfig()
	t.Cleanup(func() { appProps.SetAppConfig(original) })
	application, db := appProps.BuildDefaults()
	application["hub.timezone"] = "Asia/Tokyo"
	require.NoError(t, appProps.ReloadConfig(application, db))

	moved, err := f.service.Get(context.Background(), created.Id)
	require.NoError(t, err)
	assert.Equal(t, "Asia/Tokyo", moved.HubTimezone)
	require.NotNil(t, moved.NextFireAt)
	assert.Equal(t, 10, moved.NextFireAt.Hour(), "19:00 in Tokyo is 10:00 UTC")
}

func TestDelete_RemovesRunsAndKeepsTheirCommandsInHistory(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	created := f.create(t, []gen.AutomationTrigger{scheduleTrigger(laterToday())}, setStep(f.lampID, "state", "ON"))
	f.fire(created)
	f.commands.await(t, 0).outcome <- "acknowledged"
	f.latestRun(t, created.Id, gen.AutomationRunStatusSucceeded)

	require.NoError(t, f.service.Delete(ctx, created.Id))

	_, err := f.service.Runs(ctx, created.Id)
	assert.ErrorIs(t, err, automation.ErrNotFound)
	var runs int
	require.NoError(t, f.db.Reader.QueryRow("SELECT COUNT(*) FROM automation_runs").Scan(&runs))
	assert.Zero(t, runs)
	history, err := f.commands.history.ListBySensorID(ctx, f.lampID, 10)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Nil(t, history[0].AutomationRunId)
	assert.Nil(t, history[0].Automation)
}
