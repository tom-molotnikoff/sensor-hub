import { keyframes } from '@mui/system';

/**
 * Shared keyframes for the "Incoming" loader family.
 * A calm grey skeleton base with a single terracotta element in motion.
 */

/** Horizontal sweep across a skeleton block. */
export const shimmer = keyframes`
  0% { background-position: 130% 0; }
  100% { background-position: -130% 0; }
`;

/** Gentle opacity breathing. */
export const pulse = keyframes`
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
`;

/** Cross-fade content in when a loader gives way to real data. */
export const fadeIn = keyframes`
  from { opacity: 0; }
  to { opacity: 1; }
`;

/** Rows/columns/tiles streaming in on a loop, like records arriving. */
export const cascade = keyframes`
  0% { opacity: 0; transform: translateY(8px); }
  13% { opacity: 1; transform: none; }
  74% { opacity: 1; }
  92%, 100% { opacity: 0; }
`;

/** Self-drawing line: draw on, then sweep off, on a loop (dasharray 320). */
export const draw = keyframes`
  0% { stroke-dashoffset: 320; }
  55% { stroke-dashoffset: 0; }
  100% { stroke-dashoffset: -320; }
`;

/** Self-drawing ring: draw around, then sweep off (dasharray 251.3 = 2*pi*40). */
export const drawRing = keyframes`
  0% { stroke-dashoffset: 251.3; }
  55% { stroke-dashoffset: 0; }
  100% { stroke-dashoffset: -251.3; }
`;

/**
 * Heatmap cell warming up. Colours are theme-dependent, so they are read from
 * CSS custom properties set on the grid (--cell-base / --cell-active).
 */
export const ripple = keyframes`
  0%, 65%, 100% { background-color: var(--cell-base); }
  32% { background-color: var(--cell-active); }
`;

/** Indeterminate progress segment sliding across a track. */
export const indeterminate = keyframes`
  0% { left: -40%; }
  100% { left: 102%; }
`;
