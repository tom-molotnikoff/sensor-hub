import type { ReactNode } from 'react';
import { Box } from '@mui/material';
import { responsivePixels } from './tiers';
import { density, standalonePageWidth } from './theme/tokens';

interface StandalonePageProps {
  children?: ReactNode;
}

export default function StandalonePage({ children }: StandalonePageProps) {
  return (
    <Box
      component="main"
      data-ui="standalone-page"
      sx={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: responsivePixels(density.page),
        bgcolor: 'background.default',
      }}
    >
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'column',
          gap: responsivePixels(density.gap),
          width: '100%',
          maxWidth: standalonePageWidth,
          minWidth: 0,
        }}
      >
        {children}
      </Box>
    </Box>
  );
}
