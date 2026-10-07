package automation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
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
	automation := Automation{Name: strings.TrimSpace(input.Name), Enabled: true, Mode: ModeSingle}
	if input.Enabled != nil {
		automation.Enabled = *input.Enabled
	}
	if input.Mode != nil {
		switch mode := Mode(*input.Mode); mode {
		case ModeSingle, ModeRestart:
			automation.Mode = mode
		default:
			return Automation{}, invalid("mode must be %q or %q, got %q", ModeSingle, ModeRestart, mode)
		}
	}
	if automation.Name == "" {
		return Automation{}, invalid("name is required")
	}

	if len(input.Triggers) == 0 {
		return Automation{}, invalid("triggers must hold at least one trigger")
	}
	for i, trigger := range input.Triggers {
		parsed, err := triggerFromInput(ctx, sensors, i, trigger)
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

func triggerFromInput(ctx context.Context, sensors SensorLookup, i int, trigger gen.AutomationTrigger) (Trigger, error) {
	switch trigger.Type {
	case gen.AutomationTriggerTypeSchedule:
		return scheduleTriggerFromInput(i, trigger)
	case gen.AutomationTriggerTypeInterval:
		return intervalTriggerFromInput(i, trigger)
	case gen.AutomationTriggerTypeReading:
		return readingTriggerFromInput(ctx, sensors, i, trigger)
	default:
		return Trigger{}, invalid("triggers[%d].type must be %q, %q or %q, got %q", i,
			gen.AutomationTriggerTypeSchedule, gen.AutomationTriggerTypeInterval, gen.AutomationTriggerTypeReading, trigger.Type)
	}
}

func scheduleTriggerFromInput(i int, trigger gen.AutomationTrigger) (Trigger, error) {
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

// Longer intervals do not fit in a time.Duration.
const maxIntervalSeconds = math.MaxInt64 / int64(time.Second)

func intervalTriggerFromInput(i int, trigger gen.AutomationTrigger) (Trigger, error) {
	if trigger.Seconds == nil || *trigger.Seconds < 60 {
		return Trigger{}, invalid("triggers[%d].seconds must be a whole number of seconds, at least 60", i)
	}
	if int64(*trigger.Seconds) > maxIntervalSeconds {
		return Trigger{}, invalid("triggers[%d].seconds must be at most %d", i, maxIntervalSeconds)
	}
	return Trigger{Kind: TriggerInterval, Interval: time.Duration(*trigger.Seconds) * time.Second}, nil
}

func readingTriggerFromInput(ctx context.Context, sensors SensorLookup, i int, trigger gen.AutomationTrigger) (Trigger, error) {
	if trigger.SensorId == nil {
		return Trigger{}, invalid("triggers[%d].sensor_id is required", i)
	}
	if trigger.MeasurementType == nil || *trigger.MeasurementType == "" {
		return Trigger{}, invalid("triggers[%d].measurement_type is required", i)
	}
	if trigger.Operator == nil {
		return Trigger{}, invalid("triggers[%d].operator is required", i)
	}
	condition := ReadingCondition{Series: Series{SensorID: *trigger.SensorId}, Operator: Operator(*trigger.Operator)}

	measurementType, err := reportedType(ctx, sensors, fmt.Sprintf("triggers[%d].", i), condition.SensorID, *trigger.MeasurementType)
	if err != nil {
		return Trigger{}, err
	}
	condition.MeasurementType, condition.MeasurementTypeID = measurementType.Name, measurementType.Id

	if trigger.HoldSeconds != nil {
		if *trigger.HoldSeconds < 0 || int64(*trigger.HoldSeconds) > maxIntervalSeconds {
			return Trigger{}, invalid("triggers[%d].hold_seconds must be a whole number of seconds from 0 to %d", i, maxIntervalSeconds)
		}
		condition.Hold = time.Duration(*trigger.HoldSeconds) * time.Second
	}

	if measurementType.Category == gen.MeasurementTypeCategoryBinary {
		if condition.Operator != Becomes {
			return Trigger{}, invalid("triggers[%d].operator: %s is binary, so the operator must be %q, got %q", i, measurementType.Name, Becomes, condition.Operator)
		}
		if trigger.RearmMargin != nil {
			return Trigger{}, invalid("triggers[%d].rearm_margin: a binary trigger has no margin", i)
		}
		if trigger.Value == nil || *trigger.Value == "" {
			return Trigger{}, invalid("triggers[%d].value is required", i)
		}
		condition.Value = *trigger.Value
		return Trigger{Kind: TriggerReading, Reading: &condition}, nil
	}

	if condition.Operator != FallsBelow && condition.Operator != RisesAbove {
		return Trigger{}, invalid("triggers[%d].operator: %s is numeric, so the operator must be %q or %q, got %q", i, measurementType.Name, FallsBelow, RisesAbove, condition.Operator)
	}
	if trigger.Threshold == nil {
		return Trigger{}, invalid("triggers[%d].threshold is required", i)
	}
	if trigger.RearmMargin == nil || *trigger.RearmMargin < 0 {
		return Trigger{}, invalid("triggers[%d].rearm_margin is required, and must be 0 or more", i)
	}
	condition.Threshold, condition.Margin = *trigger.Threshold, *trigger.RearmMargin
	return Trigger{Kind: TriggerReading, Reading: &condition}, nil
}

// reportedType names the fields at fault with the given prefix.
func reportedType(ctx context.Context, sensors SensorLookup, prefix string, sensorID int, name string) (gen.MeasurementType, error) {
	sensor, err := sensors.ServiceGetSensorById(ctx, sensorID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && sensor == nil) {
		return gen.MeasurementType{}, invalid("%ssensor_id: sensor %d does not exist", prefix, sensorID)
	}
	if err != nil {
		return gen.MeasurementType{}, fmt.Errorf("look up sensor %d: %w", sensorID, err)
	}
	reported, err := sensors.ServiceGetMeasurementTypesForSensor(ctx, sensorID)
	if err != nil {
		return gen.MeasurementType{}, fmt.Errorf("look up measurement types of sensor %d: %w", sensorID, err)
	}
	index := slices.IndexFunc(reported, func(each gen.MeasurementType) bool { return each.Name == name })
	if index < 0 {
		return gen.MeasurementType{}, invalid("%smeasurement_type: %s does not report %q", prefix, sensor.Name, name)
	}
	return reported[index], nil
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
