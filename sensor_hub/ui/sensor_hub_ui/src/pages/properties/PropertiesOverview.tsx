import PageContainer from '../../tools/PageContainer';
import { useAuth } from '../../providers/AuthContext';
import PropertiesPage from '../../components/PropertiesPage';

export default function PropertiesOverview() {
  const { user } = useAuth();

  return (
    <PageContainer titleText="Properties Overview" loading={user === undefined}>
      <PropertiesPage />
    </PageContainer>
  );
}
