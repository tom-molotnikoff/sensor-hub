import { Box } from '@mui/material';
import { useChartColours } from '../../theme/chartColours';
import { usePrefersReducedMotion } from './usePrefersReducedMotion';
import { cascade } from './keyframes';
import LoaderShell from './LoaderShell';

interface WeatherColumnsLoaderProps {
  days?: number;
}

/**
 * Loader for the weather forecast card: skeleton day columns (label, icon disc,
 * hi/lo) cascade in left-to-right, so the forecast visibly "arrives" day by day.
 */
export default function WeatherColumnsLoader({ days = 6 }: WeatherColumnsLoaderProps) {
  const colours = useChartColours();
  const reduced = usePrefersReducedMotion();

  return (
    <LoaderShell>
      <Box sx={{ display: 'flex', gap: 1.5, alignItems: 'center', justifyContent: 'center', width: '100%', overflow: 'hidden' }}>
        {Array.from({ length: days }).map((_, i) => (
          <Box
            key={i}
            data-testid="wx-day"
            sx={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              gap: 0.9,
              ...(reduced
                ? {}
                : { opacity: 0, animation: `${cascade} 2.8s ease-in-out infinite`, animationDelay: `${(i * 0.14).toFixed(2)}s` }),
            }}
          >
            <Box sx={{ width: 26, height: 8, borderRadius: 1, bgcolor: colours.noData }} />
            <Box sx={{ width: 28, height: 28, borderRadius: '50%', bgcolor: colours.noData }} />
            <Box sx={{ width: 24, height: 9, borderRadius: 1, bgcolor: colours.noData }} />
            <Box sx={{ width: 18, height: 8, borderRadius: 1, bgcolor: colours.noData }} />
          </Box>
        ))}
      </Box>
    </LoaderShell>
  );
}
