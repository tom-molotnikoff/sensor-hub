import { Box, IconButton, Link, MenuItem, Select, Stack, Switch, TextField, Typography } from '@mui/material';
import UndoIcon from '@mui/icons-material/Undo';
import type { PropertyDefinition } from '../gen/aliases';
import { useIsMobile } from '../hooks/useMobile';
import { ApplyChip, ApplyNote } from './PropertyApplyNotice';
import { applySegment } from './propertyApplyCopy';

interface PropertyFieldProps {
  definition: PropertyDefinition;
  described?: boolean;
  serverValue?: string;
  editedValue?: string;
  collided?: boolean;
  error?: string;
  onChange: (value: string) => void;
  onUndo?: () => void;
  disabled?: boolean;
}

interface PropertyControlProps {
  definition: PropertyDefinition;
  value: string;
  unset: boolean;
  invalid: boolean;
  onChange: (value: string) => void;
  disabled?: boolean;
}

function PropertyControl({ definition, value, unset, invalid, onChange, disabled }: PropertyControlProps) {
  if (definition.readOnly) {
    return <Typography sx={{ fontFamily: 'monospace' }} color="text.secondary">{value}</Typography>;
  }

  if (definition.type === 'bool') {
    return (
      <Switch
        checked={(unset ? definition.default : value) === 'true'}
        onChange={(e) => onChange(e.target.checked ? 'true' : 'false')}
        disabled={disabled}
        slotProps={{ input: { role: 'switch', 'aria-label': definition.label } }}
      />
    );
  }

  if (definition.enum) {
    return (
      <Select
        value={unset ? definition.default : value}
        onChange={(e) => onChange(e.target.value)}
        size="small"
        error={invalid}
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
        value={value}
        placeholder={unset ? definition.default : undefined}
        onChange={(e) => onChange(e.target.value)}
        size="small"
        error={invalid}
        disabled={disabled}
        slotProps={{
          htmlInput: {
            'aria-label': definition.label,
            inputMode: definition.type === 'int' ? 'numeric' : undefined,
          },
        }}
      />
      {definition.unit && <Typography variant="body2" color="text.secondary">{definition.unit}</Typography>}
    </Stack>
  );
}

function shown(value: string): string {
  return value === '' ? '(empty)' : value;
}

function helperLine(definition: PropertyDefinition, described: boolean, serverValue: string): string {
  const saved = `Saved value ${shown(serverValue)}`;
  if (!described) return saved;
  const parts = [saved, `default ${shown(definition.default)}`];
  const apply = applySegment(definition, serverValue);
  if (apply) parts.push(apply);
  return parts.join(' · ');
}

export default function PropertyField({ definition, described = true, serverValue, editedValue, collided, error, onChange, onUndo, disabled }: PropertyFieldProps) {
  const isMobile = useIsMobile();
  const value = editedValue ?? serverValue ?? '';
  const modified = editedValue !== undefined && editedValue !== serverValue;
  const unset = serverValue === undefined && editedValue === undefined;

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
          {described && <ApplyChip definition={definition} />}
        </Stack>
        {described && (
          <>
            <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
              {definition.key}
            </Typography>
            <Typography variant="body2" color="text.secondary">{definition.description}</Typography>
          </>
        )}
        {definition.readOnly && (
          <Typography variant="body2" color="text.secondary">
            Set at install time and not changeable at runtime.
          </Typography>
        )}
      </Box>
      <Box sx={{ flex: '1 1 auto' }}>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
          <PropertyControl
            definition={definition}
            value={value}
            unset={unset}
            invalid={error !== undefined}
            onChange={onChange}
            disabled={disabled}
          />
          {modified && !collided && onUndo && (
            <IconButton size="small" aria-label={`Undo changes to ${definition.label}`} onClick={onUndo}>
              <UndoIcon fontSize="small" />
            </IconButton>
          )}
        </Stack>
        {error !== undefined && (
          <Typography variant="body2" color="error" sx={{ mt: 0.5 }}>
            {error}
          </Typography>
        )}
        {described && <ApplyNote definition={definition} />}
        {modified && (
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
            {collided ? `Someone else changed this to ${shown(serverValue ?? '')}` : helperLine(definition, described, serverValue ?? '')}
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
