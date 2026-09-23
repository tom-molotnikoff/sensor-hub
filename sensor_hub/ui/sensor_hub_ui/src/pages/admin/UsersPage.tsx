import Page from '../../ui/Page';
import { useAuth } from '../../providers/AuthContext';
import { hasPerm } from '../../tools/Utils';
import { Grid } from '@mui/material';
import UserManagementCard from '../../components/UserManagementCard';
import RolePermissionsCard from '../../components/RolePermissionsCard';

export default function UsersPage() {
  const { user } = useAuth();

  return (
    <Page title="User Management" loading={user === undefined}>
      <Grid container spacing={2}>
        {hasPerm(user, 'view_users') && (
          <Grid size={12}><UserManagementCard /></Grid>
        )}
        {hasPerm(user, 'view_roles') && (
          <Grid size={12}><RolePermissionsCard /></Grid>
        )}
      </Grid>
    </Page>
  );
}
