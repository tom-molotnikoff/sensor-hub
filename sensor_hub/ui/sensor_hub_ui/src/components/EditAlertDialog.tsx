import {useState} from "react";
import type {AlertRule} from "../gen/aliases";
import { apiClient } from "../gen/client";
import {
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
import PageGrid from '../ui/PageGrid';
import Stack from '../ui/Stack';

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

  // Re-seed the form whenever the dialog opens for an alert (adjust-during-render).
  const [prevOpen, setPrevOpen] = useState(open);
  const [prevAlert, setPrevAlert] = useState(selectedAlert);
  if (prevOpen !== open || prevAlert !== selectedAlert) {
    setPrevOpen(open);
    setPrevAlert(selectedAlert);
    if (open && selectedAlert) {
      setEditAlertType(selectedAlert.alert_type);
      setEditHighThreshold(selectedAlert.high_threshold?.toString() || '');
      setEditLowThreshold(selectedAlert.low_threshold?.toString() || '');
      setEditTriggerStatus(selectedAlert.trigger_status || '');
      const { value, unit } = fromSeconds(selectedAlert.rate_limit_seconds);
      setEditRateLimit(value.toString());
      setEditRateLimitUnit(unit);
      setEditEnabled(selectedAlert.enabled);
    }
  }

  const handleEdit = async () => {
    if (!selectedAlert) return;
    try {
      const body = {
        alert_type: editAlertType,
        rate_limit_seconds: toSeconds(parseInt(editRateLimit, 10), editRateLimitUnit),
        enabled: editEnabled,
        ...(editAlertType === 'numeric_range'
          ? { high_threshold: parseFloat(editHighThreshold), LowThreshold: parseFloat(editLowThreshold) }
          : { trigger_status: editTriggerStatus }),
      };
      await apiClient.PUT('/alerts/{id}', { params: { path: { id: selectedAlert.id } }, body: body as never });
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
        <Stack>
          <TextField
            fullWidth
            label="Sensor"
            value={selectedAlert?.sensor_name || ''}
            disabled
          />

          <FormControl fullWidth>
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
              />
              <TextField
                fullWidth
                label="Low Threshold"
                type="number"
                value={editLowThreshold}
                onChange={(e) => setEditLowThreshold(e.target.value)}
              />
            </>
          ) : (
            <TextField
              fullWidth
              label="Trigger Status"
              value={editTriggerStatus}
              onChange={(e) => setEditTriggerStatus(e.target.value)}
              helperText="e.g., 'true', 'false', 'open', 'closed'"
            />
          )}

          <PageGrid>
            <PageGrid.Item span={{ wide: 8 }}>
              <TextField
                label="Rate Limit"
                type="number"
                value={editRateLimit}
                onChange={(e) => setEditRateLimit(e.target.value)}
                fullWidth
              />
            </PageGrid.Item>
            <PageGrid.Item span={{ wide: 4 }}>
              <FormControl fullWidth>
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
            </PageGrid.Item>
          </PageGrid>

          <FormControlLabel
            control={
              <Switch
                checked={editEnabled}
                onChange={(e) => setEditEnabled(e.target.checked)}
              />
            }
            label="Enabled"
          />
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" onClick={handleEdit}>Save</Button>
      </DialogActions>
    </Dialog>
  );
}