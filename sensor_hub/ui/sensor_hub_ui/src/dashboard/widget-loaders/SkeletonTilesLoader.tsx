import { Box } from '@mui/material';
import { lighten } from '@mui/material/styles';
import { useChartColours } from '../../theme/chartColours';
import { usePrefersReducedMotion } from './usePrefersReducedMotion';
import { shimmer } from './keyframes';
import LoaderShell from './LoaderShell';

interface SkeletonTilesLoaderProps {
  tiles?: number;
}

/**
 * Loader for stat-tile widgets (min/max/avg). A calm label + value placeholder
 * per tile, with a shimmer sweeping the tiles in turn. Never shows numbers.
 */
export default function SkeletonTilesLoader({ tiles = 3 }: SkeletonTilesLoaderProps) {
  const colours = useChartColours();
  const reduced = usePrefersReducedMotion();

  const valueSx = reduced
    ? { bgcolor: colours.noData }
    : {
        background: `linear-gradient(90deg, ${colours.noData} 25%, ${lighten(colours.noData, 0.4)} 45%, ${colours.noData} 65%)`,
        backgroundSize: '220% 100%',
        animation: `${shimmer} 1.5s ease-in-out infinite`,
      };

  return (
    <LoaderShell>
      <Box sx={{ display: 'flex', gap: 1.5, width: '100%', alignItems: 'center', justifyContent: 'center' }}>
        {Array.from({ length: tiles }).map((_, i) => (
          <Box
            key={i}
            data-testid="stat-tile"
            sx={{
              flex: 1,
              maxWidth: 140,
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              gap: 1.2,
              p: 1.5,
              border: 1,
              borderColor: 'divider',
              borderRadius: 2,
            }}
          >
            <Box sx={{ width: '50%', height: 8, borderRadius: 1, bgcolor: colours.noData }} />
            <Box sx={{ width: '75%', height: 20, borderRadius: 1, animationDelay: `${(i * 0.25).toFixed(2)}s`, ...valueSx }} />
          </Box>
        ))}
      </Box>
    </LoaderShell>
  );
}
