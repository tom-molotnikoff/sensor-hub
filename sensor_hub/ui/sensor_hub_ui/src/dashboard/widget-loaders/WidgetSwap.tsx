import { Box } from '@mui/material';
import type { ReactNode } from 'react';
import { useLoaderVisibility } from './useLoaderVisibility';
import { usePrefersReducedMotion } from './usePrefersReducedMotion';
import { fadeIn } from './keyframes';

interface WidgetSwapProps {
  /** True while the widget's own data is still being fetched for the first time. */
  loading: boolean;
  /** The Incoming-family loader to show while loading. */
  loader: ReactNode;
  /** Override the anti-flash minimum visible time. */
  minVisibleMs?: number;
  /** The resolved UI: content, empty state, or error (decided by the widget). */
  children: ReactNode;
}

/**
 * The core integration primitive for the loader family. Shows `loader` while
 * loading (with anti-flash min-display), then cross-fades to the resolved
 * `children`. Occupies the same box throughout so there is no layout shift.
 */
export default function WidgetSwap({ loading, loader, minVisibleMs, children }: WidgetSwapProps) {
  const showLoader = useLoaderVisibility(loading, { minVisibleMs });
  const reduced = usePrefersReducedMotion();

  if (showLoader) return <>{loader}</>;

  return (
    <Box
      sx={{
        height: '100%',
        width: '100%',
        ...(reduced ? {} : { animation: `${fadeIn} 250ms ease-out` }),
      }}
    >
      {children}
    </Box>
  );
}
