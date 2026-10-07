import { Fragment, useId } from 'react';
import { Button, IconButton, MenuItem, TextField, ToggleButton, ToggleButtonGroup, Typography } from '@mui/material';
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutlineOutlined';
import type { Automation } from '../../gen/aliases';
import Card from '../../ui/Card';
import Inline from '../../ui/Inline';
import Stack from '../../ui/Stack';
import DurationField from './DurationField';
import ReadingTriggerFields from './ReadingTriggerFields';
import { newTrigger, retyped, type DraftTrigger } from './automationDraft';
import { weekdays, type Weekday } from './automationText';

const intervalUnits = [
  { name: 'minutes', seconds: 60 },
  { name: 'hours', seconds: 3_600 },
] as const;

interface TriggerCardProps {
  trigger: DraftTrigger;
  position: number;
  readOnly: boolean;
  onChange: (trigger: DraftTrigger) => void;
  onRemove: () => void;
}

function TriggerCard({ trigger, position, readOnly, onChange, onRemove }: TriggerCardProps) {
  const days = trigger.days ?? [];

  return (
    <Card variant="inset">
      <Inline>
        <TextField
          select
          size="small"
          label="Trigger"
          value={trigger.type}
          disabled={readOnly}
          onChange={(event) => onChange(retyped(trigger, event.target.value as DraftTrigger['type']))}
        >
          <MenuItem value="schedule">At a time</MenuItem>
          <MenuItem value="interval">Every…</MenuItem>
          <MenuItem value="reading">Sensor reading</MenuItem>
        </TextField>
        {trigger.type === 'reading' ? (
          <ReadingTriggerFields trigger={trigger} readOnly={readOnly} onChange={onChange} />
        ) : trigger.type === 'interval' ? (
          <DurationField
            label="Interval"
            seconds={trigger.seconds}
            units={intervalUnits}
            readOnly={readOnly}
            onChange={(seconds) => onChange({ ...trigger, seconds })}
          />
        ) : (
          <>
            <TextField
              type="time"
              size="small"
              label="Time"
              value={trigger.at ?? ''}
              disabled={readOnly}
              onChange={(event) => onChange({ ...trigger, at: event.target.value })}
            />
            <ToggleButtonGroup
              size="small"
              color="primary"
              aria-label="Weekdays"
              value={days}
              disabled={readOnly}
              onChange={(_, chosen: Weekday[]) =>
                onChange({ ...trigger, days: weekdays.map(({ day }) => day).filter((day) => chosen.includes(day)) })
              }
            >
              {weekdays.map(({ day, letter, name }) => (
                <ToggleButton key={day} value={day} aria-label={name}>
                  {letter}
                </ToggleButton>
              ))}
            </ToggleButtonGroup>
          </>
        )}
        {!readOnly && (
          <IconButton size="small" aria-label={`Remove trigger ${position}`} onClick={onRemove}>
            <DeleteOutlineIcon fontSize="small" />
          </IconButton>
        )}
      </Inline>
    </Card>
  );
}

interface ModeFieldProps {
  mode: Automation['mode'];
  readOnly: boolean;
  onChange: (mode: Automation['mode']) => void;
}

function ModeField({ mode, readOnly, onChange }: ModeFieldProps) {
  const labelId = useId();
  return (
    <Inline>
      <Typography id={labelId} variant="bodySmall" color="text.secondary">
        If a trigger fires while already running:
      </Typography>
      <TextField
        select
        size="small"
        value={mode}
        disabled={readOnly}
        slotProps={{ select: { labelId } }}
        onChange={(event) => onChange(event.target.value as Automation['mode'])}
      >
        <MenuItem value="single">Ignore it (single)</MenuItem>
        <MenuItem value="restart">Start over (restart)</MenuItem>
      </TextField>
    </Inline>
  );
}

interface WhenCardProps {
  triggers: DraftTrigger[];
  mode: Automation['mode'];
  readOnly: boolean;
  onChange: (triggers: DraftTrigger[]) => void;
  onModeChange: (mode: Automation['mode']) => void;
}

export default function WhenCard({ triggers, mode, readOnly, onChange, onModeChange }: WhenCardProps) {
  return (
    <Card title="When any of these happens">
      <Stack>
        {triggers.map((trigger, index) => (
          <Fragment key={trigger.key}>
            {index > 0 && (
              <Typography variant="sectionTitle" color="text.secondary" align="center">
                OR
              </Typography>
            )}
            <TriggerCard
              trigger={trigger}
              position={index + 1}
              readOnly={readOnly}
              onChange={(changed) => onChange(triggers.map((each) => (each.key === changed.key ? changed : each)))}
              onRemove={() => onChange(triggers.filter((each) => each.key !== trigger.key))}
            />
          </Fragment>
        ))}
        {!readOnly && (
          <Button variant="outlined" onClick={() => onChange([...triggers, newTrigger()])}>
            + Add trigger
          </Button>
        )}
        <ModeField mode={mode} readOnly={readOnly} onChange={onModeChange} />
      </Stack>
    </Card>
  );
}
