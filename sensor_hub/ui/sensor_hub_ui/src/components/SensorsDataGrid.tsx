import type { Sensor } from "../gen/aliases";
import { useState } from 'react';
import { Menu, MenuItem, type SnackbarCloseReason, Snackbar, Alert } from '@mui/material';
import { useNavigate } from "react-router";
import SensorsOffOutlinedIcon from "@mui/icons-material/SensorsOffOutlined";
import { apiClient } from "../gen/client";
import type { AuthUser } from "../providers/AuthContext.tsx";
import { hasPerm } from "../tools/Utils.ts";
import { healthStatus } from "../tools/healthStatus";
import { useSensorContext } from "../hooks/useSensorContext";
import Card from "../ui/Card";
import DataTable from "../ui/DataTable";
import EmptyState from '../ui/EmptyState';

interface SensorsDataGridProps {
  sensors: Sensor[];
  user: AuthUser;
  onAddSensor?: () => void;
}

type SensorRow = Pick<Sensor, 'id' | 'name' | 'sensor_driver' | 'health_status' | 'health_reason' | 'enabled'>;

function SensorsDataGrid({ sensors, user, onAddSensor }: SensorsDataGridProps) {
  const [menuAnchorEl, setMenuAnchorEl] = useState<null | HTMLElement>(null);
  const [selectedRow, setSelectedRow] = useState<SensorRow | null>(null);
  const [snackbarOpen, setSnackbarOpen] = useState(false);
  const [alertSeverity, setAlertSeverity] = useState<'success' | 'error'>('success');
  const [alertMessage, setAlertMessage] = useState('');

  const { loaded } = useSensorContext();

  const navigate = useNavigate();

  const handleClose = (
    _event: React.SyntheticEvent | Event,
    reason?: SnackbarCloseReason,
  ) => {
    if (reason === 'clickaway') {
      return;
    }
    setSnackbarOpen(false);
  };

  const handleRowClick = (row: SensorRow, anchor: HTMLElement) => {
    setSelectedRow(row);
    setMenuAnchorEl(anchor);
  };

  const handleTriggerReading = async () => {
    handleMenuClose();
    try {
      if (selectedRow) {
        await apiClient.POST('/sensors/collect/{sensorName}', { params: { path: { sensorName: selectedRow.name } } });
        setAlertSeverity('success');
        setAlertMessage('Reading triggered successfully');
        setSnackbarOpen(true);
      }
    } catch (err: unknown) {
      setAlertSeverity('error');
      if (err instanceof Error) {
        setAlertMessage(err.message);
      } else {
        setAlertMessage('Failed to trigger reading');
      }
    } finally {
      setSnackbarOpen(true);
    }
  }

  const handleViewDetails = () => {
    handleMenuClose();
    navigate(`/sensor/${selectedRow?.id}`);
  }

  const handleMenuClose = () => {
    setMenuAnchorEl(null);
    setSelectedRow(null);
  };

  const rows: SensorRow[] = sensors.map((sensor) => ({
    id: sensor.id,
    name: sensor.name,
    sensor_driver: sensor.sensor_driver,
    health_status: sensor.health_status,
    health_reason: sensor.health_reason,
    enabled: sensor.enabled,
  }));

  return (
    <Card title="Sensor Summary">
      {loaded && rows.length === 0 ? (
        <EmptyState
          icon={<SensorsOffOutlinedIcon fontSize="large" />}
          title="No sensors found"
          description={hasPerm(user, 'manage_sensors')
            ? "Use the Add Sensor form to register your first sensor."
            : "No sensors have been added yet. Ask an administrator to add sensors."}
          actionLabel={hasPerm(user, 'manage_sensors') ? "Add a sensor" : undefined}
          onAction={hasPerm(user, 'manage_sensors') ? (onAddSensor ?? undefined) : undefined}
          actionHref={hasPerm(user, 'manage_sensors') && !onAddSensor ? "/sensors-overview" : undefined}
        />
      ) : (
        <>
          <DataTable
            rows={rows}
            loading={!loaded}
            onRowClick={handleRowClick}
            columns={[
              { field: 'name', headerName: 'Sensor Name', flex: 1, minWidth: 100, compact: 'title' },
              { field: 'sensor_driver', headerName: 'Driver', flex: 1, minWidth: 100, compact: 'meta' },
              {
                field: 'health_status',
                headerName: 'Health Status',
                flex: 1,
                minWidth: 100,
                compact: 'status',
                statusOf: (row) => healthStatus[row.health_status],
              },
              { field: 'health_reason', headerName: 'Health Reason', flex: 2, minWidth: 200, compact: 'hidden' },
              {
                field: 'enabled',
                headerName: 'Enabled',
                flex: 1,
                minWidth: 80,
                type: 'boolean',
                compact: 'meta',
                valueFormatter: (value: boolean) => (value ? 'enabled' : 'disabled'),
              },
            ]}
          />
          <Menu
            anchorEl={menuAnchorEl}
            open={Boolean(menuAnchorEl)}
            onClose={handleMenuClose}
          >
            {(hasPerm(user, "trigger_readings")) &&
              <MenuItem onClick={handleTriggerReading}>Trigger Reading</MenuItem>
            }
            <MenuItem onClick={handleViewDetails}>View Details</MenuItem>
          </Menu>
        </>
      )}
      <Snackbar
        open={snackbarOpen}
        autoHideDuration={2000}
        onClose={handleClose}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert onClose={handleClose} severity={alertSeverity}>
          {alertMessage}
        </Alert>
      </Snackbar>
    </Card>
  );
}

export default SensorsDataGrid
