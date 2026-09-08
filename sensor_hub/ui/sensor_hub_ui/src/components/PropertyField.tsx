import { Box, IconButton, Link, MenuItem, Select, Stack, Switch, TextField, Typography } from '@mui/material';
import UndoIcon from '@mui/icons-material/Undo';
import type { PropertyDefinition } from '../gen/aliases';
import { useIsMobile } from '../hooks/useMobile';
import { ApplyChip, ApplyNote } from './PropertyApplyNotice';
import { applySegment } from './propertyApplyCopy';

interface PropertyFieldProps {
  definition: PropertyDefinition;
  serverValue?: string;
  editedValue?: string;
  collided?: boolean;
  onChange: (value: string) => void;
  onUndo?: () => void;
  disabled?: boolean;
}

interface PropertyControlProps {
  definition: PropertyDefinition;
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}

function PropertyControl({ definition, value, onChange, disabled }: PropertyControlProps) {
  if (definition.readOnly) {
    return <Typography sx={{ fontFamily: 'monospace' }} color="text.secondary">{value}</Typography>;
  }

  if (definition.type === 'bool') {
    return (
      <Switch
        checked={value === 'true'}
        onChange={(e) => onChange(e.target.checked ? 'true' : 'false')}
        disabled={disabled}
        slotProps={{ input: { role: 'switch', 'aria-label': definition.label } }}
      />
    );
  }

  if (definition.enum) {
    return (
      <Select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        size="small"
        disabled={disabled}
        slotProps={{ input: { 'aria-label': definition.label } }}
      >
        {definition.enum.map((option) => (
          <MenuItem key={option} value={option}>{option}</MenuItem>
        ))}
      </Select>
    );
  }

  return (
    <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
      <TextField
        type={definition.type === 'int' ? 'number' : 'text'}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        size="small"
        disabled={disabled}
        slotProps={{ htmlInput: { 'aria-label': definition.label } }}
      />
      {definition.unit && <Typography variant="body2" color="text.secondary">{definition.unit}</Typography>}
    </Stack>
  );
}

function shown(value: string): string {
  return value === '' ? '(empty)' : value;
}

function helperLine(definition: PropertyDefinition, serverValue: string): string {
  const parts = [`Saved value ${shown(serverValue)}`, `default ${shown(definition.default)}`];
  const apply = applySegment(definition, serverValue);
  if (apply) parts.push(apply);
  return parts.join(' · ');
}

export default function PropertyField({ definition, serverValue, editedValue, collided, onChange, onUndo, disabled }: PropertyFieldProps) {
  const isMobile = useIsMobile();
  const value = editedValue ?? serverValue ?? '';
  const modified = editedValue !== undefined && editedValue !== serverValue;

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: isMobile ? 'column' : 'row',
        alignItems: isMobile ? 'stretch' : 'center',
        gap: 2,
        py: 1.5,
        ...(modified && { borderLeft: 3, borderColor: 'primary.main', pl: 2, ml: -2 }),
      }}
    >
      <Box sx={{ flex: isMobile ? '0 0 auto' : '0 0 420px', minWidth: 160 }}>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
          <Typography sx={{ fontWeight: 500 }} color={modified ? 'primary' : 'textPrimary'}>
            {definition.label}
          </Typography>
          <ApplyChip definition={definition} />
        </Stack>
        <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
          {definition.key}
        </Typography>
        <Typography variant="body2" color="text.secondary">{definition.description}</Typography>
        {definition.readOnly && (
          <Typography variant="body2" color="text.secondary">
            Set at install time and not changeable at runtime.
          </Typography>
        )}
      </Box>
      <Box sx={{ flex: '1 1 auto' }}>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
          <PropertyControl definition={definition} value={value} onChange={onChange} disabled={disabled} />
          {modified && !collided && onUndo && (
            <IconButton size="small" aria-label={`Undo changes to ${definition.label}`} onClick={onUndo}>
              <UndoIcon fontSize="small" />
            </IconButton>
          )}
        </Stack>
        <ApplyNote definition={definition} />
        {modified && (
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
            {collided ? `Someone else changed this to ${shown(serverValue ?? '')}` : helperLine(definition, serverValue ?? '')}
            {collided && onUndo && (
              <>
                {' \u00b7 '}
                <Link
                  component="button"
                  type="button"
                  variant="body2"
                  onClick={onUndo}
                  aria-label={`Reset ${definition.label} to the value someone else saved`}
                >
                  Reset
                </Link>
              </>
            )}
          </Typography>
        )}
      </Box>
    </Box>
  );
}
