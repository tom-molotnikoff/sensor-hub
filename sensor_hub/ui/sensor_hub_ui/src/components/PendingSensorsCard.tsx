import { useEffect, useState, useCallback } from 'react';
import { DataGrid } from '@mui/x-data-grid';
import type { GridColDef } from '@mui/x-data-grid';
import { Button, Box, Chip, Stack, Typography } from '@mui/material';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import CancelIcon from '@mui/icons-material/Cancel';
import { apiClient } from '../gen/client';
import type { Sensor } from '../gen/aliases';
import LayoutCard from '../tools/LayoutCard';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import { useIsMobile } from '../hooks/useMobile';
import { logger } from '../tools/logger';
import { TypographyH2 } from '../tools/Typography';
import { getDeviceMetadataSummary } from '../tools/deviceMetadata';

export default function PendingSensorsCard() {
  const [pending, setPending] = useState<Sensor[]>([]);
  const [dismissed, setDismissed] = useState<Sensor[]>([]);
  const [showDismissed, setShowDismissed] = useState(false);
  const { user } = useAuth();
  const isMobile = useIsMobile();

  const load = useCallback(async () => {
    try {
      const [pRes, dRes] = await Promise.all([
        apiClient.GET('/sensors/status/{status}', { params: { path: { status: 'pending' } } }),
        apiClient.GET('/sensors/status/{status}', { params: { path: { status: 'dismissed' } } }),
      ]);
      setPending(pRes.data || []);
      setDismissed(dRes.data || []);
    } catch (e) { logger.error(e); }
  }, []);

  useEffect(() => { void Promise.resolve().then(load); }, [load]);

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

  const canManage = user && hasPerm(user, 'manage_sensors');

  const pendingColumns: GridColDef[] = [
    { field: 'id', headerName: 'ID', width: 60 },
    {
      field: 'name',
      headerName: 'Device Name',
      flex: 1,
      renderCell: (params) => {
        const metadataSummary = getDeviceMetadataSummary((params.row as Sensor).metadata);

        return (
          <Stack
            spacing={0}
            sx={{
              justifyContent: "center",
              py: 1
            }}>
            <Typography variant="body2">{params.row.name}</Typography>
            {metadataSummary && (
              <Typography variant="caption" sx={{
                color: "text.secondary"
              }}>
                {metadataSummary}
              </Typography>
            )}
          </Stack>
        );
      },
    },
    { field: 'sensor_driver', headerName: 'Driver', width: 160 },
    {
      field: 'actions', headerName: 'Actions', width: 260, sortable: false,
      renderCell: (params) => (
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            height: "100%"
          }}>
          <Stack direction="row" spacing={1}>
            <Button size="small" variant="contained" color="success" startIcon={<CheckCircleIcon />}
              onClick={(e) => { e.stopPropagation(); handleApprove(params.row.id); }}
              disabled={!canManage}>
              Approve
            </Button>
            <Button size="small" variant="outlined" color="warning" startIcon={<CancelIcon />}
              onClick={(e) => { e.stopPropagation(); handleDismiss(params.row.id); }}
              disabled={!canManage}>
              Dismiss
            </Button>
          </Stack>
        </Box>
      ),
    },
  ];

  const dismissedColumns: GridColDef[] = [
    { field: 'id', headerName: 'ID', width: 60 },
    { field: 'name', headerName: 'Device Name', flex: 1 },
    { field: 'sensor_driver', headerName: 'Driver', width: 160 },
    {
      field: 'actions', headerName: 'Actions', width: 140, sortable: false,
      renderCell: (params) => (
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            height: "100%"
          }}>
          <Button size="small" variant="outlined" startIcon={<CheckCircleIcon />}
            onClick={(e) => { e.stopPropagation(); handleApprove(params.row.id); }}
            disabled={!canManage}>
            Restore
          </Button>
        </Box>
      ),
    },
  ];

  const mobileHiddenFields = ['id', 'sensor_driver'];
  const filteredPendingCols = isMobile ? pendingColumns.filter(c => !mobileHiddenFields.includes(c.field)) : pendingColumns;
  const filteredDismissedCols = isMobile ? dismissedColumns.filter(c => !mobileHiddenFields.includes(c.field)) : dismissedColumns;

  return (
    <LayoutCard variant="secondary" changes={{ alignItems: 'stretch', height: '100%', width: '100%' }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 2,
          mb: 2,
          width: '100%'
        }}>
        <TypographyH2>
          Pending Sensors
          {pending.length > 0 && (
            <Chip label={pending.length} color="warning" size="small" sx={{ ml: 1 }} />
          )}
        </TypographyH2>
      </Box>
      {pending.length === 0 ? (
        <Typography
          variant="body2"
          sx={{
            color: "text.secondary",
            py: 2
          }}>
          No pending sensors. When MQTT devices are auto-discovered, they will appear here for approval.
        </Typography>
      ) : (
        <div style={{ height: 300, width: '100%' }}>
          <DataGrid rows={pending} columns={filteredPendingCols} pageSizeOptions={[5, 10]}
            initialState={{ pagination: { paginationModel: { pageSize: 5 } } }}
            disableRowSelectionOnClick />
        </div>
      )}
      {dismissed.length > 0 && (
        <Box sx={{ mt: 2 }}>
          <Button size="small" variant="text" onClick={() => setShowDismissed(!showDismissed)}>
            {showDismissed ? 'Hide' : 'Show'} dismissed sensors ({dismissed.length})
          </Button>
          {showDismissed && (
            <div style={{ height: 250, width: '100%', marginTop: 8 }}>
              <DataGrid rows={dismissed} columns={filteredDismissedCols} pageSizeOptions={[5, 10]}
                initialState={{ pagination: { paginationModel: { pageSize: 5 } } }}
                disableRowSelectionOnClick />
            </div>
          )}
        </Box>
      )}
    </LayoutCard>
  );
}
