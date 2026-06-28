import { Box } from '@mui/material';
import { useChartColours } from '../../theme/chartColours';
import { usePrefersReducedMotion } from './usePrefersReducedMotion';
import { cascade } from './keyframes';
import LoaderShell from './LoaderShell';

interface CascadeRowsLoaderProps {
  rows?: number;
}

/**
 * Loader for table / list widgets: skeleton rows stream in top-to-bottom on a
 * loop, like records arriving. Reduced-motion shows the rows statically.
 */
export default function CascadeRowsLoader({ rows = 4 }: CascadeRowsLoaderProps) {
  const colours = useChartColours();
  const reduced = usePrefersReducedMotion();

  return (
    <LoaderShell>
      <Box sx={{ width: '100%', display: 'flex', flexDirection: 'column', gap: 1.2, px: 1 }}>
        {Array.from({ length: rows }).map((_, i) => (
          <Box
            key={i}
            data-testid="cascade-row"
            sx={{
              display: 'flex',
              alignItems: 'center',
              gap: 1.2,
              ...(reduced
                ? {}
                : {
                    opacity: 0,
                    animation: `${cascade} 2.6s ease-in-out infinite`,
                    animationDelay: `${(i * 0.18).toFixed(2)}s`,
                  }),
            }}
          >
            <Box sx={{ width: 20, height: 20, borderRadius: '50%', bgcolor: colours.noData, flex: 'none' }} />
            <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 0.6, minWidth: 0 }}>
              <Box sx={{ height: 9, borderRadius: 1, bgcolor: colours.noData, width: '70%' }} />
              <Box sx={{ height: 8, borderRadius: 1, bgcolor: colours.noData, width: '42%' }} />
            </Box>
          </Box>
        ))}
      </Box>
    </LoaderShell>
  );
}
