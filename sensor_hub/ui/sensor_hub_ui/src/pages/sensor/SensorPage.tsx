import { Grid } from '@mui/material';
import Page from '../../ui/Page';
import { useSensorContext } from '../../hooks/useSensorContext';
import { useIsMobile } from '../../hooks/useMobile';
import SensorInfoCard from '../../components/SensorInfoCard';
import EditSensorDetails from '../../components/EditSensorDetails';
import SensorHealthHistory from '../../components/SensorHealthHistory';
import SensorHealthHistoryChartCard from '../../components/SensorHealthHistoryChartCard';
import SensorTemperatureDataCard from '../../components/SensorTemperatureDataCard';
import { useAuth } from '../../providers/AuthContext';
import { hasPerm } from '../../tools/Utils';

interface SensorPageProps {
  sensorId: number;
}

function SensorPage({ sensorId }: SensorPageProps) {
  const { sensors } = useSensorContext();
  const sensor = sensors.find(s => s.id === sensorId);
  const { user } = useAuth();
  const isMobile = useIsMobile();

  if (user === undefined) {
    return (
      <Page title="Sensor" loading />
    );
  }

  if (!sensor) {
    return (
      <Page title="Sensor Not Found">
        <Grid container spacing={2}>
          <Grid size={12}>
            <h2>Sensor with ID {sensorId} not found.</h2>
          </Grid>
        </Grid>
      </Page>
    );
  }

  return (
    <Page title="Sensor">
      <Grid container spacing={2}>
        {hasPerm(user, 'view_sensors') && (
          <>
            <Grid size={isMobile ? 12 : 6}><SensorInfoCard sensor={sensor} user={user} /></Grid>
            <Grid size={isMobile ? 12 : 6}><EditSensorDetails sensor={sensor} /></Grid>
          </>
        )}
        {hasPerm(user, 'view_readings') && (
          <>
            <Grid size={isMobile ? 12 : 6}><SensorHealthHistoryChartCard sensor={sensor} /></Grid>
            <Grid size={isMobile ? 12 : 6}><SensorTemperatureDataCard sensor={sensor} /></Grid>
          </>
        )}
        {hasPerm(user, 'view_sensors') && (
          <Grid size={isMobile ? 12 : 6}><SensorHealthHistory sensor={sensor} /></Grid>
        )}
      </Grid>
    </Page>
  );
}

export default SensorPage;
