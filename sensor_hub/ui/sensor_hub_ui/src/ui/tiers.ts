import { useMediaQuery } from '@mui/material';

export type Tier = 'compact' | 'wide';

export type TierValues<T> = Record<Tier, T>;

export const breakpointValues = { xs: 0, sm: 600, md: 900, lg: 1200, xl: 1536 } as const;

const tierBreakpoints = { compact: 'xs', wide: 'md' } as const satisfies Record<Tier, keyof typeof breakpointValues>;

const wideQuery = `(min-width:${breakpointValues[tierBreakpoints.wide]}px)`;

export const wideMediaQuery = `@media ${wideQuery}`;

export function responsive<T>(values: TierValues<T>) {
  return { [tierBreakpoints.compact]: values.compact, [tierBreakpoints.wide]: values.wide };
}

export function useTier(): Tier {
  return useMediaQuery(wideQuery, { noSsr: true }) ? 'wide' : 'compact';
}
