import type { ReactElement, ReactNode } from 'react';
import { Box, CircularProgress, Typography } from '@mui/material';
import AppNav from '../navigation/AppNav';
import TopAppBar from '../navigation/TopAppBar';
import { responsive, responsivePixels, useTier } from './tiers';
import { density } from './theme/tokens';

interface PageProps {
  title: string;
  titleElement?: ReactElement;
  actions?: ReactNode;
  loading?: boolean;
  children?: ReactNode;
}

function PageHeader({ title, actions }: { title: ReactNode; actions?: ReactNode }) {
  return (
    <Box data-ui="page-header" sx={{ display: 'flex', alignItems: 'center', gap: 2, paddingBottom: 1 }}>
      <Typography variant="pageTitle" noWrap data-ui="page-title" sx={{ flex: '1 1 auto', minWidth: 0 }}>
        {title}
      </Typography>
      <Box data-ui="page-actions" sx={{ display: 'flex', alignItems: 'center', gap: 1, flexShrink: 0 }}>
        {actions}
      </Box>
    </Box>
  );
}

function PageMain({ header, loading, children }: { header?: ReactNode; loading: boolean; children?: ReactNode }) {
  return (
    <Box
      component="main"
      data-ui="page"
      sx={{
        display: 'flex',
        flexDirection: 'column',
        flex: '1 1 auto',
        minWidth: 0,
        padding: responsivePixels(density.page),
        gap: 1,
      }}
    >
      {header}
      {loading ? (
        <Box sx={{ display: 'flex', justifyContent: 'center', padding: responsivePixels(density.page) }}>
          <CircularProgress />
        </Box>
      ) : (
        children
      )}
    </Box>
  );
}

export default function Page({ title, titleElement, actions, loading = false, children }: PageProps) {
  const wide = useTier() === 'wide';

  return (
    <Box data-ui="shell" sx={{ display: responsive({ compact: 'block', wide: 'flex' }) }}>
      {!wide && <TopAppBar pageTitle={title} />}
      <AppNav permanent={wide} />
      <PageMain header={wide && <PageHeader title={titleElement ?? title} actions={actions} />} loading={loading}>
        {children}
      </PageMain>
    </Box>
  );
}
