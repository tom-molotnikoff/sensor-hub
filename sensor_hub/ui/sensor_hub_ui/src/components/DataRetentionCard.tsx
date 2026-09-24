import { useState } from 'react';
import type { Sensor } from '../gen/aliases';
import { useSensorContext } from '../hooks/useSensorContext';
import { useProperties } from '../hooks/useProperties';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import { Typography, Chip, Menu, MenuItem } from '@mui/material';
import { formatRetention } from '../tools/retention';
import EditRetentionDialog from './EditRetentionDialog';
import Card from '../ui/Card';
import DataTable from '../ui/DataTable';
import Stack from '../ui/Stack';

function DataRetentionCard() {
  const { sensors } = useSensorContext();
  const properties = useProperties();
  const { user } = useAuth();

  const globalRetentionDays = parseInt(properties['sensor.data.retention.days'] || '90', 10);
  const globalRetentionHours = globalRetentionDays * 24;

  const activeSensors = sensors.filter((s) => s.status === 'active');

  const [menuAnchorEl, setMenuAnchorEl] = useState<null | HTMLElement>(null);
  const [selectedSensor, setSelectedSensor] = useState<Sensor | null>(null);
  const [openEditDialog, setOpenEditDialog] = useState(false);
  const [retentionOverrides, setRetentionOverrides] = useState<Record<number, number | null>>({});

  const displaySensors = activeSensors.map((s) => {
    if (s.id in retentionOverrides) {
      return { ...s, retention_hours: retentionOverrides[s.id] };
    }
    return s;
  });

  const handleRowClick = (sensor: Sensor, anchor: HTMLElement) => {
    setSelectedSensor(sensor);
    setMenuAnchorEl(anchor);
  };

  const closeMenu = () => setMenuAnchorEl(null);
  const canManage = user && hasPerm(user, 'manage_sensors');

  return (
    <>
      <Card title="Sensor Retention Overview">
        <Stack>
          <Typography variant="body2" sx={{ color: "text.secondary" }}>
            Global default: {formatRetention(globalRetentionHours)}. Select a sensor to edit its retention policy.
          </Typography>
          <DataTable
            rows={displaySensors}
            onRowClick={handleRowClick}
            columns={[
              { field: 'name', headerName: 'Sensor', flex: 1, minWidth: 140, compact: 'title' },
              { field: 'sensor_driver', headerName: 'Driver', flex: 1, minWidth: 120, compact: 'hidden' },
              {
                field: 'retention_hours',
                headerName: 'Retention',
                flex: 1,
                minWidth: 140,
                compact: 'meta',
                valueFormatter: (value: number | null) => (value != null ? 'Custom' : 'Global default'),
                renderCell: ({ row }) => {
                  if (row.retention_hours != null) {
                    return <Chip label={formatRetention(row.retention_hours)} color="primary" size="small" variant="outlined" />;
                  }
                  return <Typography variant="body2" sx={{ color: "text.secondary" }}>Global default</Typography>;
                },
              },
              {
                field: 'effective',
                headerName: 'Effective',
                flex: 1,
                minWidth: 120,
                compact: 'meta',
                valueGetter: (_value: never, row: Sensor) => formatRetention(row.retention_hours ?? globalRetentionHours),
              },
            ]}
          />
        </Stack>

        <Menu anchorEl={menuAnchorEl} open={Boolean(menuAnchorEl)} onClose={closeMenu}>
          <MenuItem
            disabled={!canManage}
            onClick={() => { closeMenu(); setOpenEditDialog(true); }}
          >
            Edit Retention
          </MenuItem>
        </Menu>
      </Card>
      <EditRetentionDialog
        open={openEditDialog}
        onClose={() => setOpenEditDialog(false)}
        onSaved={(sensorId, retentionHours) => {
          setRetentionOverrides((prev) => ({ ...prev, [sensorId]: retentionHours }));
        }}
        sensor={selectedSensor}
        globalRetentionHours={globalRetentionHours}
      />
    </>
  );
}

export default DataRetentionCard;
