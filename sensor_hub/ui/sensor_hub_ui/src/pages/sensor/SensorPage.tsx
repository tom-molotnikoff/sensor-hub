import { Typography } from '@mui/material';
import Page from '../../ui/Page';
import PageGrid from '../../ui/PageGrid';
import Card from '../../ui/Card';
import { useSensorContext } from '../../hooks/useSensorContext';
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

  if (user === undefined) {
    return (
      <Page title="Sensor" loading />
    );
  }

  if (!sensor) {
    return (
      <Page title="Sensor Not Found">
        <Card>
          <Typography variant="body">Sensor with ID {sensorId} not found.</Typography>
        </Card>
      </Page>
    );
  }

  return (
    <Page title="Sensor">
      <PageGrid>
        {hasPerm(user, 'view_sensors') && (
          <>
            <PageGrid.Item span={{ wide: 6 }}><SensorInfoCard sensor={sensor} user={user} /></PageGrid.Item>
            <PageGrid.Item span={{ wide: 6 }}><EditSensorDetails sensor={sensor} /></PageGrid.Item>
          </>
        )}
        {hasPerm(user, 'view_readings') && (
          <>
            <PageGrid.Item span={{ wide: 6 }}><SensorHealthHistoryChartCard sensor={sensor} /></PageGrid.Item>
            <PageGrid.Item span={{ wide: 6 }}><SensorTemperatureDataCard sensor={sensor} /></PageGrid.Item>
          </>
        )}
        {hasPerm(user, 'view_sensors') && (
          <PageGrid.Item span={{ wide: 6 }}><SensorHealthHistory sensor={sensor} /></PageGrid.Item>
        )}
      </PageGrid>
    </Page>
  );
}

export default SensorPage;
