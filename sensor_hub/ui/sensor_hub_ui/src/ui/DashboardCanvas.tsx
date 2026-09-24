import type { ReactNode, Ref } from 'react';
import { Box } from '@mui/material';

const editingRoomBelow = 200;

interface DashboardCanvasProps {
  editing: boolean;
  ref?: Ref<HTMLDivElement>;
  children?: ReactNode;
}

export default function DashboardCanvas({ editing, ref, children }: DashboardCanvasProps) {
  return (
    <Box
      ref={ref}
      data-ui="dashboard-canvas"
      sx={{ minWidth: 0, paddingBottom: editing ? `${editingRoomBelow}px` : 0 }}
    >
      {children}
    </Box>
  );
}
