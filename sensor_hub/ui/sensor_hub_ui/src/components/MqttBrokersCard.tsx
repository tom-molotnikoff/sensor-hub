import { useEffect, useState } from 'react';
import { Button, Menu, MenuItem, Chip } from '@mui/material';
import { apiClient } from '../gen/client';
import type { MQTTBroker } from '../gen/aliases';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import CreateBrokerDialog from './CreateBrokerDialog';
import { logger } from '../tools/logger';
import Card from '../ui/Card';
import DataTable from '../ui/DataTable';

export default function MqttBrokersCard() {
  const [brokers, setBrokers] = useState<MQTTBroker[]>([]);
  const [menuAnchorEl, setMenuAnchorEl] = useState<null | HTMLElement>(null);
  const [selectedRow, setSelectedRow] = useState<MQTTBroker | null>(null);
  const [openCreateDialog, setOpenCreateDialog] = useState(false);
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

  const handleToggleEnabled = async () => {
    if (!selectedRow) return;
    closeMenu();
    try {
      await apiClient.PUT('/mqtt/brokers/{id}', {
        params: { path: { id: selectedRow.id } },
        body: { name: selectedRow.name, type: selectedRow.type, host: selectedRow.host, port: selectedRow.port, enabled: !selectedRow.enabled } as never,
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

  return (
    <>
      <Card
        title="MQTT Brokers"
        actions={<Button variant="contained" onClick={() => setOpenCreateDialog(true)} disabled={!canManage}>Add Broker</Button>}
      >
        <DataTable
          rows={brokers}
          onRowClick={canManage ? handleRowClick : undefined}
          columns={[
            { field: 'id', headerName: 'ID', width: 60, compact: 'hidden' },
            { field: 'name', headerName: 'Name', flex: 1, minWidth: 140, compact: 'title' },
            { field: 'type', headerName: 'Type', width: 100, compact: 'meta' },
            { field: 'host', headerName: 'Host', flex: 1, minWidth: 140, compact: 'meta' },
            { field: 'port', headerName: 'Port', width: 80, compact: 'hidden' },
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
          ]}
        />

        {canManage && (
          <Menu anchorEl={menuAnchorEl} open={Boolean(menuAnchorEl)} onClose={closeMenu}>
            <MenuItem onClick={handleToggleEnabled}>
              {selectedRow?.enabled ? 'Disable' : 'Enable'}
            </MenuItem>
            <MenuItem onClick={handleDelete} sx={{ color: 'error.main' }}>Delete</MenuItem>
          </Menu>
        )}
      </Card>
      <CreateBrokerDialog open={openCreateDialog} onClose={() => setOpenCreateDialog(false)} onCreated={load} />
    </>
  );
}
