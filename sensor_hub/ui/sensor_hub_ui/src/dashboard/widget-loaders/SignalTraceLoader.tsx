import { Box } from '@mui/material';
import { useChartColours } from '../../theme/chartColours';
import { usePrefersReducedMotion } from './usePrefersReducedMotion';
import { draw } from './keyframes';
import LoaderShell from './LoaderShell';

const PATH = 'M0,80 L24,64 L48,72 L72,46 L96,56 L120,32 L144,48 L168,26 L192,40 L216,20 L240,16';

/**
 * Loader for line/area chart widgets: a terracotta line that draws itself
 * across the chart on a loop, like a reading streaming in. The hero motif of
 * the Incoming family.
 */
export default function SignalTraceLoader() {
  const colours = useChartColours();
  const reduced = usePrefersReducedMotion();

  return (
    <LoaderShell label="Loading chart data">
      <Box
        component="svg"
        viewBox="-6 -6 252 132"
        preserveAspectRatio="none"
        sx={{ width: '100%', height: '100%', overflow: 'hidden' }}
      >
        <line x1="0" y1="40" x2="240" y2="40" stroke={colours.grid} strokeWidth={1} />
        <line x1="0" y1="80" x2="240" y2="80" stroke={colours.grid} strokeWidth={1} />
        <Box
          component="path"
          d={PATH}
          fill="none"
          stroke={colours.categorical[0]}
          strokeWidth={2.5}
          strokeLinecap="round"
          strokeLinejoin="round"
          sx={{
            strokeDasharray: 320,
            ...(reduced ? { strokeDashoffset: 0 } : { animation: `${draw} 2.6s linear infinite` }),
          }}
        />
      </Box>
    </LoaderShell>
  );
}
