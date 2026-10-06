//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

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

// TestAutomation_ASchedulePublishesTheCommandAsTheSystem waits for the next
// whole minute, so it takes up to a minute.
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

	now := time.Now().UTC()
	due := now.Truncate(time.Minute).Add(time.Minute)
	if due.Sub(now) < 3*time.Second {
		due = due.Add(time.Minute)
	}
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
	case <-time.After(time.Until(due) + 5*time.Second):
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
