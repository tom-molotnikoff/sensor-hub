import type { ReactElement, ReactNode, Ref } from 'react';
import { Box, CircularProgress, Typography } from '@mui/material';
import { useMeasuredHeight } from '../hooks/useMeasuredHeight';
import AppNav from '../navigation/AppNav';
import TopAppBar from '../navigation/TopAppBar';
import { PageHeaderHeightContext, pageHeaderHeightVar } from './stickyTop';
import { responsive, responsivePixels, useTier } from './tiers';
import { density } from './theme/tokens';

interface PageProps {
  title: string;
  titleElement?: ReactElement;
  beforeTitle?: ReactNode;
  actions?: ReactNode;
  pinnedHeader?: boolean;
  loading?: boolean;
  children?: ReactNode;
}

interface PageHeaderProps {
  title: string;
  titleElement?: ReactElement;
  beforeTitle?: ReactNode;
  actions?: ReactNode;
  pinned: boolean;
  ref?: Ref<HTMLDivElement>;
}

const mainGap = 1;
const headerPadding = 1;

const headerGroup = { display: 'flex', alignItems: 'center', gap: 1, flexShrink: 0 } as const;

const pinnedSx = {
  position: 'sticky',
  top: 0,
  zIndex: 'appBar',
  marginTop: `-${density.page.wide}px`,
  paddingTop: `${density.page.wide}px`,
  paddingBottom: headerPadding + mainGap,
  marginBottom: -mainGap,
  bgcolor: 'background.default',
} as const;

function PageHeader({ title, titleElement, beforeTitle, actions, pinned, ref }: PageHeaderProps) {
  return (
    <Box
      ref={ref}
      data-ui="page-header"
      sx={{ display: 'flex', alignItems: 'center', gap: 2, paddingBottom: headerPadding, ...(pinned && pinnedSx) }}
    >
      {beforeTitle && (
        <Box data-ui="page-before-title" sx={headerGroup}>
          {beforeTitle}
        </Box>
      )}
      <Typography
        variant="pageTitle"
        noWrap={!titleElement}
        data-ui="page-title"
        sx={{ flex: '0 1 auto', minWidth: 0, ...(titleElement && { display: 'flex' }) }}
      >
        {titleElement ?? title}
      </Typography>
      <Box data-ui="page-actions" sx={{ ...headerGroup, marginLeft: 'auto' }}>
        {actions}
      </Box>
    </Box>
  );
}

interface PageMainProps {
  header?: ReactNode;
  headerHeight: number;
  loading: boolean;
  children?: ReactNode;
}

function PageMain({ header, headerHeight, loading, children }: PageMainProps) {
  return (
    <Box
      component="main"
      data-ui="page"
      sx={{
        ...(headerHeight > 0 && { [pageHeaderHeightVar]: `${headerHeight}px` }),
        display: 'flex',
        flexDirection: 'column',
        flex: '1 1 auto',
        minWidth: 0,
        padding: responsivePixels(density.page),
        gap: mainGap,
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

export default function Page({ title, titleElement, beforeTitle, actions, pinnedHeader = false, loading = false, children }: PageProps) {
  const wide = useTier() === 'wide';
  const { measuredRef, height } = useMeasuredHeight();
  const pinned = wide && pinnedHeader;
  const headerHeight = pinned ? height : 0;

  return (
    <Box data-ui="shell" sx={{ display: responsive({ compact: 'block', wide: 'flex' }) }}>
      {!wide && <TopAppBar pageTitle={title} />}
      <AppNav permanent={wide} />
      <PageHeaderHeightContext.Provider value={headerHeight}>
        <PageMain
          header={
            wide && (
              <PageHeader
                ref={pinned ? measuredRef : undefined}
                pinned={pinned}
                title={title}
                titleElement={titleElement}
                beforeTitle={beforeTitle}
                actions={actions}
              />
            )
          }
          headerHeight={headerHeight}
          loading={loading}
        >
          {children}
        </PageMain>
      </PageHeaderHeightContext.Provider>
    </Box>
  );
}
