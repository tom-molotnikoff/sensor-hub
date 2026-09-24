import { Box } from '@mui/material';
import type { ReactNode } from 'react';

interface LoaderShellProps {
  children: ReactNode;
  /** Accessible label announced while loading (the family is otherwise text-free). */
  label?: string;
}

/**
 * Common wrapper for every Incoming-family loader: fills the widget body,
 * centres the motif, and exposes the loading state to assistive tech via
 * role="status" + aria-busy (since the visuals carry no literal text).
 */
export default function LoaderShell({ children, label = 'Loading data' }: LoaderShellProps) {
  return (
    <Box
      role="status"
      aria-busy="true"
      aria-label={label}
      data-testid="widget-loader"
      sx={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        height: '100%',
        width: '100%',
        p: 1,
        boxSizing: 'border-box',
        overflow: 'hidden',
      }}
    >
      {children}
    </Box>
  );
}
