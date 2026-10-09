import { useState } from 'react';
import { Box, Slider, Typography } from '@mui/material';
import { useBounded } from './useBounded';
import { responsivePixels } from './tiers';
import { density } from './theme/tokens';

interface ValueSliderProps {
  // null until the value is known, which leaves the slider inert.
  value: number | null;
  min: number;
  max: number;
  step: number;
  unit?: string;
  // The slider's accessible name.
  label: string;
  caption?: string;
  readOnly?: boolean;
  onCommit: (value: number) => void;
}

const sliderMaxWidth = 320;

function decimalsOf(step: number) {
  return (String(step).split('.')[1] ?? '').length;
}

// A numeric value above a slider that sets it. The value follows the drag and
// onCommit fires once, when the slider is let go.
export default function ValueSlider({ value, min, max, step, unit, label, caption, readOnly = false, onCommit }: ValueSliderProps) {
  const bounded = useBounded();
  const [dragValue, setDragValue] = useState<number | null>(null);
  const shown = dragValue ?? value;

  return (
    <Box
      data-ui="value-slider"
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        textAlign: 'center',
        paddingX: responsivePixels(density.card),
        minWidth: 0,
        ...(bounded && { height: '100%', boxSizing: 'border-box' }),
      }}
    >
      <Typography variant="metricSm" data-ui="metric-value" noWrap>
        {shown === null ? '-' : shown.toFixed(decimalsOf(step))}
        {unit && shown !== null && (
          <Typography component="span" variant="body" color="text.secondary" sx={{ fontWeight: 'fontWeightMedium', marginLeft: '2px' }}>
            {unit}
          </Typography>
        )}
      </Typography>
      {caption && (
        <Typography variant="caption" color="text.secondary" noWrap>
          {caption}
        </Typography>
      )}
      <Slider
        aria-label={label}
        size="small"
        min={min}
        max={max}
        step={step}
        value={shown ?? min}
        disabled={readOnly || value === null}
        valueLabelDisplay="auto"
        sx={{ width: '100%', maxWidth: sliderMaxWidth, flex: 'none' }}
        onChange={(_, next) => setDragValue(next as number)}
        onChangeCommitted={(_, next) => {
          setDragValue(null);
          if (next !== value) onCommit(next as number);
        }}
      />
    </Box>
  );
}
