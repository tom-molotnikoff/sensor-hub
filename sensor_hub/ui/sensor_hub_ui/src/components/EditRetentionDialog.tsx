import { useEffect, useState } from 'react';
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  MenuItem,
  Switch,
  TextField,
  Typography,
} from '@mui/material';
import type { Sensor } from '../gen/aliases';
import { apiClient } from '../gen/client';
import { formatRetention } from '../tools/retention';
import { logger } from '../tools/logger';

type RetentionUnit = 'hours' | 'days' | 'weeks';

const unitMultipliers: Record<RetentionUnit, number> = {
  hours: 1,
  days: 24,
  weeks: 168,
};

function unitToHours(value: number, unit: RetentionUnit): number {
  return Math.round(value * unitMultipliers[unit]);
}

function hoursToUnit(hours: number, unit: RetentionUnit): number {
  return Math.round((hours / unitMultipliers[unit]) * 100) / 100;
}

function bestUnit(hours: number): RetentionUnit {
  if (hours >= 168 && hours % 168 === 0) return 'weeks';
  if (hours >= 24 && hours % 24 === 0) return 'days';
  return 'hours';
}

interface EditRetentionDialogProps {
  open: boolean;
  onClose: () => void;
  onSaved: (sensorId: number, retentionHours: number | null) => void;
  sensor: Sensor | null;
  globalRetentionHours: number;
}

export default function EditRetentionDialog({ open, onClose, onSaved, sensor, globalRetentionHours }: EditRetentionDialogProps) {
  const [useCustom, setUseCustom] = useState(false);
  const [unit, setUnit] = useState<RetentionUnit>('days');
  const [value, setValue] = useState('');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open && sensor) {
      void Promise.resolve().then(() => {
        const hasCustom = sensor.retention_hours != null;
        setUseCustom(hasCustom);
        setError(null);
        if (hasCustom && sensor.retention_hours != null) {
          const u = bestUnit(sensor.retention_hours);
          setUnit(u);
          setValue(String(hoursToUnit(sensor.retention_hours, u)));
        } else {
          setUnit('days');
          setValue('');
        }
      });
    }
  }, [open, sensor]);

  const pendingEffectiveHours = useCustom && value
    ? unitToHours(parseFloat(value), unit)
    : globalRetentionHours;

  const handleSave = async () => {
    if (!sensor) return;
    setError(null);
    try {
      const retentionHours = useCustom ? unitToHours(parseFloat(value), unit) : null;
      if (useCustom && (!retentionHours || retentionHours < 1)) {
        setError('Retention must be at least 1 hour');
        return;
      }
      await apiClient.PUT('/sensors/{id}', { params: { path: { id: sensor.id } }, body: { retention_hours: retentionHours } as never });
      onClose();
      onSaved(sensor.id, retentionHours);
    } catch (e) {
      logger.error('Failed to update retention', e);
      setError('Failed to update retention');
    }
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Edit Data Retention</DialogTitle>
      <DialogContent>
        <TextField
          fullWidth
          label="Sensor"
          value={sensor?.name || ''}
          disabled
          sx={{ mt: 1 }}
        />

        <Typography
          variant="body2"
          sx={{
            color: "text.secondary",
            mt: 2
          }}>
          Effective: <strong>{formatRetention(pendingEffectiveHours)}</strong>
          {' '}(global default: {formatRetention(globalRetentionHours)})
        </Typography>

        <FormControlLabel
          control={
            <Switch
              checked={useCustom}
              onChange={(e) => {
                setUseCustom(e.target.checked);
                if (!e.target.checked) setValue('');
              }}
            />
          }
          label="Override global retention"
          sx={{ mt: 2 }}
        />

        {useCustom && (
          <Box
            sx={{
              display: "flex",
              gap: 2,
              mt: 2
            }}>
            <TextField
              label="Retention"
              type="number"
              value={value}
              onChange={(e) => setValue(e.target.value)}
              slotProps={{ htmlInput: { min: 1, step: 1 } }}
              sx={{ flex: 1 }}
            />
            <TextField
              select
              label="Unit"
              value={unit}
              onChange={(e) => {
                const newUnit = e.target.value as RetentionUnit;
                if (value) {
                  const hours = unitToHours(parseFloat(value), unit);
                  setValue(String(hoursToUnit(hours, newUnit)));
                }
                setUnit(newUnit);
              }}
              sx={{ minWidth: 120 }}
            >
              <MenuItem value="hours">Hours</MenuItem>
              <MenuItem value="days">Days</MenuItem>
              <MenuItem value="weeks">Weeks</MenuItem>
            </TextField>
          </Box>
        )}

        {error && (
          <Typography color="error" variant="body2" sx={{ mt: 2 }}>
            {error}
          </Typography>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" onClick={handleSave}>Save</Button>
      </DialogActions>
    </Dialog>
  );
}
