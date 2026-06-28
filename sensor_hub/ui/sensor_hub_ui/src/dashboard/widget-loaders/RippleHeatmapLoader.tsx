import { Box } from '@mui/material';
import { useChartColours } from '../../theme/chartColours';
import { usePrefersReducedMotion } from './usePrefersReducedMotion';
import { ripple } from './keyframes';
import LoaderShell from './LoaderShell';

interface RippleHeatmapLoaderProps {
  /** Columns in the grid — matches the real heatmap (7). */
  columns?: number;
  /** Total cells — matches the real heatmap (30 days). */
  count?: number;
}

/**
 * Loader for the heatmap widget: a diagonal wave of warmth sweeps across the
 * grid as if days of data are filling in. Mirrors the real 7-column / 30-day
 * grid. No day numbers are shown (never fake a value).
 */
export default function RippleHeatmapLoader({ columns = 7, count = 30 }: RippleHeatmapLoaderProps) {
  const colours = useChartColours();
  const reduced = usePrefersReducedMotion();

  return (
    <LoaderShell>
      <Box
        sx={{
          width: '100%',
          maxWidth: 260,
          display: 'grid',
          gridTemplateColumns: `repeat(${columns}, 1fr)`,
          gridAutoRows: '1fr',
          gap: '4px',
          '--cell-base': colours.noData,
          '--cell-active': colours.categorical[0],
        }}
      >
        {Array.from({ length: count }).map((_, i) => {
          const row = Math.floor(i / columns);
          const col = i % columns;
          return (
            <Box
              key={i}
              data-testid="heatmap-cell"
              sx={{
                aspectRatio: '1 / 1',
                borderRadius: '2px',
                backgroundColor: 'var(--cell-base)',
                ...(reduced
                  ? {}
                  : {
                      animation: `${ripple} 2.6s ease-in-out infinite`,
                      animationDelay: `${((row + col) * 0.09).toFixed(2)}s`,
                    }),
              }}
            />
          );
        })}
      </Box>
    </LoaderShell>
  );
}
