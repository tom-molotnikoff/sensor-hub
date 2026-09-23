import Page from '../../ui/Page';
import { useAuth } from '../../providers/AuthContext';
import { hasPerm } from '../../tools/Utils';
import { Grid } from '@mui/material';
import AlertRulesCard from '../../components/AlertRulesCard';
import NotificationsCard from '../../components/NotificationsCard';
import NotificationPreferencesCard from '../../components/NotificationPreferencesCard';
import OAuthConfigCard from '../../components/OAuthConfigCard';

export default function NotificationsPage() {
  const { user } = useAuth();

  return (
    <Page title="Alerts & Notifications" loading={user === undefined}>
      <Grid container spacing={2}>
        {hasPerm(user, 'view_alerts') && (
          <Grid size={12}><AlertRulesCard /></Grid>
        )}
        {hasPerm(user, 'manage_notifications') && (
          <Grid size={12}><NotificationPreferencesCard /></Grid>
        )}
        {hasPerm(user, 'manage_oauth') && (
          <Grid size={12}><OAuthConfigCard /></Grid>
        )}
        {hasPerm(user, 'view_notifications') && (
          <Grid size={12}><NotificationsCard /></Grid>
        )}
      </Grid>
    </Page>
  );
}
