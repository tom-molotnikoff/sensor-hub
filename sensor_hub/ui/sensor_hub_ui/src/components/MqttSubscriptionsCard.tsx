import { useEffect, useState } from 'react';
import { DataGrid } from '@mui/x-data-grid';
import type { GridColDef, GridRowParams } from '@mui/x-data-grid';
import { Button, Box, Menu, MenuItem, Chip } from '@mui/material';
import { apiClient } from '../gen/client';
import type { MQTTSubscription, MQTTBroker } from '../gen/aliases';
import LayoutCard from '../tools/LayoutCard';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import { useIsMobile } from '../hooks/useMobile';
import CreateSubscriptionDialog from './CreateSubscriptionDialog';
import { logger } from '../tools/logger';
import { TypographyH2 } from '../tools/Typography';

export default function MqttSubscriptionsCard() {
  const [subscriptions, setSubscriptions] = useState<MQTTSubscription[]>([]);
  const [brokerMap, setBrokerMap] = useState<Record<number, string>>({});
  const [menuAnchorEl, setMenuAnchorEl] = useState<null | HTMLElement>(null);
  const [selectedRow, setSelectedRow] = useState<MQTTSubscription | null>(null);
  const [openCreateDialog, setOpenCreateDialog] = useState(false);
  const { user } = useAuth();
  const isMobile = useIsMobile();

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

  const handleRowClick = (params: GridRowParams, event: React.MouseEvent) => {
    const id = typeof params.id === 'number' ? params.id : Number(params.id);
    const found = subscriptions.find(s => s.id === id);
    if (found) setSelectedRow(found);
    else setSelectedRow(params.row as MQTTSubscription);
    setMenuAnchorEl(event.currentTarget as HTMLElement);
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

  const allColumns: GridColDef[] = [
    { field: 'id', headerName: 'ID', width: 60 },
    {
      field: 'broker_id', headerName: 'Broker', flex: 1,
      valueGetter: (value: number) => brokerMap[value] || `Broker #${value}`,
    },
    { field: 'topic_pattern', headerName: 'Topic Pattern', flex: 1 },
    { field: 'driver_type', headerName: 'Driver', width: 150 },
    {
      field: 'enabled', headerName: 'Status', width: 100,
      renderCell: (params) => (
        <Chip label={params.value ? 'Enabled' : 'Disabled'} color={params.value ? 'success' : 'default'} size="small" />
      ),
    },
  ];

  const mobileHiddenFields = ['id', 'driver_type'];
  const columns = isMobile
    ? allColumns.filter(col => !mobileHiddenFields.includes(col.field))
    : allColumns;

  const canManage = user && hasPerm(user, 'manage_mqtt');

  return (
    <>
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
          <TypographyH2>MQTT Subscriptions</TypographyH2>
          <Box>
            <Button variant="contained" onClick={() => setOpenCreateDialog(true)} disabled={!canManage}>Add Subscription</Button>
          </Box>
        </Box>
        <div style={{ height: 300, width: '100%' }}>
          <DataGrid rows={subscriptions} columns={columns} pageSizeOptions={[5, 10]}
            initialState={{ pagination: { paginationModel: { pageSize: 5 } } }}
            onRowClick={handleRowClick} />
        </div>

        {canManage && (
          <Menu anchorEl={menuAnchorEl} open={Boolean(menuAnchorEl)} onClose={closeMenu}>
            <MenuItem onClick={handleToggleEnabled}>
              {selectedRow?.enabled ? 'Disable' : 'Enable'}
            </MenuItem>
            <MenuItem onClick={handleDelete} sx={{ color: 'error.main' }}>Delete</MenuItem>
          </Menu>
        )}
      </LayoutCard>
      <CreateSubscriptionDialog open={openCreateDialog} onClose={() => setOpenCreateDialog(false)} onCreated={load} />
    </>
  );
}
