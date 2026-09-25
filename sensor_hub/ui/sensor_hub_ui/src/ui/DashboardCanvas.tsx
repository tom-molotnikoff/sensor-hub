import type { ReactNode, Ref } from 'react';
import { Box } from '@mui/material';

const editingRoomBelow = 200;

interface DashboardCanvasProps {
  editing: boolean;
  tracking?: boolean;
  ref?: Ref<HTMLDivElement>;
  children?: ReactNode;
}

export default function DashboardCanvas({ editing, tracking = false, ref, children }: DashboardCanvasProps) {
  return (
    <Box
      ref={ref}
      data-ui="dashboard-canvas"
      sx={{
        minWidth: 0,
        paddingBottom: editing ? `${editingRoomBelow}px` : 0,
        ...(tracking && { '& .react-grid-item': { transition: 'none' } }),
      }}
    >
      {children}
    </Box>
  );
}
