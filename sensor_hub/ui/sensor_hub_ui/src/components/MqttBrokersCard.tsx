import { useEffect, useState } from 'react';
import { DataGrid } from '@mui/x-data-grid';
import type { GridColDef, GridRowParams } from '@mui/x-data-grid';
import { Button, Box, Menu, MenuItem, Chip } from '@mui/material';
import { apiClient } from '../gen/client';
import type { MQTTBroker } from '../gen/aliases';
import LayoutCard from '../tools/LayoutCard';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import { useIsMobile } from '../hooks/useMobile';
import CreateBrokerDialog from './CreateBrokerDialog';
import { logger } from '../tools/logger';
import { TypographyH2 } from '../tools/Typography';

export default function MqttBrokersCard() {
  const [brokers, setBrokers] = useState<MQTTBroker[]>([]);
  const [menuAnchorEl, setMenuAnchorEl] = useState<null | HTMLElement>(null);
  const [selectedRow, setSelectedRow] = useState<MQTTBroker | null>(null);
  const [openCreateDialog, setOpenCreateDialog] = useState(false);
  const { user } = useAuth();
  const isMobile = useIsMobile();

  const load = () =>
    apiClient.GET('/mqtt/brokers')
      .then(({ data: b }) => setBrokers((b as MQTTBroker[] | null) ?? []))
      .catch((e) => logger.error(e));

  useEffect(() => { void load(); }, []);

  const handleRowClick = (params: GridRowParams, event: React.MouseEvent) => {
    const id = typeof params.id === 'number' ? params.id : Number(params.id);
    const found = brokers.find(b => b.id === id);
    if (found) setSelectedRow(found);
    else setSelectedRow(params.row as MQTTBroker);
    setMenuAnchorEl(event.currentTarget as HTMLElement);
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

  const allColumns: GridColDef[] = [
    { field: 'id', headerName: 'ID', width: 60 },
    { field: 'name', headerName: 'Name', flex: 1 },
    { field: 'type', headerName: 'Type', width: 100 },
    { field: 'host', headerName: 'Host', flex: 1 },
    { field: 'port', headerName: 'Port', width: 80 },
    {
      field: 'enabled', headerName: 'Status', width: 100,
      renderCell: (params) => (
        <Chip label={params.value ? 'Enabled' : 'Disabled'} color={params.value ? 'success' : 'default'} size="small" />
      ),
    },
  ];

  const mobileHiddenFields = ['id', 'type', 'port'];
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
          <TypographyH2>MQTT Brokers</TypographyH2>
          <Box>
            <Button variant="contained" onClick={() => setOpenCreateDialog(true)} disabled={!canManage}>Add Broker</Button>
          </Box>
        </Box>
        <div style={{ height: 300, width: '100%' }}>
          <DataGrid rows={brokers} columns={columns} pageSizeOptions={[5, 10]}
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
      <CreateBrokerDialog open={openCreateDialog} onClose={() => setOpenCreateDialog(false)} onCreated={load} />
    </>
  );
}
