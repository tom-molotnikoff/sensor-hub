import type { ReactNode } from 'react';
import { Box } from '@mui/material';
import { ResponsiveContainer } from 'recharts';
import { useBounded } from './useBounded';
import { responsivePixels } from './tiers';
import { chartAreaHeight, type ChartAreaSize } from './theme/tokens';

interface ChartAreaProps {
  size: ChartAreaSize;
  children?: ReactNode;
}

export default function ChartArea({ size, children }: ChartAreaProps) {
  const bounded = useBounded();
  const height = chartAreaHeight[size];

  return (
    <Box
      data-ui="chart-area"
      data-ui-min-height={bounded ? undefined : Math.min(height.compact, height.wide)}
      sx={
        bounded
          ? { flex: '1 1 0', height: '100%', minHeight: 0, minWidth: 0 }
          : { height: responsivePixels(height), minWidth: 0 }
      }
    >
      <ResponsiveContainer width="100%" height="100%">
        {children}
      </ResponsiveContainer>
    </Box>
  );
}
