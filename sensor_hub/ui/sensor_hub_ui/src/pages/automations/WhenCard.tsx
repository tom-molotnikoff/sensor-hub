import { Fragment } from 'react';
import { Button, IconButton, MenuItem, TextField, ToggleButton, ToggleButtonGroup, Typography } from '@mui/material';
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutlineOutlined';
import Card from '../../ui/Card';
import Inline from '../../ui/Inline';
import Stack from '../../ui/Stack';
import { newTrigger, type DraftTrigger } from './automationDraft';
import { weekdays, type Weekday } from './automationText';

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
        <TextField select size="small" label="Trigger" value={trigger.type} disabled={readOnly}>
          <MenuItem value="schedule">At a time</MenuItem>
        </TextField>
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
        {!readOnly && (
          <IconButton size="small" aria-label={`Remove trigger ${position}`} onClick={onRemove}>
            <DeleteOutlineIcon fontSize="small" />
          </IconButton>
        )}
      </Inline>
    </Card>
  );
}

interface WhenCardProps {
  triggers: DraftTrigger[];
  readOnly: boolean;
  onChange: (triggers: DraftTrigger[]) => void;
}

export default function WhenCard({ triggers, readOnly, onChange }: WhenCardProps) {
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
      </Stack>
    </Card>
  );
}
