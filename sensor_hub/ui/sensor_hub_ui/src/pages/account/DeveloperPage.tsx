import Page from '../../ui/Page';
import PageGrid from '../../ui/PageGrid';
import ApiKeysCard from '../../components/ApiKeysCard';
import ApiReferenceCard from '../../components/ApiReferenceCard';
import { useApiKeys } from '../../hooks/useApiKeys';
import { useAuth } from '../../providers/AuthContext';
import { hasPerm } from '../../tools/Utils';

export default function DeveloperPage() {
  const { apiKeys, loaded, refresh } = useApiKeys();
  const { user } = useAuth();

  return (
    <Page title="Developer">
      <PageGrid>
        {hasPerm(user, 'manage_api_keys') && (
          <PageGrid.Item><ApiKeysCard apiKeys={apiKeys} loaded={loaded} onRefresh={refresh} /></PageGrid.Item>
        )}
        {hasPerm(user, 'view_api_docs') && (
          <PageGrid.Item><ApiReferenceCard /></PageGrid.Item>
        )}
      </PageGrid>
    </Page>
  );
}
