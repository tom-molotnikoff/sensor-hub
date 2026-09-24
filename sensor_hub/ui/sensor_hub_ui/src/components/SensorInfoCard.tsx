import type {Sensor} from "../gen/aliases";
import LayoutCard from "../tools/LayoutCard.tsx";
import { Chip, Typography, Box, Avatar, Button} from '@mui/material';
import SensorsIcon from '@mui/icons-material/Sensors';
import { useState, useEffect } from 'react';
import { Dialog, DialogTitle, DialogContent, DialogContentText, DialogActions, Alert, CircularProgress } from '@mui/material';
import { useNavigate } from 'react-router';
import { apiClient } from "../gen/client";
import type { MeasurementTypeInfo } from '../gen/aliases';
import type {AuthUser} from "../providers/AuthContext.tsx";
import {hasPerm} from "../tools/Utils.ts";
import {TypographyH2} from "../tools/Typography.tsx";
import type { StatusKey } from "../ui/theme";
import {useProperties} from "../hooks/useProperties.ts";
import {formatRetention} from "../tools/retention.ts";
import {getDisplayableDeviceInfo} from "../tools/deviceMetadata.ts";

interface SensorInfoCardProps {
  sensor: Sensor
  onDelete?: (name: string) => void
  onDisable?: (name: string) => void
  onEnable?: (name: string) => void
  user: AuthUser;
}

const healthStatus: Record<Sensor['health_status'], StatusKey> = {
  good: 'ok',
  bad: 'bad',
  unknown: 'unknown',
};

function getHealthBgColor(status: Sensor['health_status']) {
  switch (status) {
    case 'good': return 'success.main';
    case 'bad': return 'error.main';
    case 'unknown': return 'warning.main';
    default: return 'grey.400';
  }
}

function InfoField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <Box>
      <Typography variant="subtitle2" sx={{
        color: "text.secondary"
      }}>{label}</Typography>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          mt: 0.5
        }}>{children}</Box>
    </Box>
  );
}

function SensorInfoCard({sensor, onDelete, onDisable, onEnable, user}: SensorInfoCardProps) {
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [disableDialogOpen, setDisableDialogOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [measurementTypes, setMeasurementTypes] = useState<MeasurementTypeInfo[]>([]);
  const navigate = useNavigate();
  const properties = useProperties();

  const globalRetentionDays = parseInt(properties['sensor.data.retention.days'] || '90', 10);
  const globalRetentionHours = globalRetentionDays * 24;
  const effectiveHours = sensor.retention_hours ?? globalRetentionHours;
  const deviceInfo = getDisplayableDeviceInfo(sensor.metadata);

  useEffect(() => {
    apiClient.GET('/sensors/by-id/{id}/measurement-types', { params: { path: { id: sensor.id } } })
      .then(({ data }) => setMeasurementTypes(data ?? []))
      .catch(() => setMeasurementTypes([]));
  }, [sensor.id]);

  const openDeleteDialog = () => {
    setErrorMessage(null);
    setSuccessMessage(null);
    setDeleteDialogOpen(true);
  };

  const openDisableDialog = () => {
    setErrorMessage(null);
    setSuccessMessage(null);
    setDisableDialogOpen(true);
  }

  const closeDisableDialog = () => {
    if (loading) return;
    setDisableDialogOpen(false);
  }

  const closeDeleteDialog = () => {
    if (loading) return;
    setDeleteDialogOpen(false);
  };

  const performSensorAction = async (action: 'enable' | 'disable' | 'delete') => {
    setLoading(true);
    setErrorMessage(null);

    try {
      switch (action) {
        case 'enable':
          await apiClient.POST('/sensors/enable/{sensorName}', { params: { path: { sensorName: sensor.name } } });
          setSuccessMessage('Sensor enabled');
          if (onEnable) onEnable(sensor.name);
          break;

        case 'disable':
          await apiClient.POST('/sensors/disable/{sensorName}', { params: { path: { sensorName: sensor.name } } });
          setSuccessMessage('Sensor disabled');
          setDisableDialogOpen(false);
          if (onDisable) onDisable(sensor.name);
          break;

        case 'delete':
          await apiClient.DELETE('/sensors/{name}', { params: { path: { name: sensor.name } } });
          setSuccessMessage('Sensor deleted');
          setDeleteDialogOpen(false);
          if (onDelete) onDelete(sensor.name);
          navigate('/sensors-overview');
          break;
      }

      setTimeout(() => setSuccessMessage(null), 1500);
    } catch (err: unknown) {
      const msg = (err && typeof err === 'object' && 'message' in err) ? String((err as { message: unknown }).message) : String(err);
      setErrorMessage(msg);
      return;
    } finally {
      setLoading(false);
    }
  };
  const fieldsDisabled = !(hasPerm(user, "manage_sensors"));
  const handleEnableSensor = async () => { await performSensorAction('enable'); };
  const handleConfirmDisable = async () => { await performSensorAction('disable'); };
  const handleConfirmDelete = async () => { await performSensorAction('delete'); };

  return (
    <LayoutCard variant="secondary" changes={{height: "100%", width: "100%", display: "flex", flexDirection: "column"}}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 2,
          mb: 2
        }}>
        <TypographyH2>
          {sensor.name}
        </TypographyH2>
        <Avatar sx={{ bgcolor: getHealthBgColor(sensor.health_status), width: 40, height: 40 }}>
          <SensorsIcon />
        </Avatar>
      </Box>
      <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: 2 }}>
        <InfoField label="Driver"><Chip label={sensor.sensor_driver} color="primary" size="small" /></InfoField>
        <InfoField label="Health"><Chip label={sensor.health_status} size="small" sx={{ color: `status.${healthStatus[sensor.health_status]}.strong`, bgcolor: `status.${healthStatus[sensor.health_status]}.soft` }} /></InfoField>
        <InfoField label="Enabled"><Chip label={sensor.enabled ? 'true' : 'false'} color={sensor.enabled ? 'success' : 'error'} size="small" /></InfoField>
        <InfoField label="Retention">
          {sensor.retention_hours !== null
            ? <Chip label={`Custom: ${formatRetention(effectiveHours)}`} color="primary" size="small" variant="outlined" />
            : <Typography variant="body2" sx={{
            color: "text.secondary"
          }}>Global default ({formatRetention(globalRetentionHours)})</Typography>
          }
        </InfoField>
        {sensor.external_id && (
          <InfoField label="Device ID">
            <Typography
              variant="body2"
              sx={{
                color: "text.secondary",
                fontFamily: 'monospace'
              }}>{sensor.external_id}</Typography>
          </InfoField>
        )}
        {sensor.health_reason && (
          <InfoField label="Health Reason">
            <Typography variant="body2" sx={{
              color: "text.secondary"
            }}>{sensor.health_reason}</Typography>
          </InfoField>
        )}
        {sensor.config && Object.entries(sensor.config).map(([key, value]) => (
          <InfoField key={key} label={key}>
            <Typography variant="body2" sx={{
              color: "text.secondary"
            }}>{value}</Typography>
          </InfoField>
        ))}
      </Box>
      {deviceInfo.length > 0 && (
        <Box sx={{
          mt: 2
        }}>
          <Typography variant="subtitle1" sx={{ mb: 1 }}>Device Info</Typography>
          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))',
              gap: 2,
              p: 2,
              border: 1,
              borderColor: 'divider',
              borderRadius: 2,
              bgcolor: 'action.hover',
            }}
          >
            {deviceInfo.map(({ key, label, value }) => (
              <InfoField key={key} label={label}>
                <Typography
                  variant="body2"
                  sx={[{ color: "text.secondary" }, key === 'ieee_address' && { fontFamily: 'monospace' }]}>
                  {value}
                </Typography>
              </InfoField>
            ))}
          </Box>
        </Box>
      )}
      {measurementTypes.length > 0 && (
        <Box sx={{
          mt: 2
        }}>
          <Typography
            variant="subtitle2"
            sx={{
              color: "text.secondary",
              mb: 0.5
            }}>Measurement Types</Typography>
          <Box
            sx={{
              display: "flex",
              flexWrap: "wrap",
              gap: 1
            }}>
            {measurementTypes.map((mt) => (
              <Chip key={mt.id} label={`${mt.display_name} (${mt.unit})`} size="small" variant="outlined" />
            ))}
          </Box>
        </Box>
      )}
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 1,
          mt: "auto",
          pt: 2
        }}>
        <Button
          variant="contained"
          color="error"
          onClick={openDeleteDialog}
          disabled={loading || fieldsDisabled}
        >
          Delete
        </Button>
        <Button
          variant="outlined"
          color="warning"
          onClick={openDisableDialog}
          disabled={!sensor.enabled || loading || fieldsDisabled}
        >
          Disable
        </Button>
        <Button
          variant="contained"
          color="success"
          onClick={handleEnableSensor}
          disabled={sensor.enabled || loading || fieldsDisabled}
        >
          Enable
        </Button>
      </Box>
      <Dialog open={disableDialogOpen} onClose={closeDisableDialog} >
        <DialogTitle>Disable sensor "{sensor.name}"?</DialogTitle>
        <DialogContent>
          <DialogContentText>
            This action will disable the sensor, preventing it from collecting new data. Existing data will be retained. You can re-enable the sensor later if needed.
          </DialogContentText>

          {errorMessage && <Box sx={{
            mt: 2
          }}><Alert severity="error">{errorMessage}</Alert></Box>}
          {successMessage && <Box sx={{
            mt: 2
          }}><Alert severity="success">{successMessage}</Alert></Box>}
        </DialogContent>
        <DialogActions>
          <Button onClick={closeDisableDialog} disabled={loading}>Cancel</Button>
          <Button onClick={handleConfirmDisable} color="warning" variant="contained" disabled={loading} startIcon={loading ? <CircularProgress size={18} /> : null}>
            {loading ? 'Disabling...' : 'Confirm Disable'}
          </Button>
        </DialogActions>
      </Dialog>
      <Dialog open={deleteDialogOpen} onClose={closeDeleteDialog}>
        <DialogTitle>Delete sensor "{sensor.name}"?</DialogTitle>
        <DialogContent>
          <DialogContentText>
            This action will permanently delete the sensor from the system. This will also purge any associated sensor readings, if you want to keep the readings, consider disabling the sensor instead. Purging may take some time depending on the volume of data.
          </DialogContentText>

          {errorMessage && <Box sx={{
            mt: 2
          }}><Alert severity="error">{errorMessage}</Alert></Box>}
          {successMessage && <Box sx={{
            mt: 2
          }}><Alert severity="success">{successMessage}</Alert></Box>}
        </DialogContent>
        <DialogActions>
          <Button onClick={closeDeleteDialog} disabled={loading}>Cancel</Button>
          <Button onClick={handleConfirmDelete} color="error" variant="contained" disabled={loading} startIcon={loading ? <CircularProgress size={18} /> : null}>
            {loading ? 'Deleting...' : 'Confirm Delete'}
          </Button>
        </DialogActions>
      </Dialog>
    </LayoutCard>
  );
}

export default SensorInfoCard;
