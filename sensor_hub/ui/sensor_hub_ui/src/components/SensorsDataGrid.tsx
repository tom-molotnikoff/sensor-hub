import type {Sensor, SensorHealthStatus} from "../gen/aliases";
import LayoutCard from "../tools/LayoutCard.tsx";
import {TypographyH2} from "../tools/Typography.tsx";
import {DataGrid, type GridColDef, type GridRowParams} from '@mui/x-data-grid';
import { useIsMobile } from "../hooks/useMobile";
import {useState} from 'react';
import {Menu, MenuItem, type SnackbarCloseReason, Snackbar, Alert} from '@mui/material';
import {useNavigate} from "react-router";
import { apiClient } from "../gen/client";
import type {AuthUser} from "../providers/AuthContext.tsx";
import {hasPerm} from "../tools/Utils.ts";
import { useSensorContext } from "../hooks/useSensorContext";
import EmptyState from '../ui/EmptyState';
import SensorsOffOutlinedIcon from "@mui/icons-material/SensorsOffOutlined";

interface SensorSummaryCardProps {
  sensors: Sensor[],
  cardHeight?: string | number;
  showReason: boolean;
  showType: boolean;
  showEnabled: boolean;
  title?: string;
  user: AuthUser;
  onAddSensor?: () => void;
}

type row = {
  id: string | number;
  name: string;
  sensor_driver: string;
  health_status: SensorHealthStatus;
  health_reason: string;
  enabled: boolean;
} | null;

function SensorsDataGrid({ sensors, cardHeight, showReason, showType, title, showEnabled, user, onAddSensor }: SensorSummaryCardProps) {
  const isMobile = useIsMobile();
  const [menuAnchorEl, setMenuAnchorEl] = useState<null | HTMLElement>(null);
  const [selectedRow, setSelectedRow] = useState<row>(null);
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
  const handleRowClick = (params: GridRowParams, event: React.MouseEvent) => {
    setSelectedRow(params.row);
    setMenuAnchorEl(event.currentTarget as HTMLElement);
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

  const columns: GridColDef[] = [
    { field: 'name', headerName: 'Sensor Name', flex: 1, minWidth: 100 },
    { field: 'sensor_driver', headerName: 'Driver', flex: 1, minWidth: 100 },
    { field: 'health_status', headerName: 'Health Status', flex: 1, minWidth: 100 },
    { field: 'health_reason', headerName: 'Health Reason', flex: 2, minWidth: 200 },
    { field: 'enabled', headerName: 'Enabled', flex: 1, minWidth: 80, type: 'boolean' },
  ];

  const rows: row[] = sensors.map((sensor) => ({
    id: sensor.id,
    name: sensor.name,
    sensor_driver: sensor.sensor_driver,
    health_status: sensor.health_status,
    health_reason: sensor.health_reason,
    enabled: sensor.enabled,
  }));

  const columnVisibilityModel = {
    id: true,
    name: true,
    sensor_driver: showType,
    health_status: true,
    health_reason: showReason,
    enabled: showEnabled,
  }

  return (
    <LayoutCard variant="secondary" changes={{alignItems: "center", height: cardHeight, width: "100%"}}>
      <TypographyH2>{title ? title : "Sensor Summary"}</TypographyH2>
      <div
        style={{
          height: cardHeight,
          paddingBottom: 10,
          display: "flex",
          flexDirection: "column",
          width: "100%"
        }}
      >
        {loaded && rows.length === 0 ? (
          <EmptyState
            icon={<SensorsOffOutlinedIcon sx={{ fontSize: 48 }} />}
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
            <DataGrid
              showToolbar
              rows={rows}
              columns={columns}
              pageSizeOptions={[5, 10, 25, 50, 100]}
              initialState={{
                pagination: {
                  paginationModel: { pageSize: 5, page: 0 },
                },
              }}
              getRowHeight={showReason ? () => 'auto' : undefined}
              onRowClick={handleRowClick}
              columnVisibilityModel={columnVisibilityModel}
              loading={!loaded}
              sx={{
                backgroundColor: 'background.paper',
                borderRadius: 2,
                mt: 2,
                '& .MuiDataGrid-cell': { fontSize: isMobile ? '0.9rem' : '1rem' },
                '& .MuiDataGrid-columnHeaders': { fontWeight: 'bold' },
                '.MuiDataGrid-row:hover': { cursor: 'pointer' },
              }}
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
      </div>
      <Snackbar
        open={snackbarOpen}
        autoHideDuration={2000}
        onClose={handleClose}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert onClose={handleClose} severity={alertSeverity} sx={{ width: '100%' }}>
          {alertMessage}
        </Alert>
      </Snackbar>
    </LayoutCard>
  );
}


export default SensorsDataGrid