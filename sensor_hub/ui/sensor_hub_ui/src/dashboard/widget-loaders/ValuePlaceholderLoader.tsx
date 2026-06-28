import { Box, Typography } from '@mui/material';
import { lighten } from '@mui/material/styles';
import { useChartColours } from '../../theme/chartColours';
import { usePrefersReducedMotion } from './usePrefersReducedMotion';
import { shimmer, pulse } from './keyframes';
import LoaderShell from './LoaderShell';

interface ValuePlaceholderLoaderProps {
  /**
   * Unit to anchor next to the placeholder. Resolve this from the widget's
   * configured measurement type — never guess. Omit when it cannot be resolved.
   */
  unit?: string;
  variant?: 'block' | 'dashes';
}

/**
 * Loader for big-number / current-reading widgets. Never renders a real or
 * invented value — only the shape of one (a shimmer block or em-dashes), with
 * an optional dynamic unit anchored alongside.
 */
export default function ValuePlaceholderLoader({ unit, variant = 'block' }: ValuePlaceholderLoaderProps) {
  const colours = useChartColours();
  const reduced = usePrefersReducedMotion();

  if (variant === 'dashes') {
    return (
      <LoaderShell>
        <Typography
          component="div"
          sx={{
            fontSize: '3rem',
            fontWeight: 'bold',
            color: 'text.secondary',
            letterSpacing: 2,
            ...(reduced ? {} : { animation: `${pulse} 1.6s ease-in-out infinite` }),
          }}
        >
          —.—
          {unit ? (
            <Box component="span" sx={{ fontSize: '1.4rem', ml: 0.5 }}>
              {unit}
            </Box>
          ) : null}
        </Typography>
      </LoaderShell>
    );
  }

  return (
    <LoaderShell>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 1 }}>
        <Box
          sx={{
            width: 130,
            maxWidth: '64%',
            height: 44,
            borderRadius: 1,
            bgcolor: colours.noData,
            ...(reduced
              ? {}
              : {
                  background: `linear-gradient(90deg, ${colours.noData} 25%, ${lighten(colours.noData, 0.4)} 45%, ${colours.noData} 65%)`,
                  backgroundSize: '220% 100%',
                  animation: `${shimmer} 1.5s ease-in-out infinite`,
                }),
          }}
        />
        {unit ? (
          <Typography component="span" sx={{ fontSize: '1.4rem', fontWeight: 700, color: 'text.secondary' }}>
            {unit}
          </Typography>
        ) : null}
      </Box>
    </LoaderShell>
  );
}
