import { Box } from '@mui/material';
import { useChartColours } from '../../theme/chartColours';
import { usePrefersReducedMotion } from './usePrefersReducedMotion';
import { drawRing } from './keyframes';
import LoaderShell from './LoaderShell';

/**
 * Loader for radial widgets — gauge, pie, donut, health. A terracotta ring
 * draws itself around a calm grey track and erases, a looped cousin of the
 * chart signal trace. One radial motif shared by gauge and pie.
 */
export default function CircularDrawLoader() {
  const colours = useChartColours();
  const reduced = usePrefersReducedMotion();

  return (
    <LoaderShell>
      <Box
        component="svg"
        viewBox="0 0 100 100"
        sx={{ width: '70%', maxWidth: 130, height: 'auto', aspectRatio: '1 / 1' }}
      >
        <g transform="rotate(-90 50 50)">
          <circle cx="50" cy="50" r="40" fill="none" stroke={colours.noData} strokeWidth={9} />
          <Box
            component="circle"
            cx="50"
            cy="50"
            r="40"
            fill="none"
            stroke={colours.categorical[0]}
            strokeWidth={9}
            strokeLinecap="round"
            sx={{
              strokeDasharray: 251.3,
              ...(reduced ? { strokeDashoffset: 0 } : { animation: `${drawRing} 2.4s ease-in-out infinite` }),
            }}
          />
        </g>
      </Box>
    </LoaderShell>
  );
}
