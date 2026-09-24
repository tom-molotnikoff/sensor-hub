import Page from '../../ui/Page';
import { useAuth } from '../../providers/AuthContext';
import { hasPerm } from '../../tools/Utils';
import DataRetentionCard from '../../components/DataRetentionCard';

function DataRetentionPage() {
  const { user } = useAuth();

  return (
    <Page title="Data Retention" loading={user === undefined}>
      {hasPerm(user, 'view_sensors') && <DataRetentionCard />}
    </Page>
  );
}

export default DataRetentionPage;
