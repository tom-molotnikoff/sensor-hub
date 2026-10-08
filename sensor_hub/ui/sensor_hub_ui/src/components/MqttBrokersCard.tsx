import { useEffect, useState } from 'react';
import { Alert, Button, Menu, MenuItem, Chip, Tooltip } from '@mui/material';
import { apiClient } from '../gen/client';
import type { MQTTBroker } from '../gen/aliases';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import BrokerDialog from './BrokerDialog';
import { logger } from '../tools/logger';
import { NEEDS_REENTRY_LABEL, NEEDS_REENTRY_TEXT, needsReentry } from '../tools/secretStatus';
import Card from '../ui/Card';
import DataTable from '../ui/DataTable';
import Stack from '../ui/Stack';

// How the hub reaches the broker. The embedded broker is in-process, so its
// traffic never crosses a network; an external one is either over TLS or not.
function connectionOf(broker: MQTTBroker): string {
  if (broker.type === 'embedded') return 'In-process';
  return broker.tls ? 'TLS' : 'Unencrypted';
}

export default function MqttBrokersCard() {
  const [brokers, setBrokers] = useState<MQTTBroker[]>([]);
  const [menuAnchorEl, setMenuAnchorEl] = useState<null | HTMLElement>(null);
  const [selectedRow, setSelectedRow] = useState<MQTTBroker | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<MQTTBroker | null>(null);
  const { user } = useAuth();

  const load = () =>
    apiClient.GET('/mqtt/brokers')
      .then(({ data: b }) => setBrokers((b as MQTTBroker[] | null) ?? []))
      .catch((e) => logger.error(e));

  useEffect(() => { void load(); }, []);

  const handleRowClick = (row: MQTTBroker, anchor: HTMLElement) => {
    setSelectedRow(row);
    setMenuAnchorEl(anchor);
  };

  const closeMenu = () => { setMenuAnchorEl(null); };

  const openDialog = (broker: MQTTBroker | null) => {
    closeMenu();
    setEditing(broker);
    setDialogOpen(true);
  };

  // Writes the broker back with every setting it has. The hub never sends the
  // password, so the body has none, which keeps the stored one.
  const handleToggleEnabled = async () => {
    if (!selectedRow) return;
    closeMenu();
    const { id, created_at: _createdAt, updated_at: _updatedAt, password_status: _passwordStatus, ...settings } = selectedRow;
    try {
      await apiClient.PUT('/mqtt/brokers/{id}', {
        params: { path: { id } },
        body: { ...settings, enabled: !selectedRow.enabled } as never,
      });
      await load();
    } catch (e) { logger.error('Failed to toggle broker', e); }
  };

  const handleDelete = async () => {
    if (!selectedRow) return;
    closeMenu();
    try {
      await apiClient.DELETE('/mqtt/brokers/{id}', { params: { path: { id: selectedRow.id } } });
      await load();
    } catch (e) { logger.error('Failed to delete broker', e); }
  };

  const canManage = user && hasPerm(user, 'manage_mqtt');
  const brokersNeedingReentry = brokers.filter(needsReentry);

  return (
    <>
      <Card
        title="MQTT Brokers"
        actions={<Button variant="contained" onClick={() => openDialog(null)} disabled={!canManage}>Add Broker</Button>}
      >
        <Stack>
          {brokersNeedingReentry.map((broker) => (
            <Alert key={broker.id} severity="error">
              {broker.name}: {NEEDS_REENTRY_TEXT}
            </Alert>
          ))}
          <DataTable
            rows={brokers}
            onRowClick={canManage ? handleRowClick : undefined}
            columns={[
              { field: 'id', headerName: 'ID', width: 60, compact: 'hidden' },
              { field: 'name', headerName: 'Name', flex: 1, minWidth: 140, compact: 'title' },
              { field: 'type', headerName: 'Type', width: 100, compact: 'hidden' },
              { field: 'host', headerName: 'Host', flex: 1, minWidth: 140, compact: 'meta' },
              { field: 'port', headerName: 'Port', width: 80, compact: 'hidden' },
              {
                field: 'tls',
                headerName: 'Connection',
                width: 130,
                compact: 'meta',
                valueGetter: (_value: unknown, row: MQTTBroker) => connectionOf(row),
                renderCell: ({ value }) => (value === 'Unencrypted'
                  ? <Chip label={value} color="warning" variant="outlined" size="small" />
                  : value),
              },
              {
                field: 'enabled',
                headerName: 'Status',
                width: 140,
                compact: 'status',
                statusOf: (row) => {
                  if (needsReentry(row)) return 'bad';
                  return row.enabled ? 'ok' : 'unknown';
                },
                valueFormatter: (value: boolean, row: MQTTBroker) => {
                  if (needsReentry(row)) return NEEDS_REENTRY_LABEL;
                  return value ? 'Enabled' : 'Disabled';
                },
                renderCell: ({ row, formattedValue }) => (needsReentry(row)
                  ? (
                    <Tooltip title={NEEDS_REENTRY_TEXT}>
                      <Chip label={formattedValue} color="error" size="small" />
                    </Tooltip>
                  )
                  : <Chip label={formattedValue} color={row.enabled ? 'success' : 'default'} size="small" />),
              },
            ]}
          />
        </Stack>

        {canManage && (
          <Menu anchorEl={menuAnchorEl} open={Boolean(menuAnchorEl)} onClose={closeMenu}>
            <MenuItem onClick={() => openDialog(selectedRow)}>Edit</MenuItem>
            <MenuItem onClick={handleToggleEnabled}>
              {selectedRow?.enabled ? 'Disable' : 'Enable'}
            </MenuItem>
            <MenuItem onClick={handleDelete} sx={{ color: 'error.main' }}>Delete</MenuItem>
          </Menu>
        )}
      </Card>
      <BrokerDialog open={dialogOpen} broker={editing} onClose={() => setDialogOpen(false)} onSaved={load} />
    </>
  );
}
