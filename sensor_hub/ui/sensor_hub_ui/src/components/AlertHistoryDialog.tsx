import {Button, Dialog, DialogActions, DialogContent, DialogTitle, LinearProgress, List, ListItem, ListItemText, Typography} from "@mui/material";
import {useEffect, useState} from "react";
import type {AlertHistoryEntry, AlertRule} from "../gen/aliases";
import { apiClient } from "../gen/client";
import { logger } from '../tools/logger';

interface AlertHistoryDialogProps {
  open: boolean;
  onClose: () => void;
  selectedAlert: AlertRule | null;
}

export default function AlertHistoryDialog({open, onClose, selectedAlert}: AlertHistoryDialogProps) {
  const [historyLoading, setHistoryLoading] = useState(false);
  const [historyData, setHistoryData] = useState<AlertHistoryEntry[]>([]);

  useEffect(() => {
    if (!open || !selectedAlert) return;
    let cancelled = false;
    const fetchHistory = async () => {
      setHistoryLoading(true);
      try {
      const { data: history } = await apiClient.GET('/alerts/sensor/{sensorId}/history', {
          params: { path: { sensorId: selectedAlert.SensorID }, query: { limit: 50 } }
        });
        if (!cancelled) setHistoryData(history ?? []);
      } catch (e) {
        logger.error('Failed to load alert history', e);
        if (!cancelled) setHistoryData([]);
      } finally {
        if (!cancelled) setHistoryLoading(false);
      }
    };
    fetchHistory();
    return () => { cancelled = true; };
  }, [open, selectedAlert]);

  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>Alert History - {selectedAlert?.SensorName}</DialogTitle>
      <DialogContent>
        {historyLoading ? (
          <LinearProgress />
        ) : historyData.length === 0 ? (
          <Typography>No alert history found for this sensor.</Typography>
        ) : (
          <List disablePadding>
            {historyData.map((h, index) => (
              <ListItem key={h.id} disableGutters divider={index < historyData.length - 1}>
                <ListItemText
                  primary={`Value: ${h.reading_value}`}
                  secondary={`Type: ${h.alert_type} · Sent: ${new Date(h.sent_at).toLocaleString()}`}
                />
              </ListItem>
            ))}
          </List>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
}