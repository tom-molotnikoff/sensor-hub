import {Button, Dialog, DialogActions, DialogContent, DialogTitle} from "@mui/material";
import type {AlertRule} from "../gen/aliases";
import { apiClient } from "../gen/client";
import { logger } from '../tools/logger';

interface DeleteAlertDialogProps {
  open: boolean;
  onClose: () => void;
  onDeleted: () => Promise<void>;
  selectedAlert: AlertRule | null;
}

export default function DeleteAlertDialog({open, onClose, onDeleted, selectedAlert}: DeleteAlertDialogProps) {
  const confirmDelete = async () => {
    if (!selectedAlert) return;
    try {
      await apiClient.DELETE('/alerts/{id}', { params: { path: { id: selectedAlert.id } } });
      onClose();
      await onDeleted();
    } catch (e) {
      logger.error('Failed to delete alert rule', e);
    }
  };

  return (
    <Dialog open={open} onClose={onClose}>
      <DialogTitle>Delete Alert Rule</DialogTitle>
      <DialogContent>
        Are you sure you want to delete the alert rule for sensor{' '}
        <strong>{selectedAlert?.sensor_name}</strong>?
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" color="error" onClick={confirmDelete}>
          Delete
        </Button>
      </DialogActions>
    </Dialog>
  );
}