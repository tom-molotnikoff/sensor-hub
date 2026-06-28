import { Box } from '@mui/material';
import { useChartColours } from '../../theme/chartColours';
import { usePrefersReducedMotion } from './usePrefersReducedMotion';
import { cascade } from './keyframes';
import LoaderShell from './LoaderShell';

interface SensorDetailTilesLoaderProps {
  tiles?: number;
}

/**
 * Loader for the sensor-detail card: the responsive stat-paper grid rendered as
 * cascading skeleton tiles (label + value placeholder). No values are invented.
 */
export default function SensorDetailTilesLoader({ tiles = 6 }: SensorDetailTilesLoaderProps) {
  const colours = useChartColours();
  const reduced = usePrefersReducedMotion();

  return (
    <LoaderShell>
      <Box sx={{ width: '100%', display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 1, alignContent: 'center' }}>
        {Array.from({ length: tiles }).map((_, i) => (
          <Box
            key={i}
            data-testid="detail-tile"
            sx={{
              border: 1,
              borderColor: 'divider',
              borderRadius: 2,
              p: 1.2,
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              gap: 0.9,
              ...(reduced
                ? {}
                : { opacity: 0, animation: `${cascade} 2.8s ease-in-out infinite`, animationDelay: `${(i * 0.12).toFixed(2)}s` }),
            }}
          >
            <Box sx={{ width: '64%', height: 7, borderRadius: 1, bgcolor: colours.noData }} />
            <Box sx={{ width: '84%', height: 14, borderRadius: 1, bgcolor: colours.noData }} />
          </Box>
        ))}
      </Box>
    </LoaderShell>
  );
}
