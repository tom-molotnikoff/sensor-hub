import type { TierValues } from '../tiers';

export const density = {
  page: { compact: 12, wide: 24 },
  card: { compact: 12, wide: 20 },
  gap: { compact: 12, wide: 16 },
} satisfies Record<string, TierValues<number>>;

export type Density = typeof density;
