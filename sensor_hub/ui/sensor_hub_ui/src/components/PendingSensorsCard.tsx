import { useEffect, useState, useCallback } from 'react';
import { Button, Chip, Typography } from '@mui/material';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import CancelIcon from '@mui/icons-material/Cancel';
import { apiClient } from '../gen/client';
import type { Sensor } from '../gen/aliases';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import { logger } from '../tools/logger';
import { getDeviceMetadataSummary } from '../tools/deviceMetadata';
import Card from '../ui/Card';
import DataTable, { type DataTableColumn } from '../ui/DataTable';
import Stack from '../ui/Stack';

const columns = [
  { field: 'id', headerName: 'ID', width: 60, compact: 'hidden' },
  { field: 'name', headerName: 'Device Name', flex: 1, minWidth: 160, compact: 'title' },
  {
    field: 'device',
    headerName: 'Device',
    flex: 1,
    minWidth: 140,
    compact: 'meta',
    valueGetter: (_value: never, row: Sensor) => getDeviceMetadataSummary(row.metadata) ?? '',
  },
  { field: 'sensor_driver', headerName: 'Driver', width: 160, compact: 'meta' },
] as const satisfies readonly DataTableColumn<Sensor>[];

export default function PendingSensorsCard() {
  const [pending, setPending] = useState<Sensor[]>([]);
  const [dismissed, setDismissed] = useState<Sensor[]>([]);
  const [showDismissed, setShowDismissed] = useState(false);
  const { user } = useAuth();

  const load = useCallback(() =>
    Promise.all([
      apiClient.GET('/sensors/status/{status}', { params: { path: { status: 'pending' } } }),
      apiClient.GET('/sensors/status/{status}', { params: { path: { status: 'dismissed' } } }),
    ])
      .then(([pRes, dRes]) => {
        setPending(pRes.data || []);
        setDismissed(dRes.data || []);
      })
      .catch((e) => logger.error(e)),
  []);

  useEffect(() => { void load(); }, [load]);

  const handleApprove = async (id: number) => {
    try {
      await apiClient.POST('/sensors/approve/{id}', { params: { path: { id } } });
      await load();
    } catch (e) { logger.error('Failed to approve sensor', e); }
  };

  const handleDismiss = async (id: number) => {
    try {
      await apiClient.POST('/sensors/dismiss/{id}', { params: { path: { id } } });
      await load();
    } catch (e) { logger.error('Failed to dismiss sensor', e); }
  };

  const cannotManage = !(user && hasPerm(user, 'manage_sensors'));

  return (
    <Card
      title="Pending Sensors"
      actions={pending.length > 0 && <Chip label={pending.length} color="warning" size="small" />}
    >
      <Stack>
        {pending.length === 0 ? (
          <Typography variant="body2" sx={{ color: "text.secondary" }}>
            No pending sensors. When MQTT devices are auto-discovered, they will appear here for approval.
          </Typography>
        ) : (
          <DataTable
            rows={pending}
            columns={columns}
            rowActions={(sensor) => [
              { label: 'Approve', icon: <CheckCircleIcon />, color: 'success', disabled: cannotManage, onClick: () => handleApprove(sensor.id) },
              { label: 'Dismiss', icon: <CancelIcon />, color: 'warning', disabled: cannotManage, onClick: () => handleDismiss(sensor.id) },
            ]}
          />
        )}
        {dismissed.length > 0 && (
          <div>
            <Button size="small" variant="text" onClick={() => setShowDismissed(!showDismissed)}>
              {showDismissed ? 'Hide' : 'Show'} dismissed sensors ({dismissed.length})
            </Button>
          </div>
        )}
        {dismissed.length > 0 && showDismissed && (
          <DataTable
            rows={dismissed}
            columns={columns}
            rowActions={(sensor) => [
              { label: 'Restore', icon: <CheckCircleIcon />, color: 'primary', disabled: cannotManage, onClick: () => handleApprove(sensor.id) },
            ]}
          />
        )}
      </Stack>
    </Card>
  );
}
