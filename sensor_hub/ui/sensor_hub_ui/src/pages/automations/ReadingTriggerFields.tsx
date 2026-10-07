import { useEffect } from 'react';
import { MenuItem, TextField } from '@mui/material';
import type { AutomationTrigger } from '../../gen/aliases';
import { useMarginSuggestion } from '../../hooks/useAutomations';
import { useSensorMeasurementTypes } from '../../hooks/useMeasurementTypes';
import { useSensorContext } from '../../hooks/useSensorContext';
import DurationField from './DurationField';
import NumberField from './NumberField';
import { choosePrompt, type DraftTrigger } from './automationDraft';
import { operatorNames } from './automationText';
import { useSensorName } from './useSensorName';

type Operator = NonNullable<AutomationTrigger['operator']>;

const numericOperators: Operator[] = ['falls_below', 'rises_above'];

const holdUnits = [
  { name: 'seconds', seconds: 1 },
  { name: 'minutes', seconds: 60 },
  { name: 'hours', seconds: 3_600 },
] as const;

// Binary readings arrive as the text "true" or "false".
const binaryValues = ['true', 'false'];

interface ReadingTriggerFieldsProps {
  trigger: DraftTrigger;
  readOnly: boolean;
  onChange: (trigger: DraftTrigger) => void;
}

export default function ReadingTriggerFields({ trigger, readOnly, onChange }: ReadingTriggerFieldsProps) {
  const { sensors } = useSensorContext();
  const sensorName = useSensorName();
  const { measurementTypes } = useSensorMeasurementTypes(trigger.sensor_id ?? null);
  const measurementType = measurementTypes.find((each) => each.name === trigger.measurement_type);
  const binary = trigger.operator === 'becomes';
  const operators = binary ? (['becomes'] as Operator[]) : numericOperators;
  const unit = measurementType?.unit;
  const values = trigger.value === undefined || binaryValues.includes(trigger.value) ? binaryValues : [...binaryValues, trigger.value];
  // A new series starts its threshold and margin afresh, so the fields that keep what was typed start again too.
  const series = `${trigger.sensor_id}:${trigger.measurement_type}`;

  const suggestion = useMarginSuggestion(
    trigger.awaitingMargin ? trigger.sensor_id : undefined,
    trigger.awaitingMargin ? trigger.measurement_type : undefined,
  );

  useEffect(() => {
    if (!trigger.awaitingMargin || !suggestion.isFetched) return;
    const suggested = suggestion.data?.suggested_margin ?? undefined;
    if (trigger.rearm_margin !== undefined || suggested === undefined) {
      onChange({ ...trigger, awaitingMargin: false });
    } else {
      onChange({ ...trigger, awaitingMargin: false, rearm_margin: suggested, suggestedMargin: suggested });
    }
  }, [trigger, suggestion.isFetched, suggestion.data, onChange]);

  const freshSeries = { threshold: undefined, rearm_margin: undefined, margin_hint: undefined, suggestedMargin: undefined };
  const chooseSensor = (id: number) =>
    onChange({ ...trigger, ...freshSeries, sensor_id: id, measurement_type: undefined, operator: 'falls_below', value: undefined, awaitingMargin: false });
  const chooseMeasurementType = (name: string) => {
    const chosen = measurementTypes.find((each) => each.name === name);
    if (chosen?.category === 'binary') {
      onChange({ ...trigger, ...freshSeries, measurement_type: name, operator: 'becomes', value: binaryValues[0], awaitingMargin: false });
    } else {
      onChange({ ...trigger, ...freshSeries, measurement_type: name, operator: binary ? 'falls_below' : trigger.operator, value: undefined, awaitingMargin: true });
    }
  };
  const marginHelp =
    trigger.margin_hint != null
      ? `This sensor is noisier now: suggested ${trigger.margin_hint}${unit ? ` ${unit}` : ''}`
      : trigger.suggestedMargin !== undefined && trigger.rearm_margin === trigger.suggestedMargin
        ? 'Suggested from recent readings'
        : undefined;

  return (
    <>
      <TextField
        select
        size="small"
        label="Sensor"
        value={trigger.sensor_id ?? ''}
        slotProps={choosePrompt('Choose a sensor', (id) => sensorName(Number(id)))}
        disabled={readOnly}
        onChange={(event) => chooseSensor(Number(event.target.value))}
      >
        {trigger.sensor_id !== undefined && !sensors.some((each) => each.id === trigger.sensor_id) && (
          <MenuItem value={trigger.sensor_id}>{sensorName(trigger.sensor_id)}</MenuItem>
        )}
        {sensors.map((each) => (
          <MenuItem key={each.id} value={each.id}>
            {each.name}
          </MenuItem>
        ))}
      </TextField>
      <TextField
        select
        size="small"
        label="Measurement"
        value={trigger.measurement_type ?? ''}
        slotProps={choosePrompt('Choose a measurement')}
        disabled={readOnly || trigger.sensor_id === undefined}
        onChange={(event) => chooseMeasurementType(event.target.value)}
      >
        {trigger.measurement_type !== undefined && !measurementType && (
          <MenuItem value={trigger.measurement_type}>{trigger.measurement_type}</MenuItem>
        )}
        {measurementTypes.map((each) => (
          <MenuItem key={each.name} value={each.name}>
            {each.display_name}
          </MenuItem>
        ))}
      </TextField>
      <TextField
        select
        size="small"
        label="Condition"
        value={trigger.operator ?? ''}
        disabled={readOnly}
        onChange={(event) => onChange({ ...trigger, operator: event.target.value as Operator })}
      >
        {operators.map((operator) => (
          <MenuItem key={operator} value={operator}>
            {operatorNames[operator]}
          </MenuItem>
        ))}
      </TextField>
      {binary ? (
        <TextField
          select
          size="small"
          label="Value"
          value={trigger.value ?? ''}
          disabled={readOnly}
          onChange={(event) => onChange({ ...trigger, value: event.target.value })}
        >
          {values.map((value) => (
            <MenuItem key={value} value={value}>
              {value}
            </MenuItem>
          ))}
        </TextField>
      ) : (
        <>
          <NumberField
            key={`threshold-${series}`}
            label={unit ? `Threshold (${unit})` : 'Threshold'}
            value={trigger.threshold}
            readOnly={readOnly}
            onChange={(threshold) => onChange({ ...trigger, threshold })}
          />
          <NumberField
            key={`margin-${series}-${trigger.suggestedMargin}`}
            label="Re-arm margin"
            value={trigger.rearm_margin}
            min={0}
            step={0.1}
            helperText={marginHelp}
            readOnly={readOnly}
            onChange={(rearm_margin) => onChange({ ...trigger, rearm_margin, awaitingMargin: false })}
          />
        </>
      )}
      <DurationField
        label="For at least"
        seconds={trigger.hold_seconds}
        units={holdUnits}
        min={0}
        readOnly={readOnly}
        onChange={(hold_seconds) => onChange({ ...trigger, hold_seconds })}
      />
    </>
  );
}
