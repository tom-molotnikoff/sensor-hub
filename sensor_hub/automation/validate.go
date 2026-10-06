package automation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	gen "example/sensorHub/gen"
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func invalid(format string, args ...any) error {
	return &ValidationError{Message: fmt.Sprintf(format, args...)}
}

var timeOfDay = regexp.MustCompile(`^([01][0-9]|2[0-3]):([0-5][0-9])$`)

var weekdayNames = map[gen.AutomationTriggerDays]time.Weekday{
	gen.AutomationDayMon: time.Monday,
	gen.AutomationDayTue: time.Tuesday,
	gen.AutomationDayWed: time.Wednesday,
	gen.AutomationDayThu: time.Thursday,
	gen.AutomationDayFri: time.Friday,
	gen.AutomationDaySat: time.Saturday,
	gen.AutomationDaySun: time.Sunday,
}

func fromInput(ctx context.Context, sensors SensorLookup, input gen.AutomationInput) (Automation, error) {
	automation := Automation{Name: strings.TrimSpace(input.Name), Enabled: true}
	if input.Enabled != nil {
		automation.Enabled = *input.Enabled
	}
	if automation.Name == "" {
		return Automation{}, invalid("name is required")
	}

	if len(input.Triggers) == 0 {
		return Automation{}, invalid("triggers must hold at least one trigger")
	}
	for i, trigger := range input.Triggers {
		parsed, err := triggerFromInput(i, trigger)
		if err != nil {
			return Automation{}, err
		}
		automation.Triggers = append(automation.Triggers, parsed)
	}

	if len(input.Steps) == 0 {
		return Automation{}, invalid("steps must hold at least one step")
	}
	for i, step := range input.Steps {
		parsed, err := stepFromInput(ctx, sensors, i, step)
		if err != nil {
			return Automation{}, err
		}
		automation.Steps = append(automation.Steps, parsed)
	}
	return automation, nil
}

func triggerFromInput(i int, trigger gen.AutomationTrigger) (Trigger, error) {
	if trigger.Type != gen.AutomationTriggerTypeSchedule {
		return Trigger{}, invalid("triggers[%d].type must be %q, got %q", i, gen.AutomationTriggerTypeSchedule, trigger.Type)
	}

	at := ""
	if trigger.At != nil {
		at = *trigger.At
	}
	match := timeOfDay.FindStringSubmatch(at)
	if match == nil {
		return Trigger{}, invalid("triggers[%d].at must be a time of day as HH:MM from 00:00 to 23:59, got %q", i, at)
	}
	hour, _ := strconv.Atoi(match[1])
	minute, _ := strconv.Atoi(match[2])

	if trigger.Days == nil || len(*trigger.Days) == 0 {
		return Trigger{}, invalid("triggers[%d].days must hold at least one weekday", i)
	}
	var days Weekdays
	for _, name := range *trigger.Days {
		day, ok := weekdayNames[name]
		if !ok {
			return Trigger{}, invalid("triggers[%d].days: %q is not a weekday; use mon, tue, wed, thu, fri, sat or sun", i, name)
		}
		days |= WeekdaysOf(day)
	}

	return Trigger{Kind: TriggerSchedule, Schedule: &Schedule{MinuteOfDay: hour*60 + minute, Days: days}}, nil
}

func stepFromInput(ctx context.Context, sensors SensorLookup, i int, step gen.AutomationStep) (Step, error) {
	switch step.Type {
	case gen.AutomationStepTypeSet:
		return setStepFromInput(ctx, sensors, i, step)
	case gen.AutomationStepTypeWait:
		if step.Seconds == nil || *step.Seconds < 1 {
			return Step{}, invalid("steps[%d].seconds must be a whole number of seconds, at least 1", i)
		}
		return Step{Kind: StepWait, Seconds: *step.Seconds}, nil
	default:
		return Step{}, invalid("steps[%d].type must be %q or %q, got %q", i, gen.AutomationStepTypeSet, gen.AutomationStepTypeWait, step.Type)
	}
}

func setStepFromInput(ctx context.Context, sensors SensorLookup, i int, step gen.AutomationStep) (Step, error) {
	if step.SensorId == nil {
		return Step{}, invalid("steps[%d].sensor_id is required", i)
	}
	if step.Property == nil || *step.Property == "" {
		return Step{}, invalid("steps[%d].property is required", i)
	}
	if step.Value == nil {
		return Step{}, invalid("steps[%d].value is required", i)
	}
	parsed := Step{Kind: StepSet, SensorID: *step.SensorId, Property: *step.Property, Value: *step.Value}

	sensor, err := sensors.ServiceGetSensorById(ctx, parsed.SensorID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && sensor == nil) {
		return Step{}, invalid("steps[%d].sensor_id: sensor %d does not exist", i, parsed.SensorID)
	}
	if err != nil {
		return Step{}, fmt.Errorf("look up sensor %d: %w", parsed.SensorID, err)
	}

	capability, err := writableCapability(*sensor, parsed.Property)
	if err != nil {
		return Step{}, invalid("steps[%d].property: %s", i, err)
	}
	if err := checkValue(capability, parsed.Value); err != nil {
		return Step{}, invalid("steps[%d].value: %s", i, err)
	}
	return parsed, nil
}

// The sensor must come from the sensor service, which fills in capabilities.
func writableCapability(sensor gen.Sensor, property string) (gen.Capability, error) {
	if sensor.Capabilities == nil || len(*sensor.Capabilities) == 0 {
		return gen.Capability{}, fmt.Errorf("%s has no writable capabilities", sensor.Name)
	}
	for _, capability := range *sensor.Capabilities {
		if capability.Property == property {
			return capability, nil
		}
	}
	return gen.Capability{}, fmt.Errorf("%s has no writable property %q", sensor.Name, property)
}

func checkValue(capability gen.Capability, value string) error {
	switch capability.Type {
	case gen.CapabilityTypeBinary:
		on, off := "ON", "OFF"
		if capability.ValueOn != nil {
			on = *capability.ValueOn
		}
		if capability.ValueOff != nil {
			off = *capability.ValueOff
		}
		// The driver matches binary values without regard to case.
		if !strings.EqualFold(value, on) && !strings.EqualFold(value, off) {
			return fmt.Errorf("%q is not %q or %q", value, on, off)
		}
	case gen.CapabilityTypeNumeric:
		number, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("%q is not a number", value)
		}
		if capability.Min != nil && number < *capability.Min {
			return fmt.Errorf("%s is below the minimum of %s", value, strconv.FormatFloat(*capability.Min, 'f', -1, 64))
		}
		if capability.Max != nil && number > *capability.Max {
			return fmt.Errorf("%s is above the maximum of %s", value, strconv.FormatFloat(*capability.Max, 'f', -1, 64))
		}
	case gen.CapabilityTypeEnum:
		if capability.Values == nil {
			return nil
		}
		for _, allowed := range *capability.Values {
			if allowed == value {
				return nil
			}
		}
		return fmt.Errorf("%q is not one of %s", value, strings.Join(*capability.Values, ", "))
	default:
		return fmt.Errorf("capability type %q cannot be set", capability.Type)
	}
	return nil
}
