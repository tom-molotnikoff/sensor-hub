//go:build integration

// Package benchmarks measures the paths users feel, reading ingest and
// reading queries, with and without automations. Compare the two runs of
// each benchmark with benchstat:
//
//	go test -tags integration -run '^$' -bench . -count 6 ./benchmarks/ | tee bench.txt
//	benchstat -col /automations bench.txt
package benchmarks

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"testing"
	"time"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/testharness"
)

const (
	sensorCount        = 10
	automationCount    = 200
	readingAutomations = 100
	historyDays        = 1
	// About what ten Zigbee devices publishing every 5 s send, twenty times
	// over.
	backgroundBatchEvery = 25 * time.Millisecond
)

type hub struct {
	env     *testharness.Env
	client  *testharness.Client
	sensors []gen.Sensor
	plug    gen.Sensor
}

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.Run()
}

func BenchmarkProcess(b *testing.B) {
	for _, automations := range []int{0, automationCount} {
		b.Run(fmt.Sprintf("automations=%d", automations), func(b *testing.B) {
			h := startHub(b, automations)
			ctx := context.Background()
			latencies := make([]time.Duration, 0, b.N)
			for i := 0; b.Loop(); i++ {
				sensor := h.sensors[i%len(h.sensors)]
				batch := []gen.Reading{temperature(i, time.Now().UTC())}
				started := time.Now()
				if err := h.env.Readings.Process(ctx, sensor, batch); err != nil {
					b.Fatalf("process: %v", err)
				}
				latencies = append(latencies, time.Since(started))
			}
			reportP95(b, latencies)
		})
	}
}

// Readings keep arriving while the query runs, so the automation worker is
// busy on the reading triggers, as it is on a hub with live sensors.
func BenchmarkReadingsBetween(b *testing.B) {
	for _, automations := range []int{0, automationCount} {
		b.Run(fmt.Sprintf("automations=%d", automations), func(b *testing.B) {
			h := startHub(b, automations)
			stop := h.feed(b)
			defer stop()
			end := time.Now().UTC()
			start := end.Add(-historyDays * 24 * time.Hour)
			latencies := make([]time.Duration, 0, b.N)
			for i := 0; b.Loop(); i++ {
				sensor := h.sensors[i%len(h.sensors)]
				started := time.Now()
				response, status := h.client.GetReadingsBetweenAggregated(start.Format(time.RFC3339), end.Format(time.RFC3339), sensor.Name, "temperature", "", "")
				latencies = append(latencies, time.Since(started))
				if status != http.StatusOK || len(response.Readings) == 0 {
					b.Fatalf("readings between: status %d, %d readings", status, len(response.Readings))
				}
			}
			reportP95(b, latencies)
		})
	}
}

func startHub(b *testing.B, automations int) *hub {
	b.Helper()
	env := testharness.StartServer(b, nil)
	client := testharness.NewClient(b, env.ServerURL)
	client.LoginAdmin(env)
	if status := client.ChangePassword(env.AdminPass); status != http.StatusOK {
		b.Fatalf("change the admin password: status %d", status)
	}
	h := &hub{env: env, client: client}
	h.addSensors(b)
	h.addHistory(b)
	h.addAutomations(b, automations)
	return h
}

func (h *hub) addSensors(b *testing.B) {
	b.Helper()
	ctx := context.Background()
	sensors := database.NewSensorRepository(h.env.DB, slog.Default())
	exposes := map[string]interface{}{"exposes": []interface{}{map[string]interface{}{
		"type": "binary", "property": "state", "access": float64(7), "value_on": "ON", "value_off": "OFF",
	}}}
	add := func(name string, metadata *map[string]interface{}) gen.Sensor {
		if err := sensors.AddSensor(ctx, gen.Sensor{Name: name, SensorDriver: "mqtt-zigbee2mqtt", Status: gen.SensorStatusActive,
			Config: map[string]string{}, Metadata: metadata}); err != nil {
			b.Fatalf("add sensor %s: %v", name, err)
		}
		sensor, err := sensors.GetSensorByName(ctx, name)
		if err != nil || sensor == nil {
			b.Fatalf("find sensor %s: %v", name, err)
		}
		return *sensor
	}
	for i := range sensorCount {
		h.sensors = append(h.sensors, add(fmt.Sprintf("room-%d", i), nil))
	}
	h.plug = add("plug", &exposes)
}

func (h *hub) addHistory(b *testing.B) {
	b.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	for _, sensor := range h.sensors {
		var batch []gen.Reading
		for minute := historyDays * 24 * 60; minute > 0; minute-- {
			batch = append(batch, temperature(minute, now.Add(-time.Duration(minute)*time.Minute)))
		}
		if err := h.env.Readings.Process(ctx, sensor, batch); err != nil {
			b.Fatalf("add history to %s: %v", sensor.Name, err)
		}
	}
}

// Every reading trigger watches a series the benchmark feeds, with a threshold
// it never crosses, so the worker checks every trigger on every reading and
// no run starts. The schedule triggers come due half a day away.
func (h *hub) addAutomations(b *testing.B, count int) {
	b.Helper()
	property, value := "state", "ON"
	steps := []gen.AutomationStep{{Type: gen.AutomationStepTypeSet, SensorId: &h.plug.Id, Property: &property, Value: &value}}
	measurement := "temperature"
	margin := 0.5
	at := time.Now().UTC().Add(12 * time.Hour).Format("15:04")
	days := []gen.AutomationTriggerDays{gen.AutomationDayMon, gen.AutomationDayTue, gen.AutomationDayWed,
		gen.AutomationDayThu, gen.AutomationDayFri, gen.AutomationDaySat, gen.AutomationDaySun}
	for i := range count {
		trigger := gen.AutomationTrigger{Type: gen.AutomationTriggerTypeSchedule, At: &at, Days: &days}
		if i < readingAutomations {
			operator, threshold := gen.AutomationTriggerOperatorFallsBelow, -50.0
			if i%2 == 1 {
				operator, threshold = gen.AutomationTriggerOperatorRisesAbove, 150.0
			}
			trigger = gen.AutomationTrigger{Type: gen.AutomationTriggerTypeReading, SensorId: &h.sensors[i%len(h.sensors)].Id,
				MeasurementType: &measurement, Operator: &operator, Threshold: &threshold, RearmMargin: &margin}
		}
		body, status := h.client.CreateAutomation(gen.AutomationInput{Name: fmt.Sprintf("automation %d", i),
			Triggers: []gen.AutomationTrigger{trigger}, Steps: steps})
		if status != http.StatusCreated {
			b.Fatalf("create automation %d: status %d: %s", i, status, body)
		}
	}
}

func (h *hub) feed(b *testing.B) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(backgroundBatchEvery)
		defer ticker.Stop()
		for i := 0; ; i++ {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			sensor := h.sensors[i%len(h.sensors)]
			if err := h.env.Readings.Process(ctx, sensor, []gen.Reading{temperature(i, time.Now().UTC())}); err != nil && ctx.Err() == nil {
				b.Errorf("background process: %v", err)
				return
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func temperature(i int, at time.Time) gen.Reading {
	value := 20 + 2*math.Sin(float64(i)/30)
	return gen.Reading{MeasurementType: "temperature", NumericValue: &value, Time: at.Format(time.RFC3339)}
}

func reportP95(b *testing.B, latencies []time.Duration) {
	if len(latencies) == 0 {
		return
	}
	slices.Sort(latencies)
	index := int(math.Ceil(0.95*float64(len(latencies)))) - 1
	b.ReportMetric(float64(latencies[index].Microseconds()), "p95-µs")
}
