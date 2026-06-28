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
