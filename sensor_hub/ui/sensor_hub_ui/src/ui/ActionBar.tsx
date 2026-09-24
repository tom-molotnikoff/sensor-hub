import type { ReactNode } from 'react';
import { Box } from '@mui/material';

const pickerMinWidth = 200;

interface ActionBarProps {
  picker?: ReactNode;
  trailing?: ReactNode;
  children?: ReactNode;
}

const row = { display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 1, minWidth: 0 } as const;

export default function ActionBar({ picker, trailing, children }: ActionBarProps) {
  return (
    <Box data-ui="action-bar" sx={{ ...row, '& > *': { maxWidth: '100%' } }}>
      {picker && (
        <Box
          data-ui="action-bar-picker"
          sx={{ display: 'flex', minWidth: `min(${pickerMinWidth}px, 100%)`, '& > *': { flex: '1 1 auto', minWidth: 0 } }}
        >
          {picker}
        </Box>
      )}
      {children}
      {trailing && (
        <Box data-ui="action-bar-trailing" sx={{ ...row, marginLeft: 'auto' }}>
          {trailing}
        </Box>
      )}
    </Box>
  );
}
