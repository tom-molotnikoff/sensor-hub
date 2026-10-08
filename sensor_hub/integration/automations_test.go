//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/testharness"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func everyDay() *[]gen.AutomationTriggerDays {
	return &[]gen.AutomationTriggerDays{gen.AutomationDayMon, gen.AutomationDayTue, gen.AutomationDayWed,
		gen.AutomationDayThu, gen.AutomationDayFri, gen.AutomationDaySat, gen.AutomationDaySun}
}

// soonDueMinute puts the hub's clock a few seconds short of a whole minute and
// returns that minute, so a schedule set for it comes due in seconds rather
// than at the next real minute. The hub goes back to the system clock when
// the test ends.
func soonDueMinute(t *testing.T) time.Time {
	t.Helper()
	due := time.Now().UTC().Truncate(time.Minute).Add(time.Minute)
	env.Clock.Set(due.Add(-3 * time.Second))
	t.Cleanup(env.Clock.Reset)
	return due
}

func TestHubTimezone_RejectsAZoneThatDoesNotLoad(t *testing.T) {
	body, status := client.UpdateProperties(gen.UpdatePropertiesJSONRequestBody{"hub.timezone": "Mars/Olympus_Mons"})
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Contains(t, string(body), "hub.timezone")
}

func TestAutomation_RejectsAScheduleWithNoWeekdays(t *testing.T) {
	fixture := setupCommandFixture(t, fmt.Sprintf("no-days-plug-%d", reserveTCPPort(t)))
	defer fixture.stop()

	at := "19:00"
	body, status := client.CreateAutomation(gen.AutomationInput{
		Name:     "No days",
		Triggers: []gen.AutomationTrigger{{Type: gen.AutomationTriggerTypeSchedule, At: &at, Days: &[]gen.AutomationTriggerDays{}}},
		Steps:    []gen.AutomationStep{{Type: gen.AutomationStepTypeSet, SensorId: &fixture.sensor.Id, Property: ptrStr("state"), Value: ptrStr("ON")}},
	})

	assert.Equal(t, http.StatusBadRequest, status)
	assert.Contains(t, string(body), "triggers[0].days")
}

func TestAutomation_RejectsAnIntervalUnderAMinute(t *testing.T) {
	fixture := setupCommandFixture(t, fmt.Sprintf("short-interval-plug-%d", reserveTCPPort(t)))
	defer fixture.stop()

	seconds := 59
	body, status := client.CreateAutomation(gen.AutomationInput{
		Name:     "Too often",
		Triggers: []gen.AutomationTrigger{{Type: gen.AutomationTriggerTypeInterval, Seconds: &seconds}},
		Steps:    []gen.AutomationStep{{Type: gen.AutomationStepTypeSet, SensorId: &fixture.sensor.Id, Property: ptrStr("state"), Value: ptrStr("ON")}},
	})

	assert.Equal(t, http.StatusBadRequest, status)
	assert.Contains(t, string(body), "triggers[0].seconds")
}

func TestAutomation_ASchedulePublishesTheCommandAsTheSystem(t *testing.T) {
	fixture := setupCommandFixture(t, fmt.Sprintf("timer-plug-%d", reserveTCPPort(t)))
	defer fixture.stop()
	require.Equal(t, http.StatusAccepted, client.SetProperty("hub.timezone", "UTC"))

	subscriber := pahomqtt.NewClient(pahomqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://127.0.0.1:%d", fixture.port)).
		SetClientID(fmt.Sprintf("integration-automation-%d", fixture.port)))
	token := subscriber.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	defer subscriber.Disconnect(250)
	published := make(chan time.Time, 1)
	token = subscriber.Subscribe(fmt.Sprintf("zigbee2mqtt/%s/set", fixture.sensor.Name), 1, func(_ pahomqtt.Client, msg pahomqtt.Message) {
		published <- time.Now()
	})
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())

	due := soonDueMinute(t)
	at := due.Format("15:04")
	body, status := client.CreateAutomation(gen.AutomationInput{
		Name:     "Integration lights",
		Triggers: []gen.AutomationTrigger{{Type: gen.AutomationTriggerTypeSchedule, At: &at, Days: everyDay()}},
		Steps:    []gen.AutomationStep{{Type: gen.AutomationStepTypeSet, SensorId: &fixture.sensor.Id, Property: ptrStr("state"), Value: ptrStr("ON")}},
	})
	require.Equal(t, http.StatusCreated, status, string(body))
	var created gen.Automation
	require.NoError(t, json.Unmarshal(body, &created))
	assert.Equal(t, gen.AutomationStatusArmed, created.Status)
	assert.Equal(t, "UTC", created.HubTimezone)
	require.NotNil(t, created.NextFireAt)
	assert.True(t, due.Equal(*created.NextFireAt), "next fire %s, want %s", created.NextFireAt, due)

	select {
	case <-published:
	case <-time.After(due.Sub(env.Clock.Now()) + 5*time.Second):
		t.Fatal("the automation never published its command")
	}
	pub := subscriber.Publish(fmt.Sprintf("zigbee2mqtt/%s", fixture.sensor.Name), 1, false, `{"state":"ON"}`)
	require.True(t, pub.WaitTimeout(5*time.Second))
	require.NoError(t, pub.Error())

	var run gen.AutomationRun
	require.Eventually(t, func() bool {
		runs, status := client.ListAutomationRuns(created.Id)
		require.Equal(t, http.StatusOK, status)
		if len(runs) != 1 || runs[0].Status != gen.AutomationRunStatusSucceeded {
			return false
		}
		run = runs[0]
		return true
	}, 10*time.Second, 100*time.Millisecond)
	lateness := run.StartedAt.Sub(due)
	assert.GreaterOrEqual(t, lateness, time.Duration(0))
	assert.Less(t, lateness, time.Second, "the run started %s after its due time", lateness)

	history, status := client.GetSensorCommandHistory(fixture.sensor.Id)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, history, 1)
	assert.Nil(t, history[0].User)
	assert.Equal(t, &run.Id, history[0].AutomationRunId)
	assert.Equal(t, &gen.CommandHistoryAutomation{Id: created.Id, Name: "Integration lights"}, history[0].Automation)

	viewerName := fmt.Sprintf("automation-viewer-%d", fixture.port)
	_, status = client.CreateUser(gen.CreateUserRequest{Username: viewerName, Password: "viewerpass123", Roles: &[]string{"viewer"}})
	require.Equal(t, http.StatusCreated, status)
	viewer := testharness.NewClient(t, env.ServerURL)
	require.Equal(t, http.StatusOK, viewer.Login(viewerName, "viewerpass123"))
	require.Equal(t, http.StatusOK, viewer.ChangePassword("viewerpass123"))
	listed, status := viewer.ListAutomations()
	assert.Equal(t, http.StatusOK, status)
	assert.NotEmpty(t, listed)
	_, status = viewer.CreateAutomation(gen.AutomationInput{Name: "Not allowed"})
	assert.Equal(t, http.StatusForbidden, status)

	require.Equal(t, http.StatusOK, client.DeleteAutomation(created.Id))
	history, status = client.GetSensorCommandHistory(fixture.sensor.Id)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, history, 1)
	assert.Nil(t, history[0].AutomationRunId)
	assert.Nil(t, history[0].Automation)
}

type publishedCommand struct {
	at      time.Time
	payload string
}

// The run's wait of a few seconds outlasts the restart, which takes well under
// one, so the hub comes back to a run that is still waiting.
func TestAutomation_ARunWaitingAcrossARestartSendsItsRemainingSteps(t *testing.T) {
	fixture := setupCommandFixture(t, fmt.Sprintf("wait-plug-%d", reserveTCPPort(t)))
	defer fixture.stop()
	require.Equal(t, http.StatusAccepted, client.SetProperty("hub.timezone", "UTC"))

	device := pahomqtt.NewClient(pahomqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://127.0.0.1:%d", fixture.port)).
		SetClientID(fmt.Sprintf("integration-wait-%d", fixture.port)))
	token := device.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	defer device.Disconnect(250)
	published := make(chan publishedCommand, 2)
	token = device.Subscribe(fmt.Sprintf("zigbee2mqtt/%s/set", fixture.sensor.Name), 1, func(_ pahomqtt.Client, msg pahomqtt.Message) {
		published <- publishedCommand{at: env.Clock.Now(), payload: string(msg.Payload())}
	})
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	acknowledge := func(command publishedCommand) {
		pub := device.Publish(fmt.Sprintf("zigbee2mqtt/%s", fixture.sensor.Name), 1, false, command.payload)
		require.True(t, pub.WaitTimeout(5*time.Second))
		require.NoError(t, pub.Error())
	}

	due := soonDueMinute(t)
	at := due.Format("15:04")
	seconds := 5
	body, status := client.CreateAutomation(gen.AutomationInput{
		Name:     "Integration lamp timer",
		Triggers: []gen.AutomationTrigger{{Type: gen.AutomationTriggerTypeSchedule, At: &at, Days: everyDay()}},
		Steps: []gen.AutomationStep{
			{Type: gen.AutomationStepTypeSet, SensorId: &fixture.sensor.Id, Property: ptrStr("state"), Value: ptrStr("ON")},
			{Type: gen.AutomationStepTypeWait, Seconds: &seconds},
			{Type: gen.AutomationStepTypeSet, SensorId: &fixture.sensor.Id, Property: ptrStr("state"), Value: ptrStr("OFF")},
		},
	})
	require.Equal(t, http.StatusCreated, status, string(body))
	var created gen.Automation
	require.NoError(t, json.Unmarshal(body, &created))

	select {
	case on := <-published:
		assert.JSONEq(t, `{"state":"ON"}`, on.payload)
		acknowledge(on)
	case <-time.After(due.Sub(env.Clock.Now()) + 5*time.Second):
		t.Fatal("the automation never published its first command")
	}
	waitingRun := func() (gen.AutomationRun, bool) {
		runs, status := client.ListAutomationRuns(created.Id)
		require.Equal(t, http.StatusOK, status)
		if len(runs) != 1 || runs[0].Status != gen.AutomationRunStatusWaiting {
			return gen.AutomationRun{}, false
		}
		return runs[0], true
	}
	var waiting gen.AutomationRun
	require.Eventually(t, func() bool {
		var ok bool
		waiting, ok = waitingRun()
		return ok
	}, 10*time.Second, 100*time.Millisecond)
	require.NotNil(t, waiting.ResumeAt)

	require.NoError(t, env.Restart())

	_, stillWaiting := waitingRun()
	assert.True(t, stillWaiting, "the restart ended the waiting run")
	select {
	case off := <-published:
		assert.JSONEq(t, `{"state":"OFF"}`, off.payload)
		assert.False(t, off.at.Before(*waiting.ResumeAt), "the run carried on %s before its resume time", waiting.ResumeAt.Sub(off.at))
		acknowledge(off)
	case <-time.After(waiting.ResumeAt.Sub(env.Clock.Now()) + 10*time.Second):
		t.Fatal("the run never sent the step after its wait")
	}
	require.Eventually(t, func() bool {
		runs, status := client.ListAutomationRuns(created.Id)
		require.Equal(t, http.StatusOK, status)
		return len(runs) == 1 && runs[0].Status == gen.AutomationRunStatusSucceeded && len(runs[0].StepOutcomes) == 3
	}, 10*time.Second, 100*time.Millisecond)

	require.Equal(t, http.StatusOK, client.DeleteAutomation(created.Id))
}

func TestAutomation_RunNowOnAnAutomationThatIsOffCanBeCancelledAndThenDeleted(t *testing.T) {
	fixture := setupCommandFixture(t, fmt.Sprintf("run-now-plug-%d", reserveTCPPort(t)))
	defer fixture.stop()

	device := pahomqtt.NewClient(pahomqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://127.0.0.1:%d", fixture.port)).
		SetClientID(fmt.Sprintf("integration-run-now-%d", fixture.port)))
	token := device.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	defer device.Disconnect(250)
	token = device.Subscribe(fmt.Sprintf("zigbee2mqtt/%s/set", fixture.sensor.Name), 1, func(client pahomqtt.Client, msg pahomqtt.Message) {
		client.Publish(fmt.Sprintf("zigbee2mqtt/%s", fixture.sensor.Name), 1, false, msg.Payload())
	})
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())

	off := false
	hour := 3600
	body, status := client.CreateAutomation(gen.AutomationInput{
		Name:     "Integration run now",
		Enabled:  &off,
		Triggers: []gen.AutomationTrigger{{Type: gen.AutomationTriggerTypeSchedule, At: ptrStr("03:00"), Days: everyDay()}},
		Steps: []gen.AutomationStep{
			{Type: gen.AutomationStepTypeSet, SensorId: &fixture.sensor.Id, Property: ptrStr("state"), Value: ptrStr("ON")},
			{Type: gen.AutomationStepTypeWait, Seconds: &hour},
			{Type: gen.AutomationStepTypeSet, SensorId: &fixture.sensor.Id, Property: ptrStr("state"), Value: ptrStr("OFF")},
		},
	})
	require.Equal(t, http.StatusCreated, status, string(body))
	var created gen.Automation
	require.NoError(t, json.Unmarshal(body, &created))

	started, status := client.RunAutomation(created.Id)
	require.Equal(t, http.StatusAccepted, status)
	assert.Equal(t, gen.AutomationRunTriggerKindManual, started.TriggerKind)
	require.NotNil(t, started.InitiatedBy)
	require.Eventually(t, func() bool {
		runs, status := client.ListAutomationRuns(created.Id)
		require.Equal(t, http.StatusOK, status)
		return len(runs) == 1 && runs[0].Status == gen.AutomationRunStatusWaiting
	}, 10*time.Second, 100*time.Millisecond, "the run never acknowledged its first step and waited")

	assert.Equal(t, http.StatusConflict, client.DeleteAutomation(created.Id))

	cancelled, status := client.CancelAutomationRun(created.Id, started.Id)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, gen.AutomationRunStatusCancelled, cancelled.Status)
	_, status = client.CancelAutomationRun(created.Id, started.Id)
	assert.Equal(t, http.StatusConflict, status)

	require.Equal(t, http.StatusOK, client.DeleteAutomation(created.Id))
}

func TestAutomation_RejectsAReadingTriggerOnAMeasurementTypeTheSensorDoesNotReport(t *testing.T) {
	fixture := setupCommandFixture(t, fmt.Sprintf("unreported-plug-%d", reserveTCPPort(t)))
	defer fixture.stop()

	operator := gen.AutomationTriggerOperatorFallsBelow
	threshold, margin := 16.0, 0.2
	body, status := client.CreateAutomation(gen.AutomationInput{
		Name: "Plug heat on",
		Triggers: []gen.AutomationTrigger{{Type: gen.AutomationTriggerTypeReading, SensorId: &fixture.sensor.Id,
			MeasurementType: ptrStr("temperature"), Operator: &operator, Threshold: &threshold, RearmMargin: &margin}},
		Steps: []gen.AutomationStep{{Type: gen.AutomationStepTypeSet, SensorId: &fixture.sensor.Id, Property: ptrStr("state"), Value: ptrStr("ON")}},
	})

	assert.Equal(t, http.StatusBadRequest, status)
	assert.Contains(t, string(body), "triggers[0].measurement_type")
}

func TestAutomation_ATemperatureFallingBelowItsThresholdSwitchesThePlugOnce(t *testing.T) {
	fixture := setupCommandFixture(t, fmt.Sprintf("heater-plug-%d", reserveTCPPort(t)))
	defer fixture.stop()
	ctx := context.Background()
	climateName := fmt.Sprintf("lounge-climate-%d", fixture.port)
	sensorRepo := database.NewSensorRepository(env.DB, slog.Default())
	require.NoError(t, sensorRepo.AddSensor(ctx, gen.Sensor{Name: climateName, SensorDriver: "mqtt-zigbee2mqtt", Status: gen.SensorStatusActive, Config: map[string]string{}}))
	defer func() { _ = database.NewSensorRepository(env.DB, slog.Default()).DeleteSensorByName(ctx, climateName) }()
	climate, err := sensorRepo.GetSensorByName(ctx, climateName)
	require.NoError(t, err)

	device := pahomqtt.NewClient(pahomqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://127.0.0.1:%d", fixture.port)).
		SetClientID(fmt.Sprintf("integration-reading-%d", fixture.port)))
	token := device.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	defer device.Disconnect(250)
	published := make(chan time.Time, 10)
	token = device.Subscribe(fmt.Sprintf("zigbee2mqtt/%s/set", fixture.sensor.Name), 1, func(_ pahomqtt.Client, _ pahomqtt.Message) {
		published <- time.Now()
	})
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	publish := func(topic, payload string) time.Time {
		t.Helper()
		token := device.Publish(topic, 1, false, payload)
		require.True(t, token.WaitTimeout(5*time.Second))
		require.NoError(t, token.Error())
		return time.Now()
	}
	climateTopic := fmt.Sprintf("zigbee2mqtt/%s", climateName)

	publish(climateTopic, `{"temperature":16.5}`)
	require.Eventually(t, func() bool {
		body, status := client.GetMeasurementTypesForSensor(climate.Id)
		return status == http.StatusOK && strings.Contains(string(body), `"temperature"`)
	}, 5*time.Second, 100*time.Millisecond, "the climate sensor never reported a temperature")

	operator := gen.AutomationTriggerOperatorFallsBelow
	threshold, margin := 16.0, 0.2
	body, status := client.CreateAutomation(gen.AutomationInput{
		Name: "Lounge heat on",
		Triggers: []gen.AutomationTrigger{{Type: gen.AutomationTriggerTypeReading, SensorId: &climate.Id,
			MeasurementType: ptrStr("temperature"), Operator: &operator, Threshold: &threshold, RearmMargin: &margin}},
		Steps: []gen.AutomationStep{{Type: gen.AutomationStepTypeSet, SensorId: &fixture.sensor.Id, Property: ptrStr("state"), Value: ptrStr("ON")}},
	})
	require.Equal(t, http.StatusCreated, status, string(body))
	var created gen.Automation
	require.NoError(t, json.Unmarshal(body, &created))
	defer client.DeleteAutomation(created.Id)
	assert.Nil(t, created.NextFireAt)

	crossed := publish(climateTopic, `{"temperature":15.9}`)
	select {
	case at := <-published:
		assert.Less(t, at.Sub(crossed), time.Second, "the command went out %s after the reading", at.Sub(crossed))
	case <-time.After(5 * time.Second):
		t.Fatal("the automation never switched the plug")
	}
	publish(fmt.Sprintf("zigbee2mqtt/%s", fixture.sensor.Name), `{"state":"ON"}`)
	publish(climateTopic, `{"temperature":15.8}`)

	require.Eventually(t, func() bool {
		runs, status := client.ListAutomationRuns(created.Id)
		require.Equal(t, http.StatusOK, status)
		return len(runs) == 1 && runs[0].Status == gen.AutomationRunStatusSucceeded && runs[0].TriggerKind == gen.AutomationRunTriggerKindReading
	}, 10*time.Second, 100*time.Millisecond)
	select {
	case <-published:
		t.Fatal("a reading still below the threshold switched the plug again")
	case <-time.After(time.Second):
	}
}

func TestAutomation_TwoAutomationsSwitchingAPlugBackAndForthAreStoppedByTheLoopGuard(t *testing.T) {
	fixture := setupCommandFixture(t, fmt.Sprintf("loop-plug-%d", reserveTCPPort(t)))
	defer fixture.stop()
	plugTopic := fmt.Sprintf("zigbee2mqtt/%s", fixture.sensor.Name)

	device := pahomqtt.NewClient(pahomqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://127.0.0.1:%d", fixture.port)).
		SetClientID(fmt.Sprintf("integration-loop-%d", fixture.port)))
	token := device.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	defer device.Disconnect(250)
	token = device.Publish(plugTopic, 1, false, `{"state":"OFF"}`)
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	require.Eventually(t, func() bool {
		body, status := client.GetMeasurementTypesForSensor(fixture.sensor.Id)
		return status == http.StatusOK && strings.Contains(string(body), `"state"`)
	}, 5*time.Second, 100*time.Millisecond, "the plug never reported its state")

	operator := gen.AutomationTriggerOperatorBecomes
	create := func(name, when, then string) gen.Automation {
		t.Helper()
		body, status := client.CreateAutomation(gen.AutomationInput{
			Name: name,
			Triggers: []gen.AutomationTrigger{{Type: gen.AutomationTriggerTypeReading, SensorId: &fixture.sensor.Id,
				MeasurementType: ptrStr("state"), Operator: &operator, Value: ptrStr(when)}},
			Steps: []gen.AutomationStep{{Type: gen.AutomationStepTypeSet, SensorId: &fixture.sensor.Id, Property: ptrStr("state"), Value: ptrStr(then)}},
		})
		require.Equal(t, http.StatusCreated, status, string(body))
		var created gen.Automation
		require.NoError(t, json.Unmarshal(body, &created))
		return created
	}
	// The driver reports a binary state as "true" or "false".
	switchOff := create("Loop plug off", "true", "OFF")
	defer client.DeleteAutomation(switchOff.Id)
	switchOn := create("Loop plug on", "false", "ON")
	defer client.DeleteAutomation(switchOn.Id)

	// A real plug reports long after the previous run in the chain has recorded
	// that it finished. Reporting at once can reach that run's automation while
	// it is still recorded as running, which skips the next run and ends the
	// chain before the loop guard sees it.
	othersFinished := func() bool {
		running := 0
		for _, id := range []int{switchOff.Id, switchOn.Id} {
			runs, status := client.ListAutomationRuns(id)
			if status != http.StatusOK {
				return false
			}
			for _, run := range runs {
				if run.Status == gen.AutomationRunStatusRunning {
					running++
				}
			}
		}
		return running <= 1
	}
	token = device.Subscribe(plugTopic+"/set", 1, func(plug pahomqtt.Client, msg pahomqtt.Message) {
		for deadline := time.Now().Add(5 * time.Second); !othersFinished() && time.Now().Before(deadline); {
			time.Sleep(20 * time.Millisecond)
		}
		plug.Publish(plugTopic, 1, false, msg.Payload())
	})
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())

	_, status := client.RunAutomation(switchOn.Id)
	require.Equal(t, http.StatusAccepted, status)

	var offRuns []gen.AutomationRun
	require.Eventually(t, func() bool {
		offRuns, status = client.ListAutomationRuns(switchOff.Id)
		require.Equal(t, http.StatusOK, status)
		return len(offRuns) > 0 && offRuns[0].Status == gen.AutomationRunStatusFailed
	}, 15*time.Second, 100*time.Millisecond, "the loop guard never stopped the automations")
	time.Sleep(time.Second)

	onRuns, status := client.ListAutomationRuns(switchOn.Id)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, onRuns, 3, "the manual run and two runs caused by the other automation")
	offRuns, _ = client.ListAutomationRuns(switchOff.Id)
	require.Len(t, offRuns, 3, "a fifth run in the chain was refused and nothing ran after it")
	refused := offRuns[0]
	require.NotNil(t, refused.Error)
	assert.True(t, strings.HasPrefix(*refused.Error, "loop guard"), *refused.Error)
	assert.Equal(t, &gen.AutomationCauseRun{Id: onRuns[0].Id, AutomationId: switchOn.Id, AutomationName: "Loop plug on"}, refused.CauseRun)
	assert.Equal(t, &gen.AutomationCauseRun{Id: offRuns[1].Id, AutomationId: switchOff.Id, AutomationName: "Loop plug off"}, onRuns[0].CauseRun)
	assert.Nil(t, onRuns[2].CauseRun, "Run now has no cause")
}

func TestAutomation_SuggestsAReArmMarginFromASeriesReadings(t *testing.T) {
	const sensor = "Margin Suggestion Sensor"
	addSeededSensor(t, sensor)
	start := time.Now().UTC().Add(-2 * time.Hour)
	readings := make([]seededReading, 0, 100)
	for i := range 100 {
		readings = append(readings, seededReading{at: start.Add(time.Duration(i) * time.Minute), value: 20 + float64(i%2)*0.3})
	}
	seedReadings(t, sensor, "temperature", readings)
	var sensorID int
	require.NoError(t, env.DB.Reader.QueryRow(`SELECT id FROM sensors WHERE name = ?`, sensor).Scan(&sensorID))

	body, status := client.GetMarginSuggestion(sensorID, "temperature")

	require.Equal(t, http.StatusOK, status, string(body))
	assert.JSONEq(t, `{"suggested_margin":0.3,"step":0.3,"p95_change":0.3,"sample_count":100,"confidence":"medium"}`, string(body))
}

func TestAutomation_ADeviceListWithoutTheStepsPropertyBreaksTheAutomationUntilItComesBack(t *testing.T) {
	fixture := setupCommandFixture(t, fmt.Sprintf("broken-plug-%d", reserveTCPPort(t)))
	defer fixture.stop()

	bridge := pahomqtt.NewClient(pahomqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://127.0.0.1:%d", fixture.port)).
		SetClientID(fmt.Sprintf("integration-broken-%d", fixture.port)))
	token := bridge.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	defer bridge.Disconnect(250)
	publishDevices := func(exposes string) {
		t.Helper()
		payload := fmt.Sprintf(`[{"ieee_address":"0x%016x","friendly_name":%q,
			"definition":{"model":"TS011F","vendor":"Tuya","description":"Smart plug","exposes":%s}}]`,
			fixture.port, fixture.sensor.Name, exposes)
		pub := bridge.Publish("zigbee2mqtt/bridge/devices", 0, false, payload)
		require.True(t, pub.WaitTimeout(5*time.Second))
		require.NoError(t, pub.Error())
	}
	statusOf := func(id int) gen.Automation {
		automation, status := client.GetAutomation(id)
		require.Equal(t, http.StatusOK, status)
		return automation
	}

	body, status := client.CreateAutomation(gen.AutomationInput{
		Name:     "Integration broken plug",
		Triggers: []gen.AutomationTrigger{{Type: gen.AutomationTriggerTypeSchedule, At: ptrStr("03:00"), Days: everyDay()}},
		Steps:    []gen.AutomationStep{{Type: gen.AutomationStepTypeSet, SensorId: &fixture.sensor.Id, Property: ptrStr("state"), Value: ptrStr("ON")}},
	})
	require.Equal(t, http.StatusCreated, status, string(body))
	var created gen.Automation
	require.NoError(t, json.Unmarshal(body, &created))
	defer client.DeleteAutomation(created.Id)

	publishDevices(`[{"type":"binary","property":"child_lock","name":"child_lock","access":7,"value_on":"LOCK","value_off":"UNLOCK"}]`)

	require.Eventually(t, func() bool {
		return statusOf(created.Id).Status == gen.AutomationStatusBroken
	}, 5*time.Second, 100*time.Millisecond, "the device list refresh never broke the automation")
	broken := statusOf(created.Id)
	assert.Equal(t, ptrStr(fmt.Sprintf("step 1: %s no longer has state", fixture.sensor.Name)), broken.StatusReason)
	assert.Nil(t, broken.NextFireAt)
	_, status = client.RunAutomation(created.Id)
	assert.Equal(t, http.StatusConflict, status)
	runs, status := client.ListAutomationRuns(created.Id)
	require.Equal(t, http.StatusOK, status)
	assert.Empty(t, runs)

	publishDevices(`[{"type":"binary","property":"state","name":"state","access":7,"value_on":"ON","value_off":"OFF"}]`)

	require.Eventually(t, func() bool {
		return statusOf(created.Id).Status == gen.AutomationStatusArmed
	}, 5*time.Second, 100*time.Millisecond, "the automation stayed broken after the property came back")
	assert.NotNil(t, statusOf(created.Id).NextFireAt)
}

func TestAutomation_DeletingASensorDeletesTheAutomationsThatUseItWithTheirActiveRun(t *testing.T) {
	fixture := setupCommandFixture(t, fmt.Sprintf("kept-plug-%d", reserveTCPPort(t)))
	defer fixture.stop()
	ctx := context.Background()
	doomedName := fmt.Sprintf("doomed-lamp-%d", fixture.port)
	metadata := map[string]interface{}{"exposes": []interface{}{map[string]interface{}{
		"type": "binary", "property": "state", "access": float64(7), "value_on": "ON", "value_off": "OFF",
	}}}
	sensorRepo := database.NewSensorRepository(env.DB, slog.Default())
	require.NoError(t, sensorRepo.AddSensor(ctx, gen.Sensor{Name: doomedName, SensorDriver: "mqtt-zigbee2mqtt",
		Status: gen.SensorStatusActive, Config: map[string]string{}, Metadata: &metadata}))
	defer func() { _ = database.NewSensorRepository(env.DB, slog.Default()).DeleteSensorByName(ctx, doomedName) }()
	doomed, err := sensorRepo.GetSensorByName(ctx, doomedName)
	require.NoError(t, err)

	device := pahomqtt.NewClient(pahomqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://127.0.0.1:%d", fixture.port)).
		SetClientID(fmt.Sprintf("integration-doomed-%d", fixture.port)))
	token := device.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	defer device.Disconnect(250)
	token = device.Subscribe(fmt.Sprintf("zigbee2mqtt/%s/set", fixture.sensor.Name), 1, func(client pahomqtt.Client, msg pahomqtt.Message) {
		client.Publish(fmt.Sprintf("zigbee2mqtt/%s", fixture.sensor.Name), 1, false, msg.Payload())
	})
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	reading := device.Publish(fmt.Sprintf("zigbee2mqtt/%s", doomedName), 1, false, `{"temperature":20}`)
	require.True(t, reading.WaitTimeout(5*time.Second))
	require.NoError(t, reading.Error())
	require.Eventually(t, func() bool {
		body, status := client.GetMeasurementTypesForSensor(doomed.Id)
		return status == http.StatusOK && strings.Contains(string(body), `"temperature"`)
	}, 5*time.Second, 100*time.Millisecond, "the doomed sensor never reported a temperature")

	create := func(input gen.AutomationInput) gen.Automation {
		t.Helper()
		body, status := client.CreateAutomation(input)
		require.Equal(t, http.StatusCreated, status, string(body))
		var created gen.Automation
		require.NoError(t, json.Unmarshal(body, &created))
		return created
	}
	setStep := func(sensorID int, value string) gen.AutomationStep {
		return gen.AutomationStep{Type: gen.AutomationStepTypeSet, SensorId: &sensorID, Property: ptrStr("state"), Value: ptrStr(value)}
	}
	schedule := []gen.AutomationTrigger{{Type: gen.AutomationTriggerTypeSchedule, At: ptrStr("03:00"), Days: everyDay()}}
	hour := 3600
	stepOnDoomed := create(gen.AutomationInput{Name: "Kept plug, then doomed lamp", Triggers: schedule, Steps: []gen.AutomationStep{
		setStep(fixture.sensor.Id, "ON"), {Type: gen.AutomationStepTypeWait, Seconds: &hour}, setStep(fixture.sensor.Id, "OFF"), setStep(doomed.Id, "OFF"),
	}})
	operator := gen.AutomationTriggerOperatorRisesAbove
	threshold, margin := 30.0, 0.5
	triggerOnDoomed := create(gen.AutomationInput{Name: "Kept plug when doomed lamp is hot", Triggers: []gen.AutomationTrigger{{
		Type: gen.AutomationTriggerTypeReading, SensorId: &doomed.Id, MeasurementType: ptrStr("temperature"),
		Operator: &operator, Threshold: &threshold, RearmMargin: &margin,
	}}, Steps: []gen.AutomationStep{setStep(fixture.sensor.Id, "ON")}})
	unrelated := create(gen.AutomationInput{Name: "Kept plug only", Triggers: schedule, Steps: []gen.AutomationStep{setStep(fixture.sensor.Id, "ON")}})
	defer client.DeleteAutomation(unrelated.Id)

	_, status := client.RunAutomation(stepOnDoomed.Id)
	require.Equal(t, http.StatusAccepted, status)
	require.Eventually(t, func() bool {
		runs, status := client.ListAutomationRuns(stepOnDoomed.Id)
		require.Equal(t, http.StatusOK, status)
		return len(runs) == 1 && runs[0].Status == gen.AutomationRunStatusWaiting
	}, 10*time.Second, 100*time.Millisecond, "the run never acknowledged its first step and waited")

	require.Equal(t, http.StatusOK, client.DeleteSensor(doomedName))

	for _, deleted := range []gen.Automation{stepOnDoomed, triggerOnDoomed} {
		_, status := client.GetAutomation(deleted.Id)
		assert.Equal(t, http.StatusNotFound, status, "%s was not deleted with its sensor", deleted.Name)
	}
	_, status = client.GetAutomation(unrelated.Id)
	assert.Equal(t, http.StatusOK, status)
	history, status := client.GetSensorCommandHistory(fixture.sensor.Id)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, history, 1)
	assert.Nil(t, history[0].AutomationRunId)
	assert.Nil(t, history[0].Automation)
}
