import { useState, useEffect } from 'react';
import {
  Box,
  TextField,
  Button,
  Switch,
  FormControlLabel,
  Alert,
  Typography,
  CircularProgress,
  MenuItem,
} from '@mui/material';
import type { Sensor } from '../gen/aliases';
import LayoutCard from '../tools/LayoutCard';
import { TypographyH2 } from '../tools/Typography';
import { apiClient } from '../gen/client';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import { useProperties } from '../hooks/useProperties';
import { bestUnit, formatRetention, hoursToUnit, unitToHours, type RetentionUnit } from '../tools/retention';

interface SensorRetentionCardProps {
  sensor: Sensor;
}

function SensorRetentionCard({ sensor }: SensorRetentionCardProps) {
  const { user } = useAuth();
  const properties = useProperties();
  const globalRetentionDays = parseInt(properties['sensor.data.retention.days'] || '90', 10);
  const globalRetentionHours = globalRetentionDays * 24;

  const [useCustom, setUseCustom] = useState(sensor.retention_hours !== null);
  const [unit, setUnit] = useState<RetentionUnit>('days');
  const [value, setValue] = useState('');
  const [saving, setSaving] = useState(false);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  useEffect(() => {
    void Promise.resolve().then(() => {
      const hasCustom = sensor.retention_hours != null;
      setUseCustom(hasCustom);
      if (hasCustom && sensor.retention_hours != null) {
        const h = sensor.retention_hours;
        const u = bestUnit(h);
        setUnit(u);
        setValue(String(hoursToUnit(h, u)));
      } else {
        setUnit('days');
        setValue('');
      }
    });
  }, [sensor.retention_hours]);

  const fieldsDisabled = !user || !hasPerm(user, 'manage_sensors');

  const pendingEffectiveHours = useCustom && value
    ? unitToHours(parseFloat(value), unit)
    : globalRetentionHours;

  const handleSave = async () => {
    setSuccessMessage(null);
    setErrorMessage(null);
    setSaving(true);
    try {
      const retentionHours = useCustom ? unitToHours(parseFloat(value), unit) : null;
      if (useCustom && (!retentionHours || retentionHours < 1)) {
        setErrorMessage('Retention must be at least 1 hour');
        setSaving(false);
        return;
      }
      await apiClient.PUT('/sensors/{id}', { params: { path: { id: sensor.id } }, body: { retention_hours: retentionHours } as never });
      setSuccessMessage(retentionHours ? `Retention set to ${formatRetention(retentionHours)}` : 'Reverted to global default');
    } catch {
      setErrorMessage('Failed to update retention');
    } finally {
      setSaving(false);
    }
  };

  const hasChanges = (() => {
    if (useCustom !== (sensor.retention_hours !== null)) return true;
    if (useCustom && value !== '') {
      const newHours = unitToHours(parseFloat(value), unit);
      return newHours !== sensor.retention_hours;
    }
    return false;
  })();

  return (
    <LayoutCard variant="secondary" changes={{ height: '100%', width: '100%' }}>
      <TypographyH2>Data Retention</TypographyH2>
      <Typography
        variant="body2"
        sx={{
          color: "text.secondary",
          mb: 2
        }}>
        Effective: <strong>{formatRetention(pendingEffectiveHours)}</strong>
        {' '}(global default: {formatRetention(globalRetentionHours)})
      </Typography>
      {successMessage && (
        <Alert severity="success" onClose={() => setSuccessMessage(null)} sx={{ mb: 2 }}>
          {successMessage}
        </Alert>
      )}
      {errorMessage && (
        <Alert severity="error" onClose={() => setErrorMessage(null)} sx={{ mb: 2 }}>
          {errorMessage}
        </Alert>
      )}
      <FormControlLabel
        control={
          <Switch
            checked={useCustom}
            onChange={(e) => {
              setUseCustom(e.target.checked);
              if (!e.target.checked) setValue('');
            }}
            disabled={fieldsDisabled}
          />
        }
        label="Override global retention for this sensor"
      />
      {useCustom && (
        <Box sx={{ display: 'flex', gap: 2, alignItems: 'flex-start', mt: 2 }}>
          <TextField
            label="Retention"
            type="number"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            disabled={fieldsDisabled}
            slotProps={{ htmlInput: { min: 1, step: 1 } }}
            size="small"
            sx={{ width: 140 }}
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
            disabled={fieldsDisabled}
            size="small"
            sx={{ width: 120 }}
          >
            <MenuItem value="hours">Hours</MenuItem>
            <MenuItem value="days">Days</MenuItem>
            <MenuItem value="weeks">Weeks</MenuItem>
          </TextField>
        </Box>
      )}
      <Box sx={{ display: 'flex', justifyContent: 'flex-end', mt: 2 }}>
        <Button
          variant="contained"
          onClick={handleSave}
          disabled={fieldsDisabled || saving || !hasChanges}
          startIcon={saving ? <CircularProgress color="inherit" size={18} /> : undefined}
        >
          {saving ? 'Saving...' : 'Save'}
        </Button>
      </Box>
    </LayoutCard>
  );
}

export default SensorRetentionCard;
