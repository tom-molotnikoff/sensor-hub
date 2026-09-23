import Page from '../../ui/Page';
import { useAuth } from '../../providers/AuthContext';
import PropertiesPage from '../../components/PropertiesPage';

export default function PropertiesOverview() {
  const { user } = useAuth();

  return (
    <Page title="Properties Overview" loading={user === undefined}>
      <PropertiesPage />
    </Page>
  );
}
