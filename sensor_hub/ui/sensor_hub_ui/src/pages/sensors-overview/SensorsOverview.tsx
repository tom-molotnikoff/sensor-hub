import Page from '../../ui/Page';
import { useIsMobile } from '../../hooks/useMobile';
import { Grid } from '@mui/material';
import SensorHealthCard from '../../components/SensorHealthCard';
import AddNewSensor from '../../components/AddNewSensor';
import SensorTypeCard from '../../components/SensorTypeCard';
import TotalReadingsForEachSensorCard from '../../components/TotalReadingsForEachSensorCard';
import AllSensorsCard from '../../components/AllSensorsCard';
import PendingSensorsCard from '../../components/PendingSensorsCard';
import { useAuth } from '../../providers/AuthContext';
import { hasPerm } from '../../tools/Utils';

function SensorsOverview() {
  const isMobile = useIsMobile();
  const { user } = useAuth();

  return (
    <Page title="Sensors Overview" loading={user === undefined}>
      <Grid container spacing={2}>
        {hasPerm(user, 'manage_sensors') && (
          <Grid size={isMobile ? 12 : 4}><AddNewSensor /></Grid>
        )}
        {hasPerm(user, 'view_sensors') && (
          <>
            <Grid size={isMobile ? 12 : 4}><SensorHealthCard /></Grid>
            <Grid size={isMobile ? 12 : 4}><SensorTypeCard /></Grid>
            <Grid size={isMobile ? 12 : 8}><AllSensorsCard /></Grid>
            <Grid size={isMobile ? 12 : 4}><TotalReadingsForEachSensorCard /></Grid>
          </>
        )}
        {hasPerm(user, 'manage_sensors') && (
          <Grid size={12}><PendingSensorsCard /></Grid>
        )}
      </Grid>
    </Page>
  );
}

export default SensorsOverview;
