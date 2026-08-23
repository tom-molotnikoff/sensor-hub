import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl, FormControlLabel,
  InputLabel,
  MenuItem,
  Select, Switch,
  TextField
} from "@mui/material";
import { apiClient } from "../gen/client";
import {useEffect, useState} from "react";
import {useSensorContext} from "../hooks/useSensorContext.ts";
import {useSensorMeasurementTypes} from "../hooks/useMeasurementTypes.ts";
import { logger } from '../tools/logger';

interface CreateAlertDialogProps {
  open: boolean;
  onClose: () => void;
  onCreated: () => Promise<void>;
}

export default function CreateAlertDialog({open, onClose, onCreated}: CreateAlertDialogProps) {
  const [createAlertType, setCreateAlertType] = useState<'numeric_range' | 'status_based'>('numeric_range');
  const [createSensorId, setCreateSensorId] = useState<number>(0);
  const [createMeasurementTypeId, setCreateMeasurementTypeId] = useState<number>(0);
  const [createHighThreshold, setCreateHighThreshold] = useState<string>('');
  const [createLowThreshold, setCreateLowThreshold] = useState<string>('');
  const [createTriggerStatus, setCreateTriggerStatus] = useState<string>('');
  const [createRateLimit, setCreateRateLimit] = useState<string>('1');
  const [createRateLimitUnit, setCreateRateLimitUnit] = useState<'seconds' | 'minutes' | 'hours'>('hours');
  const [createEnabled, setCreateEnabled] = useState<boolean>(true);
  const { sensors } = useSensorContext();
  const { measurementTypes } = useSensorMeasurementTypes(createSensorId || null);

  useEffect(() => {
    void Promise.resolve().then(() => setCreateMeasurementTypeId(0));
  }, [createSensorId]);

  useEffect(() => {
    if (createMeasurementTypeId && measurementTypes.length > 0) {
      const mt = measurementTypes.find(m => m.id === createMeasurementTypeId);
      if (mt) {
        void Promise.resolve().then(() =>
          setCreateAlertType(mt.category === 'binary' ? 'status_based' : 'numeric_range'));
      }
    }
  }, [createMeasurementTypeId, measurementTypes]);

  const resetForm = () => {
    setCreateSensorId(0);
    setCreateMeasurementTypeId(0);
    setCreateAlertType('numeric_range');
    setCreateHighThreshold('');
    setCreateLowThreshold('');
    setCreateTriggerStatus('');
    setCreateRateLimit('1');
    setCreateRateLimitUnit('hours');
    setCreateEnabled(true);
  };

  const toSeconds = (value: number, unit: 'seconds' | 'minutes' | 'hours') => {
    if (unit === 'minutes') return value * 60;
    if (unit === 'hours') return value * 3600;
    return value;
  };

  const handleCreate = async () => {
    try {
      const body = {
        SensorID: createSensorId,
        MeasurementTypeID: createMeasurementTypeId,
        AlertType: createAlertType,
        RateLimitSeconds: toSeconds(parseInt(createRateLimit, 10), createRateLimitUnit),
        Enabled: createEnabled,
        ...(createAlertType === 'numeric_range'
          ? { HighThreshold: parseFloat(createHighThreshold), LowThreshold: parseFloat(createLowThreshold) }
          : { TriggerStatus: createTriggerStatus }),
      };
      await apiClient.POST('/alerts', { body: body as never });
      resetForm();
      onClose();
      await onCreated();
    } catch (e) {
      logger.error('Failed to create alert rule', e);
    }
  };

  const handleCancel = () => {
    resetForm();
    onClose();
  };

  const selectedMT = measurementTypes.find(m => m.id === createMeasurementTypeId);

  return (
    <Dialog open={open} onClose={handleCancel} maxWidth="sm" fullWidth>
      <DialogTitle>Create Alert Rule</DialogTitle>
      <DialogContent>
        <FormControl fullWidth sx={{ mt: 1 }}>
          <InputLabel id="create-sensor-label">Sensor</InputLabel>
          <Select
            labelId="create-sensor-label"
            value={createSensorId}
            label="Sensor"
            onChange={(e) => setCreateSensorId(Number(e.target.value))}
          >
            {sensors.map(s => (
              <MenuItem key={s.id} value={s.id}>{s.name}</MenuItem>
            ))}
          </Select>
        </FormControl>

        {createSensorId > 0 && (
          <FormControl fullWidth sx={{ mt: 2 }}>
            <InputLabel id="create-mt-label">Measurement Type</InputLabel>
            <Select
              labelId="create-mt-label"
              value={createMeasurementTypeId}
              label="Measurement Type"
              onChange={(e) => setCreateMeasurementTypeId(Number(e.target.value))}
            >
              {measurementTypes.map(mt => (
                <MenuItem key={mt.id} value={mt.id}>{mt.display_name} ({mt.unit || mt.category})</MenuItem>
              ))}
            </Select>
          </FormControl>
        )}

        {createMeasurementTypeId > 0 && (
          <>
            <FormControl fullWidth sx={{ mt: 2 }}>
              <InputLabel id="create-type-label">Alert Type</InputLabel>
              <Select
                labelId="create-type-label"
                value={createAlertType}
                label="Alert Type"
                onChange={(e) => setCreateAlertType(e.target.value as 'numeric_range' | 'status_based')}
              >
                {selectedMT?.category !== 'binary' && (
                  <MenuItem value="numeric_range">Numeric Range</MenuItem>
                )}
                <MenuItem value="status_based">Status Based</MenuItem>
              </Select>
            </FormControl>

            {createAlertType === 'numeric_range' ? (
              <>
                <TextField
                  fullWidth
                  label="High Threshold"
                  type="number"
                  value={createHighThreshold}
                  onChange={(e) => setCreateHighThreshold(e.target.value)}
                  sx={{ mt: 2 }}
                />
                <TextField
                  fullWidth
                  label="Low Threshold"
                  type="number"
                  value={createLowThreshold}
                  onChange={(e) => setCreateLowThreshold(e.target.value)}
                  sx={{ mt: 2 }}
                />
              </>
            ) : (
              <TextField
                fullWidth
                label="Trigger Status"
                value={createTriggerStatus}
                onChange={(e) => setCreateTriggerStatus(e.target.value)}
                sx={{ mt: 2 }}
                helperText="e.g., 'true', 'false', 'open', 'closed'"
              />
            )}

            <Box
              sx={{
                display: "flex",
                gap: 2,
                mt: 2
              }}>
              <TextField
                label="Rate Limit"
                type="number"
                value={createRateLimit}
                onChange={(e) => setCreateRateLimit(e.target.value)}
                sx={{ flex: 1 }}
              />
              <FormControl sx={{ minWidth: 120 }}>
                <InputLabel id="create-rate-unit-label">Unit</InputLabel>
                <Select
                  labelId="create-rate-unit-label"
                  value={createRateLimitUnit}
                  label="Unit"
                  onChange={(e) => setCreateRateLimitUnit(e.target.value as 'seconds' | 'minutes' | 'hours')}
                >
                  <MenuItem value="seconds">Seconds</MenuItem>
                  <MenuItem value="minutes">Minutes</MenuItem>
                  <MenuItem value="hours">Hours</MenuItem>
                </Select>
              </FormControl>
            </Box>

            <FormControlLabel
              control={
                <Switch
                  checked={createEnabled}
                  onChange={(e) => setCreateEnabled(e.target.checked)}
                />
              }
              label="Enabled"
              sx={{ mt: 2 }}
            />
          </>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleCancel}>Cancel</Button>
        <Button variant="contained" onClick={handleCreate} disabled={!createSensorId || !createMeasurementTypeId}>Create</Button>
      </DialogActions>
    </Dialog>
  );
}