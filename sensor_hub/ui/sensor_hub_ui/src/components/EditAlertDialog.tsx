import {useEffect, useState} from "react";
import type {AlertRule} from "../gen/aliases";
import { apiClient } from "../gen/client";
import {
  Box,
  Button,
  Dialog, DialogActions,
  DialogContent, DialogTitle,
  FormControl,
  FormControlLabel,
  InputLabel,
  MenuItem,
  Select, Switch,
  TextField
} from "@mui/material";
import { logger } from '../tools/logger';

type RateLimitUnit = 'seconds' | 'minutes' | 'hours';

const fromSeconds = (totalSeconds: number): { value: number; unit: RateLimitUnit } => {
  if (totalSeconds > 0 && totalSeconds % 3600 === 0) return { value: totalSeconds / 3600, unit: 'hours' };
  if (totalSeconds > 0 && totalSeconds % 60 === 0) return { value: totalSeconds / 60, unit: 'minutes' };
  return { value: totalSeconds, unit: 'seconds' };
};

const toSeconds = (value: number, unit: RateLimitUnit) => {
  if (unit === 'minutes') return value * 60;
  if (unit === 'hours') return value * 3600;
  return value;
};

interface EditAlertDialogProps {
  open: boolean;
  onClose: () => void;
  onSaved: () => Promise<void>;
  selectedAlert: AlertRule | null;
}

export default function EditAlertDialog({open, onClose, onSaved, selectedAlert}: EditAlertDialogProps) {
  const [editAlertType, setEditAlertType] = useState<'numeric_range' | 'status_based'>('numeric_range');
  const [editHighThreshold, setEditHighThreshold] = useState<string>('');
  const [editLowThreshold, setEditLowThreshold] = useState<string>('');
  const [editTriggerStatus, setEditTriggerStatus] = useState<string>('');
  const [editRateLimit, setEditRateLimit] = useState<string>('1');
  const [editRateLimitUnit, setEditRateLimitUnit] = useState<RateLimitUnit>('hours');
  const [editEnabled, setEditEnabled] = useState<boolean>(true);

  useEffect(() => {
    if (open && selectedAlert) {
      void Promise.resolve().then(() => {
        setEditAlertType(selectedAlert.AlertType);
        setEditHighThreshold(selectedAlert.HighThreshold?.toString() || '');
        setEditLowThreshold(selectedAlert.LowThreshold?.toString() || '');
        setEditTriggerStatus(selectedAlert.TriggerStatus || '');
        const { value, unit } = fromSeconds(selectedAlert.RateLimitSeconds);
        setEditRateLimit(value.toString());
        setEditRateLimitUnit(unit);
        setEditEnabled(selectedAlert.Enabled);
      });
    }
  }, [open, selectedAlert]);

  const handleEdit = async () => {
    if (!selectedAlert) return;
    try {
      const body = {
        AlertType: editAlertType,
        RateLimitSeconds: toSeconds(parseInt(editRateLimit, 10), editRateLimitUnit),
        Enabled: editEnabled,
        ...(editAlertType === 'numeric_range'
          ? { HighThreshold: parseFloat(editHighThreshold), LowThreshold: parseFloat(editLowThreshold) }
          : { TriggerStatus: editTriggerStatus }),
      };
      await apiClient.PUT('/alerts/{id}', { params: { path: { id: selectedAlert.ID } }, body: body as never });
      onClose();
      await onSaved();
    } catch (e) {
      logger.error('Failed to update alert rule', e);
    }
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Edit Alert Rule</DialogTitle>
      <DialogContent>
        <TextField
          fullWidth
          label="Sensor"
          value={selectedAlert?.SensorName || ''}
          disabled
          sx={{ mt: 1 }}
        />

        <FormControl fullWidth sx={{ mt: 2 }}>
          <InputLabel id="edit-type-label">Alert Type</InputLabel>
          <Select
            labelId="edit-type-label"
            value={editAlertType}
            label="Alert Type"
            onChange={(e) => setEditAlertType(e.target.value as 'numeric_range' | 'status_based')}
          >
            <MenuItem value="numeric_range">Numeric Range</MenuItem>
            <MenuItem value="status_based">Status Based</MenuItem>
          </Select>
        </FormControl>

        {editAlertType === 'numeric_range' ? (
          <>
            <TextField
              fullWidth
              label="High Threshold"
              type="number"
              value={editHighThreshold}
              onChange={(e) => setEditHighThreshold(e.target.value)}
              sx={{ mt: 2 }}
            />
            <TextField
              fullWidth
              label="Low Threshold"
              type="number"
              value={editLowThreshold}
              onChange={(e) => setEditLowThreshold(e.target.value)}
              sx={{ mt: 2 }}
            />
          </>
        ) : (
          <TextField
            fullWidth
            label="Trigger Status"
            value={editTriggerStatus}
            onChange={(e) => setEditTriggerStatus(e.target.value)}
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
            value={editRateLimit}
            onChange={(e) => setEditRateLimit(e.target.value)}
            sx={{ flex: 1 }}
          />
          <FormControl sx={{ minWidth: 120 }}>
            <InputLabel id="edit-rate-unit-label">Unit</InputLabel>
            <Select
              labelId="edit-rate-unit-label"
              value={editRateLimitUnit}
              label="Unit"
              onChange={(e) => setEditRateLimitUnit(e.target.value as RateLimitUnit)}
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
              checked={editEnabled}
              onChange={(e) => setEditEnabled(e.target.checked)}
            />
          }
          label="Enabled"
          sx={{ mt: 2 }}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" onClick={handleEdit}>Save</Button>
      </DialogActions>
    </Dialog>
  );
}