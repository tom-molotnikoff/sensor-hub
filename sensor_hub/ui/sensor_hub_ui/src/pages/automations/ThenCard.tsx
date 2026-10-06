import { useState, type DragEvent, type KeyboardEvent } from 'react';
import { Button, IconButton, MenuItem, Slider, TextField, ToggleButton, ToggleButtonGroup, Typography } from '@mui/material';
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutlineOutlined';
import DragIndicatorIcon from '@mui/icons-material/DragIndicator';
import type { Capability } from '../../gen/aliases';
import { useSensorContext } from '../../hooks/useSensorContext';
import ActionBar from '../../ui/ActionBar';
import Card from '../../ui/Card';
import Inline from '../../ui/Inline';
import Stack from '../../ui/Stack';
import { defaultValue, moved, newStep, writableCapabilities, type DraftStep } from './automationDraft';
import { useSensorName } from './useSensorName';

interface ValueControlProps {
  capability: Capability;
  value: string | undefined;
  readOnly: boolean;
  onChange: (value: string) => void;
}

function ValueControl({ capability, value, readOnly, onChange }: ValueControlProps) {
  switch (capability.type) {
    case 'binary':
      return (
        <ToggleButtonGroup
          exclusive
          size="small"
          color="primary"
          aria-label="Value"
          value={value ?? null}
          disabled={readOnly}
          onChange={(_, chosen: string | null) => chosen !== null && onChange(chosen)}
        >
          {[capability.value_on, capability.value_off].map((option) => (
            <ToggleButton key={option} value={option ?? ''}>
              {option}
            </ToggleButton>
          ))}
        </ToggleButtonGroup>
      );
    case 'numeric': {
      const min = capability.min ?? 0;
      const max = capability.max ?? 100;
      const number = Number(value ?? min);
      return (
        <ActionBar
          picker={
            <Slider
              aria-label="Value"
              min={min}
              max={max}
              value={Number.isFinite(number) ? number : min}
              disabled={readOnly}
              onChange={(_, chosen) => onChange(String(chosen))}
            />
          }
        >
          <TextField
            type="number"
            size="small"
            label={capability.unit ? `Value (${capability.unit})` : 'Value'}
            value={value ?? ''}
            disabled={readOnly}
            slotProps={{ htmlInput: { min, max } }}
            onChange={(event) => onChange(event.target.value)}
          />
          <Typography variant="bodySmall" color="text.secondary">
            {min}-{max}
          </Typography>
        </ActionBar>
      );
    }
    case 'enum':
      return (
        <TextField select size="small" label="Value" value={value ?? ''} disabled={readOnly} onChange={(event) => onChange(event.target.value)}>
          {(capability.values ?? []).map((option) => (
            <MenuItem key={option} value={option}>
              {option}
            </MenuItem>
          ))}
        </TextField>
      );
  }
}

const choosePrompt = (prompt: string, display: (value: string) => string = (value) => value) => ({
  inputLabel: { shrink: true },
  select: { displayEmpty: true, renderValue: (value: unknown) => (value === '' ? prompt : display(String(value))) },
});

interface StepCardProps {
  step: DraftStep;
  position: number;
  count: number;
  readOnly: boolean;
  onChange: (step: DraftStep) => void;
  onRemove: () => void;
  onMove: (to: number) => void;
  onDragStart: () => void;
}

function StepCard({ step, position, count, readOnly, onChange, onRemove, onMove, onDragStart }: StepCardProps) {
  const { sensors } = useSensorContext();
  const sensorName = useSensorName();
  const controllable = sensors.filter((sensor) => writableCapabilities(sensor).length > 0);
  const sensor = sensors.find((each) => each.id === step.sensor_id);
  const capabilities = writableCapabilities(sensor);
  const capability = capabilities.find((each) => each.property === step.property);

  const chooseSensor = (id: number) => {
    const first = writableCapabilities(sensors.find((each) => each.id === id))[0];
    onChange({ ...step, sensor_id: id, property: first?.property, value: defaultValue(first) });
  };
  const chooseProperty = (property: string) =>
    onChange({ ...step, property, value: defaultValue(capabilities.find((each) => each.property === property)) });

  const moveWithKeys = (event: KeyboardEvent) => {
    const to = { ArrowUp: position - 2, ArrowDown: position }[event.key];
    if (to === undefined || to < 0 || to >= count) return;
    event.preventDefault();
    onMove(to);
  };

  return (
    <Card variant="inset">
      <Stack>
        <Inline>
          {!readOnly && (
            <IconButton
              size="small"
              draggable
              aria-label={`Move step ${position}`}
              title="Drag, or use the arrow keys, to reorder"
              onDragStart={(event) => {
                const card = event.currentTarget.closest('[data-ui=card]');
                if (card) event.dataTransfer.setDragImage(card, 0, 0);
                event.dataTransfer.setData('text/plain', String(position));
                onDragStart();
              }}
              onKeyDown={moveWithKeys}
            >
              <DragIndicatorIcon fontSize="small" />
            </IconButton>
          )}
          <Typography variant="sectionTitle">{position}</Typography>
          <Typography variant="body">Set</Typography>
          <TextField
            select
            size="small"
            label="Device"
            value={step.sensor_id ?? ''}
            slotProps={choosePrompt('Choose a device', (id) => sensorName(Number(id)))}
            disabled={readOnly}
            onChange={(event) => chooseSensor(Number(event.target.value))}
          >
            {step.sensor_id !== undefined && !controllable.some((each) => each.id === step.sensor_id) && (
              <MenuItem value={step.sensor_id}>{sensorName(step.sensor_id)}</MenuItem>
            )}
            {controllable.map((each) => (
              <MenuItem key={each.id} value={each.id}>
                {each.name}
              </MenuItem>
            ))}
          </TextField>
          <TextField
            select
            size="small"
            label="Property"
            value={step.property ?? ''}
            slotProps={choosePrompt('Choose a property')}
            disabled={readOnly || step.sensor_id === undefined}
            onChange={(event) => chooseProperty(event.target.value)}
          >
            {step.property !== undefined && !capability && <MenuItem value={step.property}>{step.property}</MenuItem>}
            {capabilities.map((each) => (
              <MenuItem key={each.property} value={each.property}>
                {each.property}
              </MenuItem>
            ))}
          </TextField>
          {!capability && step.value !== undefined && <Typography variant="body">to {step.value}</Typography>}
          {!readOnly && (
            <IconButton size="small" aria-label={`Remove step ${position}`} onClick={onRemove}>
              <DeleteOutlineIcon fontSize="small" />
            </IconButton>
          )}
        </Inline>
        {capability && (
          <ValueControl
            capability={capability}
            value={step.value}
            readOnly={readOnly}
            onChange={(value) => onChange({ ...step, value })}
          />
        )}
      </Stack>
    </Card>
  );
}

interface ThenCardProps {
  steps: DraftStep[];
  readOnly: boolean;
  onChange: (steps: DraftStep[]) => void;
}

export default function ThenCard({ steps, readOnly, onChange }: ThenCardProps) {
  const [dragged, setDragged] = useState<number | null>(null);

  const dropOn = (index: number) => (event: DragEvent) => {
    if (dragged === null) return;
    event.preventDefault();
    if (event.type === 'drop') {
      onChange(moved(steps, dragged, index));
      setDragged(null);
    }
  };

  return (
    <Card title="Then in this order">
      <Stack>
        {steps.map((step, index) => (
          <div key={step.key} onDragOver={dropOn(index)} onDrop={dropOn(index)} onDragEnd={() => setDragged(null)}>
            <StepCard
              step={step}
              position={index + 1}
              count={steps.length}
              readOnly={readOnly}
              onChange={(changed) => onChange(steps.map((each) => (each.key === changed.key ? changed : each)))}
              onRemove={() => onChange(steps.filter((each) => each.key !== step.key))}
              onMove={(to) => onChange(moved(steps, index, to))}
              onDragStart={() => setDragged(index)}
            />
          </div>
        ))}
        {!readOnly && (
          <Button variant="outlined" onClick={() => onChange([...steps, newStep()])}>
            + Set a device
          </Button>
        )}
      </Stack>
    </Card>
  );
}
