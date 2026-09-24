import type { ReactNode } from 'react';
import { Box, Typography } from '@mui/material';

export interface Stat {
  key: string | number;
  label: ReactNode;
  value: ReactNode;
}

interface StatGridProps {
  stats: Stat[];
}

const statMinWidth = 140;

export default function StatGrid({ stats }: StatGridProps) {
  return (
    <Box
      data-ui="stat-grid"
      sx={{
        display: 'grid',
        gridTemplateColumns: `repeat(auto-fill, minmax(min(${statMinWidth}px, 100%), 1fr))`,
        gap: 1,
        minWidth: 0,
      }}
    >
      {stats.map((stat) => (
        <Box
          key={stat.key}
          data-ui="stat"
          sx={{
            minWidth: 0,
            padding: 1.5,
            textAlign: 'center',
            bgcolor: 'background.paper',
            border: 1,
            borderColor: 'divider',
            borderRadius: 1,
          }}
        >
          <Typography variant="caption" component="div" color="text.secondary">
            {stat.label}
          </Typography>
          <Typography variant="h6" component="div" sx={{ fontWeight: 'fontWeightBold' }}>
            {stat.value}
          </Typography>
        </Box>
      ))}
    </Box>
  );
}
