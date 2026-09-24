import Page from '../../ui/Page';
import PageGrid from '../../ui/PageGrid';
import { useAuth } from '../../providers/AuthContext';
import { hasPerm } from '../../tools/Utils';
import MqttBrokersCard from '../../components/MqttBrokersCard';
import MqttSubscriptionsCard from '../../components/MqttSubscriptionsCard';
import MqttStatsCard from '../../components/MqttStatsCard';
import PendingSensorsCard from '../../components/PendingSensorsCard';

export default function MqttPage() {
  const { user } = useAuth();

  return (
    <Page title="MQTT" loading={user === undefined}>
      <PageGrid>
        {hasPerm(user, 'view_mqtt') && (
          <PageGrid.Item><MqttStatsCard /></PageGrid.Item>
        )}
        {hasPerm(user, 'view_sensors') && (
          <PageGrid.Item><PendingSensorsCard /></PageGrid.Item>
        )}
        {hasPerm(user, 'view_mqtt') && (
          <>
            <PageGrid.Item><MqttBrokersCard /></PageGrid.Item>
            <PageGrid.Item><MqttSubscriptionsCard /></PageGrid.Item>
          </>
        )}
      </PageGrid>
    </Page>
  );
}
