import type { ReactNode } from 'react';
import { Box } from '@mui/material';
import { responsivePixels } from './tiers';
import { density } from './theme/tokens';

interface StackProps {
  children?: ReactNode;
}

export default function Stack({ children }: StackProps) {
  return (
    <Box
      data-ui="stack"
      sx={{ display: 'flex', flexDirection: 'column', gap: responsivePixels(density.gap), minWidth: 0 }}
    >
      {children}
    </Box>
  );
}
