import { useEffect, useState } from 'react';
import { Alert, Button, Chip, Menu, MenuItem } from '@mui/material';
import { apiClient } from '../gen/client';
import { unwrap } from '../api/unwrap';
import type { MQTTClient, MQTTClientCreated } from '../gen/aliases';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import { logger } from '../tools/logger';
import Card from '../ui/Card';
import DataTable from '../ui/DataTable';
import CreateMqttClientDialog from './CreateMqttClientDialog';
import MqttClientPasswordDialog from './MqttClientPasswordDialog';

export default function MqttClientsCard() {
  const [clients, setClients] = useState<MQTTClient[]>([]);
  const [menuAnchorEl, setMenuAnchorEl] = useState<null | HTMLElement>(null);
  const [selectedRow, setSelectedRow] = useState<MQTTClient | null>(null);
  const [openCreateDialog, setOpenCreateDialog] = useState(false);
  const [shownPassword, setShownPassword] = useState<MQTTClientCreated | null>(null);
  const [error, setError] = useState('');
  const { user } = useAuth();

  const load = () =>
    apiClient.GET('/mqtt/clients')
      .then(({ data }) => setClients(data ?? []))
      .catch((e) => logger.error(e));

  useEffect(() => { void load(); }, []);

  const handleRowClick = (row: MQTTClient, anchor: HTMLElement) => {
    setSelectedRow(row);
    setMenuAnchorEl(anchor);
  };

  const closeMenu = () => { setMenuAnchorEl(null); };

  const run = async (action: string, request: () => Promise<unknown>) => {
    closeMenu();
    setError('');
    try {
      await request();
    } catch (e) {
      setError(e instanceof Error ? e.message : `Failed to ${action}`);
      logger.error(`Failed to ${action}`, e);
    }
    await load();
  };

  const handleRotate = (row: MQTTClient) => run('rotate the password', async () => {
    setShownPassword(await unwrap(apiClient.POST('/mqtt/clients/{id}/rotate-password', { params: { path: { id: row.id } } })));
  });

  const handleToggleEnabled = (row: MQTTClient) => run('update the client', () =>
    unwrap(apiClient.PUT('/mqtt/clients/{id}', {
      params: { path: { id: row.id } },
      body: { name: row.name, topic_prefix: row.topic_prefix, enabled: !row.enabled },
    })));

  const handleDelete = (row: MQTTClient) => run('delete the client', async () => {
    const { response } = await apiClient.DELETE('/mqtt/clients/{id}', { params: { path: { id: row.id } } });
    if (!response.ok) throw new Error(`${response.status} ${response.statusText}`);
  });

  const handleCreated = async (created: MQTTClientCreated) => {
    setShownPassword(created);
    await load();
  };

  const canManage = user && hasPerm(user, 'manage_mqtt');

  return (
    <>
      <Card
        title="MQTT Clients"
        actions={<Button variant="contained" onClick={() => setOpenCreateDialog(true)} disabled={!canManage}>Add Client</Button>}
      >
        {error && <Alert severity="error" onClose={() => setError('')}>{error}</Alert>}
        <DataTable
          rows={clients}
          onRowClick={canManage ? handleRowClick : undefined}
          columns={[
            { field: 'name', headerName: 'Name', flex: 1, minWidth: 140, compact: 'title' },
            { field: 'topic_prefix', headerName: 'Topic prefix', flex: 1, minWidth: 140, compact: 'meta' },
            {
              field: 'enabled',
              headerName: 'Status',
              width: 110,
              compact: 'status',
              statusOf: (row) => (row.enabled ? 'ok' : 'unknown'),
              valueFormatter: (value: boolean) => (value ? 'Enabled' : 'Disabled'),
              renderCell: ({ row, formattedValue }) => (
                <Chip label={formattedValue} color={row.enabled ? 'success' : 'default'} size="small" />
              ),
            },
            {
              field: 'connected',
              headerName: 'Connected',
              width: 130,
              compact: 'meta',
              valueFormatter: (value: boolean) => (value ? 'Connected' : 'Not connected'),
            },
            {
              field: 'last_connected_at',
              headerName: 'Last connected',
              width: 180,
              compact: 'hidden',
              valueFormatter: (value: string | null) => (value ? new Date(value).toLocaleString() : 'Never'),
            },
          ]}
        />

        {canManage && (
          <Menu anchorEl={menuAnchorEl} open={Boolean(menuAnchorEl)} onClose={closeMenu}>
            <MenuItem onClick={() => selectedRow && handleRotate(selectedRow)}>Rotate password</MenuItem>
            <MenuItem onClick={() => selectedRow && handleToggleEnabled(selectedRow)}>
              {selectedRow?.enabled ? 'Disable' : 'Enable'}
            </MenuItem>
            <MenuItem onClick={() => selectedRow && handleDelete(selectedRow)} sx={{ color: 'error.main' }}>Delete</MenuItem>
          </Menu>
        )}
      </Card>
      <CreateMqttClientDialog open={openCreateDialog} onClose={() => setOpenCreateDialog(false)} onCreated={handleCreated} />
      <MqttClientPasswordDialog client={shownPassword} onClose={() => setShownPassword(null)} />
    </>
  );
}
