import Page from '../../ui/Page';
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
      {hasPerm(user, 'manage_api_keys') && (
        <ApiKeysCard apiKeys={apiKeys} loaded={loaded} onRefresh={refresh} />
      )}
      {hasPerm(user, 'view_api_docs') && (
        <ApiReferenceCard />
      )}
    </Page>
  );
}
