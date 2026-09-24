import Page from '../../ui/Page';
import PageGrid from '../../ui/PageGrid';
import SensorHealthCard from '../../components/SensorHealthCard';
import AddNewSensor from '../../components/AddNewSensor';
import SensorTypeCard from '../../components/SensorTypeCard';
import TotalReadingsForEachSensorCard from '../../components/TotalReadingsForEachSensorCard';
import AllSensorsCard from '../../components/AllSensorsCard';
import PendingSensorsCard from '../../components/PendingSensorsCard';
import { useAuth } from '../../providers/AuthContext';
import { hasPerm } from '../../tools/Utils';

function SensorsOverview() {
  const { user } = useAuth();

  return (
    <Page title="Sensors Overview" loading={user === undefined}>
      <PageGrid equalHeight>
        {hasPerm(user, 'manage_sensors') && (
          <PageGrid.Item span={{ wide: 4 }}><AddNewSensor /></PageGrid.Item>
        )}
        {hasPerm(user, 'view_sensors') && (
          <>
            <PageGrid.Item span={{ wide: 4 }}><SensorHealthCard /></PageGrid.Item>
            <PageGrid.Item span={{ wide: 4 }}><SensorTypeCard /></PageGrid.Item>
            <PageGrid.Item span={{ wide: 8 }}><AllSensorsCard /></PageGrid.Item>
            <PageGrid.Item span={{ wide: 4 }}><TotalReadingsForEachSensorCard /></PageGrid.Item>
          </>
        )}
        {hasPerm(user, 'manage_sensors') && (
          <PageGrid.Item span={{ wide: 12 }}><PendingSensorsCard /></PageGrid.Item>
        )}
      </PageGrid>
    </Page>
  );
}

export default SensorsOverview;
