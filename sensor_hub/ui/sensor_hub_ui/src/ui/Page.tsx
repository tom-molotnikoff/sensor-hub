import type { ReactNode } from 'react';
import { Box, CircularProgress } from '@mui/material';
import NavigationSidebar from '../navigation/NavigationSidebar';
import TopAppBar from '../navigation/TopAppBar';
import { responsive, type TierValues } from './tiers';
import { density } from './theme/tokens';

interface PageProps {
  title: string;
  loading?: boolean;
  children?: ReactNode;
}

const pixels = (values: TierValues<number>) =>
  responsive({ compact: `${values.compact}px`, wide: `${values.wide}px` });

export default function Page({ title, loading = false, children }: PageProps) {
  return (
    <>
      <TopAppBar pageTitle={title} />
      <NavigationSidebar />
      <Box
        component="main"
        data-ui="page"
        sx={{ display: 'flex', flexDirection: 'column', padding: pixels(density.page), gap: 1 }}
      >
        {loading ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', padding: pixels(density.page) }}>
            <CircularProgress />
          </Box>
        ) : (
          children
        )}
      </Box>
    </>
  );
}
