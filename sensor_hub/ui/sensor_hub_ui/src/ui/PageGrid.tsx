import type { ReactNode } from 'react';
import { Box } from '@mui/material';
import { responsive, responsivePixels } from './tiers';
import { density } from './theme/tokens';

type Columns = 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12;

interface PageGridProps {
  equalHeight?: boolean;
  children?: ReactNode;
}

interface PageGridItemProps {
  span?: { wide: Columns };
  children?: ReactNode;
}

function PageGridItem({ span = { wide: 12 }, children }: PageGridItemProps) {
  return (
    <Box
      data-ui="page-grid-item"
      sx={{
        gridColumn: responsive({ compact: 'auto', wide: `span ${span.wide}` }),
        display: 'flex',
        flexDirection: 'column',
        minWidth: 0,
        '& > *': { flex: '1 1 auto' },
        '& > [data-ui=sticky]': { flex: 'none' },
      }}
    >
      {children}
    </Box>
  );
}

function PageGrid({ equalHeight = false, children }: PageGridProps) {
  return (
    <Box
      data-ui="page-grid"
      sx={{
        display: 'grid',
        gridTemplateColumns: responsive({ compact: 'minmax(0, 1fr)', wide: 'repeat(12, minmax(0, 1fr))' }),
        alignItems: responsive({ compact: 'start', wide: equalHeight ? 'stretch' : 'start' }),
        gap: responsivePixels(density.gap),
      }}
    >
      {children}
    </Box>
  );
}

PageGrid.Item = PageGridItem;

export default PageGrid;
