import type { ReactNode } from 'react';
import { Box } from '@mui/material';
import { responsivePixels } from './tiers';
import { density } from './theme/tokens';

interface InlineProps {
  children?: ReactNode;
}

export default function Inline({ children }: InlineProps) {
  return (
    <Box
      data-ui="inline"
      sx={{
        display: 'flex',
        flexWrap: 'wrap',
        alignItems: 'center',
        gap: responsivePixels(density.gap),
        minWidth: 0,
      }}
    >
      {children}
    </Box>
  );
}
