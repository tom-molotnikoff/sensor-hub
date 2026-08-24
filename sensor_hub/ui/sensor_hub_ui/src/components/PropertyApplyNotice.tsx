import { Chip, Typography } from '@mui/material';
import type { PropertyDefinition } from '../gen/aliases';
import { CONSEQUENCE_NOTES, actionChipLabel, applyAction } from './propertyApplyCopy';

// The one chip the page is allowed: what an action property requires. Not interactive.
export function ApplyChip({ definition }: { definition: PropertyDefinition }) {
  const action = applyAction(definition);
  if (!action) return null;
  return <Chip label={actionChipLabel(action)} size="small" color="warning" variant="outlined" />;
}

// One muted line beneath the control where an action has a consequence worth stating.
export function ApplyNote({ definition }: { definition: PropertyDefinition }) {
  const note = CONSEQUENCE_NOTES[definition.key];
  if (!note) return null;
  return (
    <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
      {note}
    </Typography>
  );
}
