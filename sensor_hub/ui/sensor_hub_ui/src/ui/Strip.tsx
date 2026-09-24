import type { ReactNode } from 'react';
import { Box, Paper } from '@mui/material';

export type StripSize = 'sm' | 'md';

interface StripProps {
  label: string;
  size: StripSize;
  children?: ReactNode;
}

interface StripCellProps {
  surface?: 'raised' | 'outlined';
  highlighted?: boolean;
  children?: ReactNode;
}

interface StripDetailProps {
  children?: ReactNode;
}

export const stripNarrowWidth = 480;

const cells = {
  sm: {
    gap: 1,
    minWidth: 56,
    padding: 1,
    radius: 2,
    narrow: { minWidth: 48, paddingX: 0.75, paddingY: 0.75, radius: 3 },
  },
  md: {
    gap: 1.5,
    minWidth: 100,
    padding: 1.5,
    radius: 2,
    narrow: { minWidth: 72, paddingX: 1.5, paddingY: 1, radius: 3 },
  },
} as const;

export default function Strip({ label, size, children }: StripProps) {
  const cell = cells[size];

  return (
    <Box data-ui="strip" data-ui-size={size} sx={{ containerType: 'inline-size', minWidth: 0 }}>
      <Box
        data-ui="strip-track"
        role="region"
        aria-label={label}
        tabIndex={0}
        sx={{
          display: 'flex',
          gap: cell.gap,
          overflowX: 'auto',
          paddingBottom: 1,
          scrollbarWidth: 'thin',
          '&::-webkit-scrollbar': { height: 6 },
          '&::-webkit-scrollbar-thumb': { borderRadius: 3, bgcolor: 'action.disabled' },
          '& > [data-ui=strip-cell]': {
            flex: '1 1 0',
            minWidth: cell.minWidth,
            padding: cell.padding,
            borderRadius: cell.radius,
          },
          [`@container (max-width: ${stripNarrowWidth - 0.05}px)`]: {
            '& > [data-ui=strip-cell]': {
              flex: '0 0 auto',
              minWidth: cell.narrow.minWidth,
              paddingX: cell.narrow.paddingX,
              paddingY: cell.narrow.paddingY,
              borderRadius: cell.narrow.radius,
            },
            '& [data-ui=strip-detail]': { display: 'none' },
          },
        }}
      >
        {children}
      </Box>
    </Box>
  );
}

export function StripCell({ surface = 'raised', highlighted = false, children }: StripCellProps) {
  const raised = surface === 'raised';

  return (
    <Paper
      data-ui="strip-cell"
      aria-current={highlighted ? 'true' : undefined}
      variant={raised ? 'elevation' : 'outlined'}
      elevation={raised ? (highlighted ? 3 : 1) : undefined}
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: 0.5,
        boxSizing: 'border-box',
        textAlign: 'center',
        ...(highlighted && { border: 2, borderColor: 'primary.main' }),
      }}
    >
      {children}
    </Paper>
  );
}

export function StripDetail({ children }: StripDetailProps) {
  return (
    <Box data-ui="strip-detail" sx={{ display: 'contents' }}>
      {children}
    </Box>
  );
}
