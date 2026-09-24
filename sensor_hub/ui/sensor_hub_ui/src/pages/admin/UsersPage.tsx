import Page from '../../ui/Page';
import PageGrid from '../../ui/PageGrid';
import { useAuth } from '../../providers/AuthContext';
import { hasPerm } from '../../tools/Utils';
import UserManagementCard from '../../components/UserManagementCard';
import RolePermissionsCard from '../../components/RolePermissionsCard';

export default function UsersPage() {
  const { user } = useAuth();

  return (
    <Page title="User Management" loading={user === undefined}>
      <PageGrid>
        {hasPerm(user, 'view_users') && (
          <PageGrid.Item><UserManagementCard /></PageGrid.Item>
        )}
        {hasPerm(user, 'view_roles') && (
          <PageGrid.Item><RolePermissionsCard /></PageGrid.Item>
        )}
      </PageGrid>
    </Page>
  );
}
