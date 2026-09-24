import type { TierValues } from '../tiers';

export const density = {
  page: { compact: 12, wide: 24 },
  card: { compact: 12, wide: 20 },
  gap: { compact: 12, wide: 16 },
} satisfies Record<string, TierValues<number>>;

export type Density = typeof density;

export const emptyStateMinHeight = { sm: 120, md: 200, lg: 300 } as const;

export type EmptyStateSize = keyof typeof emptyStateMinHeight;

export const chartAreaHeight = {
  sm: { compact: 200, wide: 200 },
  md: { compact: 280, wide: 280 },
  lg: { compact: 320, wide: 400 },
} satisfies Record<string, TierValues<number>>;

export type ChartAreaSize = keyof typeof chartAreaHeight;
