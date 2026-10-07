import { MenuItem, TextField } from '@mui/material';
import type { AutomationTrigger } from '../../gen/aliases';
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

  const chooseSensor = (id: number) =>
    onChange({ ...trigger, sensor_id: id, measurement_type: undefined, operator: 'falls_below', threshold: undefined, rearm_margin: undefined, value: undefined });
  const chooseMeasurementType = (name: string) => {
    const chosen = measurementTypes.find((each) => each.name === name);
    if (chosen?.category === 'binary') {
      onChange({ ...trigger, measurement_type: name, operator: 'becomes', threshold: undefined, rearm_margin: undefined, value: binaryValues[0] });
    } else {
      onChange({ ...trigger, measurement_type: name, operator: binary ? 'falls_below' : trigger.operator, threshold: undefined, rearm_margin: undefined, value: undefined });
    }
  };

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
            key={`margin-${series}`}
            label="Re-arm margin"
            value={trigger.rearm_margin}
            min={0}
            step={0.1}
            readOnly={readOnly}
            onChange={(rearm_margin) => onChange({ ...trigger, rearm_margin })}
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
