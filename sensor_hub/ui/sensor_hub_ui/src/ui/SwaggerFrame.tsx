import type { ReactNode } from 'react';
import { Box } from '@mui/material';
import { swaggerDark, swaggerDarkClass, swaggerLight } from './theme/swagger';
import { useIsDark } from './theme/useIsDark';

export default function SwaggerFrame({ children }: { children?: ReactNode }) {
  const isDark = useIsDark();

  return (
    <Box
      data-ui="swagger-frame"
      className={isDark ? swaggerDarkClass : undefined}
      sx={{
        minWidth: 0,
        overflowX: 'auto',
        borderRadius: 1,
        ...swaggerLight,
        ...swaggerDark,
      }}
    >
      {children}
    </Box>
  );
}
