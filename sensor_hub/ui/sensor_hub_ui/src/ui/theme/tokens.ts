import type { TierValues } from '../tiers';

export const density = {
  page: { compact: 12, wide: 24 },
  card: { compact: 12, wide: 20 },
  gap: { compact: 12, wide: 16 },
} satisfies Record<string, TierValues<number>>;

export type Density = typeof density;

export const standalonePageWidth = 444;

export const emptyStateMinHeight = { sm: 120, md: 200, lg: 300 } as const;

export type EmptyStateSize = keyof typeof emptyStateMinHeight;

export const chartAreaHeight = {
  sm: { compact: 200, wide: 200 },
  md: { compact: 280, wide: 280 },
  lg: { compact: 320, wide: 400 },
} satisfies Record<string, TierValues<number>>;

export type ChartAreaSize = keyof typeof chartAreaHeight;

export const metricDialSize = { sm: 96, md: 140, lg: 200 } as const;

export const metricMinHeight = 64;

export const metricFit = {
  value: { md: { width: 160, height: 96 }, lg: { width: 240, height: 160 } },
  dial: { md: 180, lg: 280 },
} as const;

export const stripNarrowWidth = 480;

export const stripCells = {
  sm: {
    gap: 1,
    minWidth: 56,
    padding: 1,
    radius: 2,
    narrow: { minWidth: 48, paddingX: 0.75, paddingY: 0.75, radius: 3 },
  },
  md: {
    gap: 1.5,
    minWidth: 100,
    padding: 1.5,
    radius: 2,
    narrow: { minWidth: 72, paddingX: 1.5, paddingY: 1, radius: 3 },
  },
} as const;
