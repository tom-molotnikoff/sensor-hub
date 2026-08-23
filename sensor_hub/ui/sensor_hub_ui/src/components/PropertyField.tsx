import { Box, MenuItem, Select, Stack, Switch, TextField, Typography } from '@mui/material';
import type { PropertyDefinition } from '../gen/aliases';
import { useIsMobile } from '../hooks/useMobile';

interface PropertyFieldProps {
  definition: PropertyDefinition;
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}

function PropertyControl({ definition, value, onChange, disabled }: PropertyFieldProps) {
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

export default function PropertyField(props: PropertyFieldProps) {
  const { definition } = props;
  const isMobile = useIsMobile();

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: isMobile ? 'column' : 'row',
        alignItems: isMobile ? 'stretch' : 'center',
        gap: 2,
        py: 1.5,
      }}
    >
      <Box sx={{ flex: isMobile ? '0 0 auto' : '0 0 420px', minWidth: 160 }}>
        <Typography sx={{ fontWeight: 500 }}>{definition.label}</Typography>
        <Typography variant="body2" color="text.secondary">{definition.description}</Typography>
        {definition.readOnly && (
          <Typography variant="body2" color="text.secondary">
            Set at install time and not changeable at runtime.
          </Typography>
        )}
      </Box>
      <Box sx={{ flex: '1 1 auto' }}>
        <PropertyControl {...props} />
      </Box>
    </Box>
  );
}
