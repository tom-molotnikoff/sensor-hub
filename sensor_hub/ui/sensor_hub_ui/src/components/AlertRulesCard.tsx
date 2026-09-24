import { useEffect, useState } from 'react';
import { Button, Chip, Menu, MenuItem } from '@mui/material';
import NotificationsNoneOutlinedIcon from '@mui/icons-material/NotificationsNoneOutlined';
import { apiClient } from '../gen/client';
import type { AlertRule } from '../gen/aliases';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import AlertHistoryDialog from './AlertHistoryDialog';
import DeleteAlertDialog from './DeleteAlertDialog';
import EditAlertDialog from './EditAlertDialog';
import CreateAlertDialog from './CreateAlertDialog';
import Card from '../ui/Card';
import DataTable from '../ui/DataTable';
import EmptyState from '../ui/EmptyState';
import { logger } from '../tools/logger';

const alertTypeLabels: Record<string, string> = {
  numeric_range: 'Numeric range',
  status_based: 'Status based',
};

function formatRateLimit(seconds: number): string {
  if (seconds === 0) return 'None';
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`;
  return `${Math.round(seconds / 3600)}h`;
}

const orDash = (value: unknown) => (value === null || value === undefined || value === '' ? '-' : String(value));

export default function AlertRulesCard() {
  const [alertRules, setAlertRules] = useState<AlertRule[]>([]);
  const [menuAnchorEl, setMenuAnchorEl] = useState<null | HTMLElement>(null);
  const [selectedRow, setSelectedRow] = useState<AlertRule | null>(null);
  const [openEditDialog, setOpenEditDialog] = useState(false);
  const [openHistoryDialog, setOpenHistoryDialog] = useState(false);
  const [openDeleteDialog, setOpenDeleteDialog] = useState(false);
  const [openCreateDialog, setOpenCreateDialog] = useState(false);

  const { user } = useAuth();

  const load = () =>
    apiClient.GET('/alerts')
      .then(({ data }) => setAlertRules(data ?? []))
      .catch((e) => logger.error('Failed to load alert rules', e));

  useEffect(() => { void load(); }, []);

  const handleRowClick = (row: AlertRule, anchor: HTMLElement) => {
    setSelectedRow(row);
    setMenuAnchorEl(anchor);
  };

  const closeMenu = () => { setMenuAnchorEl(null); };

  const fieldsDisabled = !user || !hasPerm(user, "manage_alerts");
  const openCreate = () => setOpenCreateDialog(true);

  return (
    <>
      <Card
        title="Alert Rules"
        actions={
          <Button variant="contained" disabled={fieldsDisabled} onClick={openCreate}>
            Create Alert Rule
          </Button>
        }
      >
        {alertRules.length === 0 ? (
          <EmptyState
            icon={<NotificationsNoneOutlinedIcon fontSize="large" />}
            title="No alert rules configured"
            description="Create an alert rule to get notified when sensor readings go out of range."
            actionLabel={fieldsDisabled ? undefined : "Create Alert Rule"}
            onAction={fieldsDisabled ? undefined : openCreate}
            size="lg"
          />
        ) : (
          <DataTable
            rows={alertRules}
            onRowClick={handleRowClick}
            columns={[
              { field: 'sensor_name', headerName: 'Sensor', flex: 1, minWidth: 140, compact: 'title' },
              { field: 'measurement_type', headerName: 'Measurement', width: 130, compact: 'meta' },
              {
                field: 'alert_type',
                headerName: 'Alert Type',
                width: 150,
                compact: 'meta',
                valueFormatter: (value: string) => alertTypeLabels[value] ?? value,
              },
              { field: 'high_threshold', headerName: 'High', width: 80, compact: 'hidden', valueFormatter: orDash },
              { field: 'low_threshold', headerName: 'Low', width: 80, compact: 'hidden', valueFormatter: orDash },
              { field: 'trigger_status', headerName: 'Status', width: 100, compact: 'hidden', valueFormatter: orDash },
              {
                field: 'rate_limit_seconds',
                headerName: 'Rate Limit',
                width: 130,
                compact: 'hidden',
                valueFormatter: (value: number) => formatRateLimit(value),
              },
              {
                field: 'enabled',
                headerName: 'Enabled',
                width: 110,
                compact: 'status',
                statusOf: (row) => (row.enabled ? 'ok' : 'unknown'),
                valueFormatter: (value: boolean) => (value ? 'Enabled' : 'Disabled'),
                renderCell: ({ row, formattedValue }) => (
                  <Chip label={formattedValue} color={row.enabled ? 'success' : 'default'} size="small" />
                ),
              },
              {
                field: 'last_alert_sent_at',
                headerName: 'Last Alert Sent',
                width: 180,
                compact: 'hidden',
                valueFormatter: (value: string | null | undefined) => (value ? new Date(value).toLocaleString() : 'Never'),
              },
            ]}
          />
        )}

        {user && hasPerm(user, "view_alerts") && (
          <Menu anchorEl={menuAnchorEl} open={Boolean(menuAnchorEl)} onClose={closeMenu}>
            <MenuItem disabled={fieldsDisabled} onClick={() => { closeMenu(); setOpenEditDialog(true); }}>Edit</MenuItem>
            <MenuItem disabled={fieldsDisabled} onClick={() => { closeMenu(); setOpenDeleteDialog(true); }}>Delete</MenuItem>
            <MenuItem onClick={() => { closeMenu(); setOpenHistoryDialog(true); }}>View History</MenuItem>
          </Menu>
        )}
      </Card>
      <CreateAlertDialog open={openCreateDialog} onClose={() => setOpenCreateDialog(false)} onCreated={load} />
      <EditAlertDialog open={openEditDialog} onClose={() => setOpenEditDialog(false)} onSaved={load} selectedAlert={selectedRow} />
      <DeleteAlertDialog open={openDeleteDialog} onClose={() => setOpenDeleteDialog(false)} onDeleted={load} selectedAlert={selectedRow} />
      <AlertHistoryDialog open={openHistoryDialog} onClose={() => setOpenHistoryDialog(false)} selectedAlert={selectedRow} />
    </>
  );
}
