import { useRef, useState, type KeyboardEvent, type PointerEvent } from 'react';
import { Box, Typography } from '@mui/material';
import { alpha, useTheme } from '@mui/material/styles';
import { useBounded } from './useBounded';
import { responsivePixels } from './tiers';
import { density } from './theme/tokens';

interface SlideSwitchProps {
  checked: boolean | null;
  label: string;
  readOnly?: boolean;
  onChange: (checked: boolean) => void;
}

const trackWidth = 220;
const trackHeight = 72;
const trackPadding = 4;
const thumbWidth = 104;
const thumbHeight = 64;
const dragTravel = trackWidth - trackPadding * 2 - thumbWidth;
const lateLatch = 0.78;
const preLatchProgressMax = 0.34;
const postLatchProgressMin = 0.82;

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

function dragDistance(progress: number, startChecked: boolean) {
  return startChecked ? 1 - progress : progress;
}

function crossedLatch(progress: number, startChecked: boolean) {
  return dragDistance(progress, startChecked) >= lateLatch;
}

function latchedProgress(progress: number, startChecked: boolean) {
  const distance = dragDistance(progress, startChecked);
  const mapped = distance < lateLatch
    ? Math.pow(distance / lateLatch, 1.9) * preLatchProgressMax
    : postLatchProgressMin + (1 - Math.pow(1 - (distance - lateLatch) / (1 - lateLatch), 2.2)) * (1 - postLatchProgressMin);
  return startChecked ? 1 - mapped : mapped;
}

export default function SlideSwitch({ checked, label, readOnly = false, onChange }: SlideSwitchProps) {
  const theme = useTheme();
  const bounded = useBounded();
  const [dragProgress, setDragProgress] = useState<number | null>(null);
  const [dragStartChecked, setDragStartChecked] = useState(false);
  const dragOriginX = useRef(0);
  const dragMoved = useRef(false);
  const suppressClick = useRef(false);

  const resolved = checked !== null;
  const on = checked ?? false;
  const interactive = !readOnly && resolved;
  const dragging = dragProgress !== null;
  const progress = dragProgress ?? (resolved ? (on ? 1 : 0) : 0.5);
  const shownOn = dragging ? (crossedLatch(progress, dragStartChecked) ? !dragStartChecked : dragStartChecked) : on;
  const thumbLeft = trackPadding + (dragging ? latchedProgress(progress, dragStartChecked) : progress) * dragTravel;

  const { palette } = theme;
  const trackColour = !resolved
    ? alpha(palette.text.secondary, 0.22)
    : shownOn
      ? alpha(palette.primary.main, readOnly ? 0.55 : 0.95)
      : alpha(palette.text.secondary, readOnly ? 0.2 : 0.35);
  const onOpacity = !resolved ? 0.55 : shownOn ? 1 : 0.35;
  const offOpacity = !resolved ? 0.55 : shownOn ? 0.35 : 1;

  const handlePointerDown = (event: PointerEvent<HTMLDivElement>) => {
    if (!interactive) return;
    dragOriginX.current = event.clientX;
    dragMoved.current = false;
    setDragStartChecked(on);
    setDragProgress(on ? 1 : 0);
    if (typeof event.currentTarget.setPointerCapture === 'function') {
      event.currentTarget.setPointerCapture(event.pointerId);
    }
  };

  const handlePointerMove = (event: PointerEvent<HTMLDivElement>) => {
    if (dragProgress === null) return;
    const delta = event.clientX - dragOriginX.current;
    if (Math.abs(delta) > 4) dragMoved.current = true;
    setDragProgress(clamp((dragStartChecked ? 1 : 0) + delta / dragTravel, 0, 1));
  };

  const finishDrag = (event: PointerEvent<HTMLDivElement>) => {
    if (dragProgress === null) return;
    const finalProgress = dragProgress;
    setDragProgress(null);
    if (typeof event.currentTarget.releasePointerCapture === 'function') {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    if (!dragMoved.current) return;
    suppressClick.current = true;
    const next = crossedLatch(finalProgress, dragStartChecked) ? !dragStartChecked : dragStartChecked;
    if (next !== on) onChange(next);
  };

  const handleClick = () => {
    if (!interactive) return;
    if (suppressClick.current) {
      suppressClick.current = false;
      return;
    }
    onChange(!on);
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (!interactive) return;
    if (event.key === ' ' || event.key === 'Enter') {
      event.preventDefault();
      onChange(!on);
    }
  };

  return (
    <Box
      data-ui="slide-switch"
      sx={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        paddingX: responsivePixels(density.card),
        minWidth: 0,
        ...(bounded && { height: '100%', boxSizing: 'border-box' }),
      }}
    >
      <Box
        data-ui="slide-switch-track"
        role="checkbox"
        aria-checked={resolved ? on : 'mixed'}
        aria-disabled={!interactive}
        aria-label={label}
        tabIndex={interactive ? 0 : -1}
        onClick={handleClick}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={finishDrag}
        onPointerCancel={finishDrag}
        onKeyDown={handleKeyDown}
        sx={{
          position: 'relative',
          flex: 'none',
          width: '100%',
          maxWidth: `${trackWidth}px`,
          height: `${trackHeight}px`,
          borderRadius: `${trackHeight / 2}px`,
          backgroundColor: trackColour,
          boxShadow: shownOn
            ? `inset 0 0 0 1px ${alpha(palette.common.white, 0.14)}, 0 0 24px ${alpha(palette.primary.main, readOnly ? 0.1 : 0.22)}`
            : `inset 0 0 0 1px ${alpha(palette.text.primary, resolved ? 0.08 : 0.12)}`,
          cursor: interactive ? (dragging ? 'grabbing' : 'pointer') : 'default',
          userSelect: 'none',
          touchAction: 'none',
          outline: 'none',
          transition: dragging
            ? 'background-color 90ms linear, box-shadow 90ms linear'
            : 'background-color 180ms ease, box-shadow 180ms ease',
          '&::before': {
            content: '""',
            position: 'absolute',
            left: '50%',
            top: 16,
            bottom: 16,
            width: 2,
            transform: 'translateX(-50%)',
            borderRadius: 999,
            backgroundColor: !resolved
              ? alpha(palette.text.primary, 0.16)
              : shownOn
                ? alpha(palette.common.white, 0.28)
                : alpha(palette.text.primary, 0.12),
          },
        }}
      >
        <Box
          sx={{
            position: 'absolute',
            inset: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            paddingX: 2.5,
            color: shownOn ? palette.common.white : palette.text.primary,
          }}
        >
          <Typography component="span" variant="body2" sx={{ fontWeight: 'fontWeightBold', letterSpacing: '0.1em', opacity: onOpacity }}>
            ON
          </Typography>
          <Typography component="span" variant="body2" sx={{ fontWeight: 'fontWeightBold', letterSpacing: '0.1em', opacity: offOpacity }}>
            OFF
          </Typography>
        </Box>
        <Box
          data-ui="slide-switch-thumb"
          sx={{
            position: 'absolute',
            top: trackPadding,
            left: 0,
            width: `${thumbWidth}px`,
            height: `${thumbHeight}px`,
            borderRadius: `${thumbHeight / 2}px`,
            transform: `translateX(${thumbLeft}px) scale(${dragging ? 0.985 : 1})`,
            backgroundColor: palette.common.white,
            boxShadow: dragging
              ? `0 6px 14px ${alpha(palette.common.black, 0.18)}`
              : !resolved
                ? `0 0 10px ${alpha(palette.text.secondary, 0.14)}, 0 8px 18px ${alpha(palette.common.black, 0.08)}`
                : shownOn
                  ? `0 0 18px ${alpha(palette.primary.main, readOnly ? 0.18 : 0.34)}, 0 0 30px ${alpha(palette.primary.light, readOnly ? 0.1 : 0.22)}`
                  : `0 0 10px ${alpha(palette.text.secondary, 0.1)}, 0 10px 24px ${alpha(palette.common.black, 0.12)}`,
            transition: dragging ? 'none' : 'transform 240ms cubic-bezier(0.2, 0.9, 0.25, 1.25), box-shadow 200ms ease',
            '&::before': {
              content: '""',
              position: 'absolute',
              inset: 16,
              borderRadius: 999,
              background: !resolved
                ? `linear-gradient(90deg, ${alpha(palette.text.secondary, 0.12)}, ${alpha(palette.text.secondary, 0.08)})`
                : shownOn
                  ? `linear-gradient(90deg, ${alpha(palette.primary.main, 0.28)}, ${alpha(palette.primary.light, 0.12)})`
                  : `linear-gradient(90deg, ${alpha(palette.text.secondary, 0.16)}, ${alpha(palette.text.secondary, 0.06)})`,
            },
          }}
        />
      </Box>
    </Box>
  );
}
