import { IconButton, Link, MenuItem, Select, Switch, TextField, Typography } from '@mui/material';
import UndoIcon from '@mui/icons-material/Undo';
import type { PropertyDefinition } from '../gen/aliases';
import { ApplyChip, ApplyNote } from './PropertyApplyNotice';
import { applySegment } from './propertyApplyCopy';
import Inline from '../ui/Inline';
import PageGrid from '../ui/PageGrid';

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
    <Inline>
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
    </Inline>
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
  const value = editedValue ?? serverValue ?? '';
  const modified = editedValue !== undefined && editedValue !== serverValue;
  const unset = serverValue === undefined && editedValue === undefined;

  return (
    <PageGrid>
      <PageGrid.Item span={{ wide: 5 }}>
        <Inline>
          <Typography variant="sectionTitle" component="span" color={modified ? 'primary' : 'textPrimary'}>
            {definition.label}
          </Typography>
          {described && <ApplyChip definition={definition} />}
        </Inline>
        {described && (
          <>
            <Typography variant="code" color="text.secondary">
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
      </PageGrid.Item>
      <PageGrid.Item span={{ wide: 7 }}>
        <Inline>
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
        </Inline>
        {error !== undefined && (
          <Typography variant="body2" color="error">
            {error}
          </Typography>
        )}
        {described && <ApplyNote definition={definition} />}
        {modified && (
          <Typography variant="body2" color="text.secondary">
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
      </PageGrid.Item>
    </PageGrid>
  );
}
