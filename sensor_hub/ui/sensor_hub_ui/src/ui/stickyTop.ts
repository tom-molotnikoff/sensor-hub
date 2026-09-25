import { useSyncExternalStore } from 'react';
import { useTheme, type Theme } from '@mui/material';
import type { CSSObject } from '@mui/system';
import { useTier, wideMediaQuery } from './tiers';

const mediaPrefix = '@media ';

type ToolbarHeights = { base: number; queries: [string, number][] };

function toolbarHeights(theme: Theme): ToolbarHeights {
  const heights: ToolbarHeights = { base: 0, queries: [] };
  for (const [key, value] of Object.entries(theme.mixins.toolbar)) {
    if (key === 'minHeight') heights.base = value as number;
    else if (key.startsWith(mediaPrefix) && typeof value === 'object' && value !== null && 'minHeight' in value) {
      heights.queries.push([key, (value as { minHeight: number }).minHeight]);
    }
  }
  return heights;
}

export function stickyTop(theme: Theme, offset: number): CSSObject {
  const { base, queries } = toolbarHeights(theme);
  const rules: CSSObject = { top: base + offset };
  for (const [query, height] of queries) rules[query] = { top: height + offset };
  return { ...rules, [wideMediaQuery]: { top: offset } };
}

function subscribe(onChange: () => void) {
  window.addEventListener('resize', onChange);
  return () => window.removeEventListener('resize', onChange);
}

export function useStickyTop(): number {
  const theme = useTheme();
  const wide = useTier() === 'wide';
  const appBarHeight = useSyncExternalStore(subscribe, () => {
    const { base, queries } = toolbarHeights(theme);
    return queries.reduce(
      (height, [query, queryHeight]) => (window.matchMedia(query.slice(mediaPrefix.length)).matches ? queryHeight : height),
      base,
    );
  });
  return wide ? 0 : appBarHeight;
}
