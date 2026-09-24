import Page from '../../ui/Page';
import PageGrid from '../../ui/PageGrid';
import ChangePasswordCard from '../../components/ChangePasswordCard';

export default function ChangePasswordPage() {
  return (
    <Page title="Change password">
      <PageGrid>
        <PageGrid.Item span={{ wide: 6 }}><ChangePasswordCard /></PageGrid.Item>
      </PageGrid>
    </Page>
  );
}
