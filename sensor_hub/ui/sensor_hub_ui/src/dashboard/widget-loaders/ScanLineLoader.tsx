import { Box } from '@mui/material';
import { alpha } from '@mui/material/styles';
import { useChartColours } from '../../theme/chartColours';
import { usePrefersReducedMotion } from './usePrefersReducedMotion';
import { keyframes } from '@mui/system';
import LoaderShell from './LoaderShell';

const scanMove = keyframes`
  0% { top: -26px; }
  100% { top: 100%; }
`;

interface ScanLineLoaderProps {
  rows?: number;
}

/**
 * Alternative table/list loader: a terracotta line sweeps down static skeleton
 * rows, like a reader pulling them in. Reduced-motion hides the moving line.
 */
export default function ScanLineLoader({ rows = 4 }: ScanLineLoaderProps) {
  const colours = useChartColours();
  const reduced = usePrefersReducedMotion();

  return (
    <LoaderShell>
      <Box sx={{ position: 'relative', width: '100%', height: '100%', overflow: 'hidden', display: 'flex', alignItems: 'center' }}>
        {!reduced && (
          <Box
            sx={{
              position: 'absolute',
              left: 0,
              right: 0,
              height: 26,
              background: `linear-gradient(${alpha(colours.categorical[0], 0.25)}, transparent)`,
              borderTop: `2px solid ${colours.categorical[0]}`,
              animation: `${scanMove} 2.2s ease-in-out infinite`,
            }}
          />
        )}
        <Box sx={{ width: '100%', display: 'flex', flexDirection: 'column', gap: 1.2, px: 1 }}>
          {Array.from({ length: rows }).map((_, i) => (
            <Box key={i} sx={{ display: 'flex', alignItems: 'center', gap: 1.2 }}>
              <Box sx={{ width: 20, height: 20, borderRadius: '50%', bgcolor: colours.noData, flex: 'none' }} />
              <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 0.6, minWidth: 0 }}>
                <Box sx={{ height: 9, borderRadius: 1, bgcolor: colours.noData, width: '70%' }} />
                <Box sx={{ height: 8, borderRadius: 1, bgcolor: colours.noData, width: '42%' }} />
              </Box>
            </Box>
          ))}
        </Box>
      </Box>
    </LoaderShell>
  );
}
