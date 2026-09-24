import { Box } from '@mui/material';
import type { StatusKey } from '../theme';

export default function StatusPill({ status, label }: { status: StatusKey; label: string }) {
  return (
    <Box
      component="span"
      data-ui="status-pill"
      data-status={status}
      sx={{
        flexShrink: 0,
        paddingX: 1,
        paddingY: 0.25,
        borderRadius: 999,
        typography: 'bodySmall',
        color: `status.${status}.strong`,
        bgcolor: `status.${status}.soft`,
      }}
    >
      {label}
    </Box>
  );
}
