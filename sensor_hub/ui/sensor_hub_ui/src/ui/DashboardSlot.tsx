import type { ReactNode } from 'react';
import { Box } from '@mui/material';

interface DashboardSlotProps {
  height: number | 'content';
  children?: ReactNode;
}

export default function DashboardSlot({ height, children }: DashboardSlotProps) {
  const fixed = height !== 'content';
  return (
    <Box
      data-ui="dashboard-slot"
      data-ui-height={fixed ? height : undefined}
      sx={{ display: 'flex', flexDirection: 'column', minWidth: 0, ...(fixed && { height }) }}
    >
      {children}
    </Box>
  );
}
