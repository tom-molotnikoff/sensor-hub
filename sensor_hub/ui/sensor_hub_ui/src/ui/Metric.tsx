import type { ReactNode } from 'react';
import { Box, CircularProgress, Typography } from '@mui/material';
import { useBounded } from './useBounded';
import { responsivePixels } from './tiers';
import { theme } from './theme';
import { density, metricDialSize, metricFit, metricMinHeight } from './theme/tokens';

export type MetricSize = keyof typeof metricDialSize;

interface MetricDial {
  percent: number;
  tone: string;
}

interface MetricProps {
  value: number | string | null;
  unit?: string;
  label?: string;
  caption?: string;
  size?: MetricSize;
  tone?: string;
  dial?: MetricDial;
}

const variants = { sm: 'metricSm', md: 'metricMd', lg: 'metricLg' } as const;

function formatted(value: number | string | null) {
  if (value === null) return '—';
  return typeof value === 'number' ? value.toFixed(1) : value;
}

function fitted(fit: { md: string; lg: string }) {
  return {
    [`@container ${fit.md}`]: { '& [data-ui=metric-value]': { fontSize: theme.typography.metricMd.fontSize } },
    [`@container ${fit.lg}`]: { '& [data-ui=metric-value]': { fontSize: theme.typography.metricLg.fontSize } },
  };
}

const valueFit = fitted({
  md: `(min-width: ${metricFit.value.md.width}px) and (min-height: ${metricFit.value.md.height}px)`,
  lg: `(min-width: ${metricFit.value.lg.width}px) and (min-height: ${metricFit.value.lg.height}px)`,
});

const dialFit = fitted({
  md: `(min-width: ${metricFit.dial.md}px)`,
  lg: `(min-width: ${metricFit.dial.lg}px)`,
});

const fill = {
  flex: '1 1 0',
  minHeight: 0,
  alignSelf: 'stretch',
  containerType: 'size',
  display: 'flex',
  justifyContent: 'center',
  alignItems: 'center',
} as const;

function Value({ value, unit, tone, size }: { value: string; unit?: string; tone?: string; size: MetricSize }) {
  return (
    <Typography variant={variants[size]} data-ui="metric-value" color={tone} noWrap>
      {value}
      {unit && (
        <Typography component="span" variant="body" color="text.secondary" sx={{ fontWeight: 'fontWeightMedium', marginLeft: '2px' }}>
          {unit}
        </Typography>
      )}
    </Typography>
  );
}

function Dial({ dial, bounded, size, children }: { dial: MetricDial; bounded: boolean; size: MetricSize; children: ReactNode }) {
  const side = bounded ? 'min(100cqw, 100cqh)' : `${metricDialSize[size]}px`;
  return (
    <Box
      data-ui="metric-dial"
      sx={{
        position: 'relative',
        width: side,
        height: side,
        flex: 'none',
        containerType: 'size',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        ...(bounded && dialFit),
      }}
    >
      <CircularProgress
        variant="determinate"
        value={dial.percent}
        size="100%"
        thickness={6}
        sx={{ position: 'absolute', inset: 0, transform: 'rotate(-90deg) !important', color: dial.tone }}
      />
      {children}
    </Box>
  );
}

export default function Metric({ value, unit, label, caption, size = 'md', tone, dial }: MetricProps) {
  const bounded = useBounded();
  const text = formatted(value);
  const shown = <Value value={text} unit={unit} tone={tone} size={bounded ? 'sm' : size} />;

  return (
    <Box
      data-ui="metric"
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        textAlign: 'center',
        gap: 0.5,
        minWidth: 0,
        ...(bounded && {
          flex: '1 1 0',
          height: '100%',
          minHeight: metricMinHeight,
          padding: responsivePixels(density.card),
          boxSizing: 'border-box',
        }),
      }}
    >
      {dial ? (
        <Box sx={bounded ? fill : undefined}>
          <Dial dial={dial} bounded={bounded} size={size}>
            {shown}
          </Dial>
        </Box>
      ) : (
        <Box sx={bounded ? { ...fill, ...valueFit } : undefined}>
          {shown}
        </Box>
      )}
      {label && (
        <Typography variant="bodySmall" color="text.secondary">
          {label}
        </Typography>
      )}
      {caption && (
        <Typography variant="caption" color="text.secondary" noWrap>
          {caption}
        </Typography>
      )}
    </Box>
  );
}

interface MetricGroupProps {
  children?: ReactNode;
}

export function MetricGroup({ children }: MetricGroupProps) {
  const bounded = useBounded();
  return (
    <Box
      data-ui="metric-group"
      sx={{
        display: 'grid',
        gridAutoFlow: 'column',
        gridAutoColumns: 'minmax(0, 1fr)',
        gap: responsivePixels(density.gap),
        minWidth: 0,
        ...(bounded && { height: '100%', flex: '1 1 0', minHeight: metricMinHeight }),
      }}
    >
      {children}
    </Box>
  );
}
