import { Grid } from '@mui/material';
import Page from '../../ui/Page';
import { useAuth } from '../../providers/AuthContext';
import { hasPerm } from '../../tools/Utils';
import DataRetentionCard from '../../components/DataRetentionCard';

function DataRetentionPage() {
  const { user } = useAuth();

  return (
    <Page title="Data Retention" loading={user === undefined}>
      <Grid container spacing={2}>
        {hasPerm(user, 'view_sensors') && (
          <Grid size={12}><DataRetentionCard /></Grid>
        )}
      </Grid>
    </Page>
  );
}

export default DataRetentionPage;
