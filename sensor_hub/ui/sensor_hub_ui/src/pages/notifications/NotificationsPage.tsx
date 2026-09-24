import Page from '../../ui/Page';
import PageGrid from '../../ui/PageGrid';
import { useAuth } from '../../providers/AuthContext';
import { hasPerm } from '../../tools/Utils';
import AlertRulesCard from '../../components/AlertRulesCard';
import NotificationsCard from '../../components/NotificationsCard';
import NotificationPreferencesCard from '../../components/NotificationPreferencesCard';
import OAuthConfigCard from '../../components/OAuthConfigCard';

export default function NotificationsPage() {
  const { user } = useAuth();

  return (
    <Page title="Alerts & Notifications" loading={user === undefined}>
      <PageGrid>
        {hasPerm(user, 'view_alerts') && (
          <PageGrid.Item><AlertRulesCard /></PageGrid.Item>
        )}
        {hasPerm(user, 'manage_notifications') && (
          <PageGrid.Item><NotificationPreferencesCard /></PageGrid.Item>
        )}
        {hasPerm(user, 'manage_oauth') && (
          <PageGrid.Item><OAuthConfigCard /></PageGrid.Item>
        )}
        {hasPerm(user, 'view_notifications') && (
          <PageGrid.Item><NotificationsCard /></PageGrid.Item>
        )}
      </PageGrid>
    </Page>
  );
}
