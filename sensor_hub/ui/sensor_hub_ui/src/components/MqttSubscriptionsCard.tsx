import { useEffect, useState } from 'react';
import { Button, Menu, MenuItem, Chip } from '@mui/material';
import { apiClient } from '../gen/client';
import type { MQTTSubscription, MQTTBroker } from '../gen/aliases';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import CreateSubscriptionDialog from './CreateSubscriptionDialog';
import { logger } from '../tools/logger';
import Card from '../ui/Card';
import DataTable from '../ui/DataTable';

export default function MqttSubscriptionsCard() {
  const [subscriptions, setSubscriptions] = useState<MQTTSubscription[]>([]);
  const [brokerMap, setBrokerMap] = useState<Record<number, string>>({});
  const [menuAnchorEl, setMenuAnchorEl] = useState<null | HTMLElement>(null);
  const [selectedRow, setSelectedRow] = useState<MQTTSubscription | null>(null);
  const [openCreateDialog, setOpenCreateDialog] = useState(false);
  const { user } = useAuth();

  const load = () =>
    Promise.all([
      apiClient.GET('/mqtt/subscriptions'),
      apiClient.GET('/mqtt/brokers'),
    ])
      .then(([{ data: subs }, { data: brokers }]) => {
        setSubscriptions((subs as MQTTSubscription[] | null) ?? []);
        const map: Record<number, string> = {};
        ((brokers as MQTTBroker[] | null) ?? []).forEach((b: MQTTBroker) => { map[b.id] = b.name; });
        setBrokerMap(map);
      })
      .catch((e) => logger.error(e));

  useEffect(() => { void load(); }, []);

  const handleRowClick = (row: MQTTSubscription, anchor: HTMLElement) => {
    setSelectedRow(row);
    setMenuAnchorEl(anchor);
  };

  const closeMenu = () => { setMenuAnchorEl(null); };

  const handleToggleEnabled = async () => {
    if (!selectedRow) return;
    closeMenu();
    try {
      await apiClient.PUT('/mqtt/subscriptions/{id}', {
        params: { path: { id: selectedRow.id } },
        body: { broker_id: selectedRow.broker_id, topic_pattern: selectedRow.topic_pattern, driver_type: selectedRow.driver_type, enabled: !selectedRow.enabled } as never,
      });
      await load();
    } catch (e) { logger.error('Failed to toggle subscription', e); }
  };

  const handleDelete = async () => {
    if (!selectedRow) return;
    closeMenu();
    try {
      await apiClient.DELETE('/mqtt/subscriptions/{id}', { params: { path: { id: selectedRow.id } } });
      await load();
    } catch (e) { logger.error('Failed to delete subscription', e); }
  };

  const canManage = user && hasPerm(user, 'manage_mqtt');

  return (
    <>
      <Card
        title="MQTT Subscriptions"
        actions={<Button variant="contained" onClick={() => setOpenCreateDialog(true)} disabled={!canManage}>Add Subscription</Button>}
      >
        <DataTable
          rows={subscriptions}
          onRowClick={handleRowClick}
          columns={[
            { field: 'id', headerName: 'ID', width: 60, compact: 'hidden' },
            {
              field: 'broker_id',
              headerName: 'Broker',
              flex: 1,
              minWidth: 140,
              compact: 'meta',
              valueGetter: (value: number) => brokerMap[value] || `Broker #${value}`,
            },
            { field: 'topic_pattern', headerName: 'Topic Pattern', flex: 1, minWidth: 160, compact: 'title' },
            { field: 'driver_type', headerName: 'Driver', width: 150, compact: 'meta' },
            {
              field: 'enabled',
              headerName: 'Status',
              width: 110,
              compact: 'status',
              statusOf: (row) => (row.enabled ? 'ok' : 'unknown'),
              valueFormatter: (value: boolean) => (value ? 'Enabled' : 'Disabled'),
              renderCell: ({ row }) => (
                <Chip label={row.enabled ? 'Enabled' : 'Disabled'} color={row.enabled ? 'success' : 'default'} size="small" />
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
      <CreateSubscriptionDialog open={openCreateDialog} onClose={() => setOpenCreateDialog(false)} onCreated={load} />
    </>
  );
}
