import type { ReactNode, Ref } from 'react';
import { Box, Skeleton, Typography } from '@mui/material';
import DragIndicatorIcon from '@mui/icons-material/DragIndicator';
import Bounded from './Bounded';
import Stack from './Stack';

interface FrameProps {
  title?: string;
  actions?: ReactNode;
  editing?: boolean;
  dragHandle?: boolean;
  state?: string;
  ref?: Ref<HTMLDivElement>;
  children?: ReactNode;
}

export default function Frame({ title, actions, editing = false, dragHandle = false, state, ref, children }: FrameProps) {
  const grabbable = editing && dragHandle;

  return (
    <Box
      ref={ref}
      data-ui="frame"
      data-widget-state={state}
      sx={{
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
        position: 'relative',
        bgcolor: 'background.paper',
        border: 1,
        borderStyle: editing ? 'dashed' : 'solid',
        borderColor: editing ? 'primary.main' : 'divider',
        borderRadius: 2,
        boxShadow: 'none',
        userSelect: editing ? 'none' : 'auto',
      }}
    >
      {(title || actions) && (
        <Box
          data-ui="frame-header"
          className={grabbable ? 'drag-handle' : undefined}
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 1,
            paddingX: 1.5,
            paddingY: 0.5,
            borderBottom: 1,
            borderColor: 'divider',
            flexShrink: 0,
            ...(editing && { bgcolor: 'action.hover' }),
            ...(grabbable && { cursor: 'grab' }),
          }}
        >
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, minWidth: 0 }}>
            {grabbable && <DragIndicatorIcon fontSize="small" color="action" />}
            {title && (
              <Typography variant="caption" color="text.secondary" noWrap>
                {title}
              </Typography>
            )}
          </Box>
          {actions && <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, flexShrink: 0 }}>{actions}</Box>}
        </Box>
      )}
      <Box
        data-ui="frame-body"
        sx={{
          flex: '1 1 auto',
          minHeight: 0,
          overflow: 'hidden',
          padding: editing ? 1 : 0,
          '& > *': { height: '100%', width: '100%' },
        }}
      >
        <Bounded>{children}</Bounded>
      </Box>
    </Box>
  );
}

export function FramePlaceholder({ label }: { label: string }) {
  return (
    <Box
      data-ui="frame-placeholder"
      sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 1, padding: 2, opacity: 0.5 }}
    >
      <Typography variant="body2" color="text.secondary">
        {label}
      </Typography>
      <Box sx={{ width: '80%' }}>
        <Stack>
          <Skeleton variant="rectangular" height={8} />
          <Skeleton variant="rectangular" height={8} width="60%" />
          <Skeleton variant="rectangular" height={8} width="40%" />
        </Stack>
      </Box>
    </Box>
  );
}
