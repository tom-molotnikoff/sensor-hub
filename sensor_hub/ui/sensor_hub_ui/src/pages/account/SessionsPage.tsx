import Page from '../../ui/Page';
import PageGrid from '../../ui/PageGrid';
import SessionsCard from '../../components/SessionsCard';

export default function SessionsPage() {
  return (
    <Page title="Active sessions">
      <PageGrid>
        <PageGrid.Item><SessionsCard /></PageGrid.Item>
      </PageGrid>
    </Page>
  );
}
