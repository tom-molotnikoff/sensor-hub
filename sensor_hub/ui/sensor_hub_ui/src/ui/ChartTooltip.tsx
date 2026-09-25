import type { ReactNode } from 'react';
import { Box, Typography } from '@mui/material';
import type { TooltipContentProps, TooltipPayloadEntry } from 'recharts';

type ChartTooltipProps = Partial<Pick<TooltipContentProps, 'active' | 'payload' | 'label' | 'formatter'>>;

interface Row {
  colour?: string;
  name: ReactNode;
  value: ReactNode;
}

const swatchSize = 10;

const timeFormat: Intl.DateTimeFormatOptions = {
  weekday: 'short',
  day: 'numeric',
  month: 'short',
  hour: '2-digit',
  minute: '2-digit',
};

function rowsOf(payload: readonly TooltipPayloadEntry[], formatter: ChartTooltipProps['formatter']): Row[] {
  return payload.flatMap((entry, index) => {
    if (entry.type === 'none') return [];
    if (!formatter) return [{ colour: entry.color, name: entry.name, value: entry.value }];
    const formatted = formatter(entry.value, entry.name, entry, index, payload);
    if (formatted == null) return [];
    const [value, name] = Array.isArray(formatted) ? formatted : [formatted, entry.name];
    return [{ colour: entry.color, name, value }];
  });
}

export default function ChartTooltip({ active, payload, label, formatter }: ChartTooltipProps) {
  if (!active || !payload?.length) return null;
  const rows = rowsOf(payload, formatter);
  const time = typeof label === 'string' || typeof label === 'number' ? new Date(label) : null;

  return (
    <Box
      data-ui="chart-tooltip"
      sx={{
        bgcolor: 'background.paper',
        border: 1,
        borderColor: 'divider',
        borderRadius: 2,
        boxShadow: 3,
        paddingX: 1.5,
        paddingY: 1.25,
        whiteSpace: 'nowrap',
      }}
    >
      {time && (
        <Typography data-ui="chart-tooltip-time" variant="caption" component="div" color="text.secondary" sx={{ marginBottom: 0.75 }}>
          {time.toLocaleString(undefined, timeFormat)}
        </Typography>
      )}
      {rows.map(({ colour, name, value }, index) => (
        <Box key={index} data-ui="chart-tooltip-row" sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Box
            data-ui="chart-tooltip-swatch"
            sx={{ width: swatchSize, height: swatchSize, flexShrink: 0, borderRadius: 0.5, bgcolor: colour }}
          />
          <Typography data-ui="chart-tooltip-name" variant="bodySmall" component="span" color="text.primary">
            {name}
          </Typography>
          <Typography
            data-ui="chart-tooltip-value"
            variant="bodySmall"
            component="span"
            color="text.primary"
            sx={{ marginLeft: 'auto', paddingLeft: 2, fontWeight: 'fontWeightMedium', fontVariantNumeric: 'tabular-nums' }}
          >
            {value}
          </Typography>
        </Box>
      ))}
    </Box>
  );
}
