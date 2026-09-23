import Page from '../../ui/Page';
import { useAuth } from '../../providers/AuthContext';
import { hasPerm } from '../../tools/Utils';
import { Grid } from '@mui/material';
import MqttBrokersCard from '../../components/MqttBrokersCard';
import MqttSubscriptionsCard from '../../components/MqttSubscriptionsCard';
import MqttStatsCard from '../../components/MqttStatsCard';
import PendingSensorsCard from '../../components/PendingSensorsCard';

export default function MqttPage() {
  const { user } = useAuth();

  return (
    <Page title="MQTT" loading={user === undefined}>
      <Grid container spacing={2}>
        {hasPerm(user, 'view_mqtt') && (
          <Grid size={12}><MqttStatsCard /></Grid>
        )}
        {hasPerm(user, 'view_sensors') && (
          <Grid size={12}><PendingSensorsCard /></Grid>
        )}
        {hasPerm(user, 'view_mqtt') && (
          <>
            <Grid size={12}><MqttBrokersCard /></Grid>
            <Grid size={12}><MqttSubscriptionsCard /></Grid>
          </>
        )}
      </Grid>
    </Page>
  );
}
